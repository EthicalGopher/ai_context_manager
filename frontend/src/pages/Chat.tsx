import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { FiTerminal, FiFolder } from 'react-icons/fi'
import { api, errMsg } from '../lib/api'
import { useStore, type Conv } from '../store'
import { useWorkspace } from '../lib/hooks'
import { Composer, type Attached } from '../components/Composer'
import { ModelPicker } from '../components/ModelPicker'
import { MessageView, type Msg } from '../components/MessageView'
import { TerminalPane } from '../components/TerminalPane'
import { Button, Confirm, ErrorBox } from '../components/ui'

const SUGGESTIONS = [
  'Summarize this project and what I should work on next',
  'Review my recent changes and point out risks',
  'Find and fix a bug I describe below',
  'Add what you learn today to the project memory',
]

/** Time to let an agent's interface come up before typing into it (tests set 0). */
export const TERMINAL_WARMUP = { ms: 2500 }
const merge = (m: Msg[], fresh: Msg[]) => { const seen = new Set(m.map((x) => x.id)); return [...m, ...fresh.filter((x) => !seen.has(x.id))] }
const getFlag = (k: string, d: boolean) => { try { const v = localStorage.getItem(k); return v === null ? d : v === '1' } catch { return d } }
const setFlag = (k: string, v: boolean) => { try { localStorage.setItem(k, v ? '1' : '0') } catch { /* storage unavailable */ } }

export default function Chat() {
  const { id } = useParams()
  const convId = id ? Number(id) : 0
  const ws = useWorkspace()
  const navigate = useNavigate()
  const { convs, setConvs, setError: setGlobalError } = useStore()
  const conv: Conv | undefined = convs.find((c) => c.id === convId)
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [error, setError] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [termOpen, setTermOpen] = useState(() => getFlag('aicw-term-open', true))
  const [autoTerm, setAutoTerm] = useState(true)
  const [target, setTarget] = useState<'terminal' | 'chat'>(() => { try { return localStorage.getItem('aicw-target') === 'chat' ? 'chat' : 'terminal' } catch { return 'terminal' } })
  const [running, setRunning] = useState(false)
  const [working, setWorking] = useState(false)
  const [edits, setEdits] = useState(false)
  const [saveMsg, setSaveMsg] = useState<Msg | null>(null)
  const [pageTitle, setPageTitle] = useState('')
  const [pageNames, setPageNames] = useState<string[]>([])
  const [agents, setAgents] = useState<{ id: string; name: string; state: string }[]>([])
  const bottom = useRef<HTMLDivElement>(null)
  const last = useRef(0)

  const refreshConvs = useCallback(async () => { if (ws) setConvs((await api.Conversations(ws.id)) as any) }, [ws?.id])
  useEffect(() => { api.GetSettings().then((st: any) => { setEdits(!!st.chatEdits); setAutoTerm(st.autoTerminal !== false) }).catch(() => {}) }, [])
  useEffect(() => { api.ListAgents().then((a: any) => setAgents(a.map((x: any) => ({ id: x.profile.id, name: x.profile.displayName, state: x.status.state })))).catch(() => {}) }, [])

  // Load + poll messages so replies the agent posts through AICW tools appear live.
  useEffect(() => {
    setMsgs([]); last.current = 0; setNote(''); setRunning(false); setWorking(false)
    if (!convId) return
    let live = true
    const tick = async () => {
      try {
        const fresh: any[] = (await api.Messages(convId, last.current)) as any
        if (!live || fresh.length === 0) return
        last.current = fresh[fresh.length - 1].id
        setMsgs((m) => merge(m, fresh))
      } catch (e) { if (live) setError(errMsg(e)) }
    }
    tick()
    const t = setInterval(() => { if (!document.hidden) tick() }, 1500)
    const r = setInterval(() => { api.TermStatus(convId).then((x: any) => live && setRunning(!!x)).catch(() => {}); api.ChatStatus(convId).then((x: any) => live && setWorking(!!x.running)).catch(() => {}) }, 1500)
    api.ChatStatus(convId).then((x: any) => live && setWorking(!!x.running)).catch(() => {})
    api.TermStatus(convId).then((x: any) => live && setRunning(!!x)).catch(() => {})
    return () => { live = false; clearInterval(t); clearInterval(r) }
  }, [convId])

  useEffect(() => { bottom.current?.scrollIntoView?.({ block: "end" }) }, [msgs.length])

  const ensureConv = async (title = ''): Promise<number> => {
    if (convId) return convId
    if (!ws) throw new Error('Open a project folder first')
    const c: any = await api.NewConversation(ws.id, title, '')
    await refreshConvs()
    navigate(`/c/${c.id}`, { replace: true })
    return c.id
  }

  const pickTarget = (t: 'terminal' | 'chat') => { setTarget(t); try { localStorage.setItem('aicw-target', t) } catch { /* storage unavailable */ } }

  // Make sure an agent terminal is running for this chat (starting it if needed).
  const ensureTerminal = async (cid: number): Promise<boolean> => {
    if ((await api.TermStatus(cid)) as any) return true
    const c: any = ((await api.Conversations(ws!.id)) as any).find((x: any) => x.id === cid)
    const r: any = await api.TermStart({ workspaceId: ws!.id, conversationId: cid, agent: c?.agent ?? '', mode: c?.sessionId > 0 ? 'resume' : 'new', cols: 100, rows: 30 } as any)
    if (r.error) { setError(r.error + (r.fallback ? '\n' + r.fallback : '')); return false }
    setTermOpen(true); setFlag('aicw-term-open', true); setRunning(true)
    await new Promise((res) => setTimeout(res, TERMINAL_WARMUP.ms))
    return true
  }

  const send = async (text: string, media: Attached[]) => {
    setBusy(true); setNote('')
    try {
      const cid = await ensureConv()
      let mode: 'terminal' | 'chat' = target
      if (mode === 'terminal' && !(await ensureTerminal(cid))) mode = 'chat'
      const r: any = await api.SendMessage({ conversationId: cid, text, mediaIds: media.map((m) => m.id), mode } as any)
      setNote(r.sent ? 'Delivered to the agent in the terminal.' : r.note)
      if (r.running) setWorking(true)
      if (cid !== convId) { last.current = 0; setMsgs([]) }
      const fresh: any[] = (await api.Messages(cid, cid === convId ? last.current : 0)) as any
      if (fresh.length) { last.current = fresh[fresh.length - 1].id; setMsgs((m) => merge(cid === convId ? m : [], fresh)) }
      refreshConvs()
    } finally { setBusy(false) }
  }

  const targetSwitch = (
    <div className="seg ml-1" role="radiogroup" aria-label="Send to">
      {(['terminal', 'chat'] as const).map((t) => (
        <button key={t} role="radio" aria-checked={target === t} aria-selected={target === t} onClick={() => pickTarget(t)} title={t === 'terminal' ? 'Type the message into the agent running in the terminal' : 'Run the agent in the background and show its reply here'}>{t === 'terminal' ? 'Terminal' : 'Chat reply'}</button>))}
    </div>
  )

  const toggleTerm = async () => {
    const next = !termOpen
    setTermOpen(next); setFlag('aicw-term-open', next)
    if (next && !convId) { try { await ensureConv('New chat') } catch (e) { setError(errMsg(e)) } }
  }

  if (!ws) return <Welcome />

  const empty = msgs.length === 0
  return (
    <div className="flex h-full min-w-0">
      <section className="flex min-w-0 flex-1 flex-col" aria-label="Conversation">
        <header className="flex items-center justify-between gap-3 border-b border-line px-5 py-2.5">
          <Title conv={conv} onRename={async (t) => { try { await api.RenameConversation(convId, t); refreshConvs() } catch (e) { setError(errMsg(e)) } }} />
          <div className="flex items-center gap-2">
            <button disabled={busy || working || !convId || empty} onClick={() => send('/write-pages', []).catch((e) => setError(errMsg(e)))}
              title="The agent writes everything from this conversation into project pages by itself (same as typing /write-pages)"
              className="rounded-lg border border-line px-2.5 py-1 text-xs font-bold text-fg hover:bg-line/60 disabled:opacity-40">Write to pages</button>
            <label className="flex items-center gap-2 text-xs text-mute" title="When off, the agent answers and reads files but cannot change them in chat. The terminal always uses the agent's own permission prompts.">
              <input type="checkbox" checked={edits} onChange={async (e) => { setEdits(e.target.checked); try { await api.SetChatEdits(e.target.checked) } catch (x) { setError(errMsg(x)); setEdits(!e.target.checked) } }} /> Allow file edits
            </label>
            {conv && (
              <select aria-label="Agent for this chat" value={conv.agent || 'claude'} onChange={async (e) => { try { await api.SetConversationAgent(conv.id, e.target.value); refreshConvs() } catch (x) { setError(errMsg(x)) } }} className="rounded-lg border border-line bg-bg px-2 py-1 text-xs">
                {agents.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}<option value="shell">Plain shell</option>
              </select>)}
            {conv && <ModelPicker agent={conv.agent || 'claude'} onError={setError} />}
            <Button variant={termOpen ? 'primary' : 'secondary'} onClick={toggleTerm} aria-pressed={termOpen}>
              <FiTerminal aria-hidden /> Terminal {running && <span className="h-2 w-2 rounded-full bg-ok" aria-label="running" />}
            </Button>
          </div>
        </header>
        <ErrorBox error={error} onDismiss={() => setError('')} />
        <div className="min-h-0 flex-1 overflow-auto">
          {empty ? (
            <div className="flex h-full flex-col items-center justify-center px-4 pb-10">
              <h1 className="mb-1 text-3xl font-bold">What are we working on?</h1>
              <p className="mb-6 max-w-lg text-center text-sm text-mute">Chat about <b className="text-fg">{ws.name}</b>. Attach screenshots, audio or video, dictate with your voice, and run an AI CLI in the terminal — it can read your notes and skills and add to them by itself.</p>
              <Composer wsId={ws.id} busy={busy} autoFocus onSend={send} placeholder={`Ask about ${ws.name}…`} onVoiceHelp={() => navigate('/settings')} footerLeft={targetSwitch} />
              <div className="flex max-w-3xl flex-wrap justify-center gap-2 px-4">
                {SUGGESTIONS.map((s) => <button key={s} className="rounded-full border border-line bg-panel px-3 py-1.5 text-sm text-mute hover:bg-raised hover:text-fg" onClick={() => send(s, [])}>{s}</button>)}
              </div>
            </div>
          ) : (
            <div className="mx-auto max-w-3xl space-y-6 px-4 pb-10 pt-6" role="log" aria-live="polite">
              {msgs.map((m) => <MessageView key={m.id} m={m} onSave={m.role === 'system' ? undefined : async (x) => { setSaveMsg(x); setPageTitle(''); try { setPageNames((((await api.ListPages(ws.id)) as any) ?? []).map((p: any) => p.title)) } catch { /* optional */ } }} />)}
              {working && (
                <div className="flex items-center gap-3 px-1 text-sm text-mute" role="status" aria-live="polite">
                  <span className="flex gap-1" aria-hidden><i className="h-2 w-2 animate-bounce rounded-full bg-mute" /><i className="h-2 w-2 animate-bounce rounded-full bg-mute [animation-delay:150ms]" /><i className="h-2 w-2 animate-bounce rounded-full bg-mute [animation-delay:300ms]" /></span>
                  Agent is working…
                  <button className="rounded-lg border border-line px-2 py-0.5 text-xs hover:bg-raised" onClick={() => api.StopChat(convId).then(() => setWorking(false))}>Stop</button>
                </div>)}
              {note && !working && <p role="status" className="text-center text-xs text-mute">{note}</p>}
              <div ref={bottom} />
            </div>
          )}
        </div>
        {!empty && <Composer wsId={ws.id} busy={busy} onSend={send} onVoiceHelp={() => navigate('/settings')} footerLeft={targetSwitch} />}
      </section>
      {saveMsg && (
        <Confirm title="Save to a page" confirmLabel="Save" onCancel={() => setSaveMsg(null)} onConfirm={async () => {
          try { const p: any = await api.SaveMessageToPage(saveMsg.id, pageTitle); setSaveMsg(null); setNote(`Saved to the page “${p.title}”. Find it under Pages.`) } catch (e) { setError(errMsg(e)); setSaveMsg(null) }
        }}>
          <p className="mb-3 text-sm text-mute">The message text and its attachments are added to the end of the page. Pick an existing page or type a new title.</p>
          <input list="page-titles" aria-label="Page title" autoFocus value={pageTitle} onChange={(e) => setPageTitle(e.target.value)} placeholder="Page title" className="w-full rounded-lg border border-line bg-bg px-3 py-2" />
          <datalist id="page-titles">{pageNames.map((n) => <option key={n} value={n} />)}</datalist>
        </Confirm>)}
      {termOpen && !conv && (
        <aside aria-label="Agent terminal" className="flex w-[46%] min-w-[22rem] max-w-[60rem] flex-col border-l border-line bg-[#0e0e11]">
          <div className="flex items-center justify-between border-b border-line bg-panel px-3 py-2"><span className="flex items-center gap-2 text-sm font-bold"><FiTerminal aria-hidden /> Terminal</span>
            <Button variant="ghost" onClick={() => { setTermOpen(false); setFlag('aicw-term-open', false) }} aria-label="Hide terminal">Hide</Button></div>
          <p className="m-auto max-w-xs p-6 text-center text-sm text-mute">Your agent starts here as soon as you send the first message{autoTerm ? '' : ' (or click Start once the chat exists)'}. Choose “Chat reply” instead to get answers as messages.</p>
        </aside>)}
      {termOpen && conv && (
        <div className="w-[46%] min-w-[22rem] max-w-[60rem]">
          <TerminalPane conv={conv} agents={agents} autoStart={autoTerm} onClose={() => { setTermOpen(false); setFlag('aicw-term-open', false) }}
            onAgent={async (a) => { try { await api.SetConversationAgent(conv.id, a); refreshConvs() } catch (e) { setGlobalError(errMsg(e)) } }}
            onStateChange={() => { refreshConvs(); api.TermStatus(conv.id).then((x: any) => setRunning(!!x)).catch(() => {}) }} />
        </div>
      )}
    </div>
  )
}

function Title({ conv, onRename }: { conv?: Conv; onRename: (t: string) => void }) {
  const [edit, setEdit] = useState(false)
  const [v, setV] = useState('')
  if (!conv) return <h2 className="text-base font-bold text-mute">New chat</h2>
  if (edit) return <input autoFocus aria-label="Chat title" value={v} onChange={(e) => setV(e.target.value)} onBlur={() => { setEdit(false); if (v.trim() && v !== conv.title) onRename(v) }} onKeyDown={(e) => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur(); if (e.key === 'Escape') setEdit(false) }} className="w-72 rounded-lg border border-line bg-bg px-2 py-1 font-bold" />
  return <button className="truncate rounded-lg px-2 py-1 text-left text-base font-bold hover:bg-raised" title="Rename" onClick={() => { setV(conv.title); setEdit(true) }}>{conv.title}</button>
}

function Welcome() {
  const { setOpen, setError } = useStore()
  const navigate = useNavigate()
  const [recent, setRecent] = useState<any[]>([])
  useEffect(() => { api.WorkspaceList().then((l: any) => setRecent(l ?? [])).catch(() => {}) }, [])
  const open = async (p: string) => { try { setOpen((await api.OpenWorkspace(p, false)) as any); navigate('/') } catch (e) { setError(errMsg(e)) } }
  return (
    <div className="m-auto flex h-full max-w-xl flex-col items-center justify-center gap-5 px-6 text-center">
      <h1 className="text-3xl font-bold">AI Context Workspace</h1>
      <p className="text-mute">Open a project folder to start chatting with your AI CLI, keep its memory and skills, and attach screenshots, audio and video.</p>
      <Button variant="primary" onClick={async () => { try { const p = await api.PickDirectory(); if (p) open(p) } catch (e) { setError(errMsg(e)) } }}><FiFolder aria-hidden /> Open project folder…</Button>
      {recent.length > 0 && <ul className="w-full space-y-1 text-left">{recent.slice(0, 6).map((w) => (
        <li key={w.workspace.id}><button className="w-full rounded-xl border border-line bg-panel px-4 py-2 hover:bg-raised" onClick={() => open(w.workspace.path)}><div className="font-bold">{w.workspace.name}</div><code className="text-xs text-mute">{w.workspace.path}</code></button></li>))}</ul>}
      <p className="text-xs text-mute">Or run <code>aicw .</code> in a terminal.</p>
    </div>
  )
}
