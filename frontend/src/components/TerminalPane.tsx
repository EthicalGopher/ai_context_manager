import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { FiPlay, FiTerminal, FiX, FiRotateCw } from 'react-icons/fi'
import { api, errMsg, onEvent } from '../lib/api'
import { Button, Spinner } from './ui'

const b64 = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0))

interface Conv { id: number; workspaceId: number; agent: string; sessionId: number }
interface AgentOpt { id: string; name: string; state: string }

/** Embedded terminal: runs the agent CLI (or a shell) inside the app. */
export function TerminalPane({ conv, agents, autoStart, onAgent, onStateChange, onClose }: { conv: Conv; agents: AgentOpt[]; autoStart?: boolean; onAgent: (id: string) => void; onStateChange: () => void; onClose: () => void }) {
  const [termId, setTermId] = useState('')
  const [info, setInfo] = useState<any>(null)
  const [error, setError] = useState('')
  const [exited, setExited] = useState<number | null>(null)
  const [busy, setBusy] = useState(true)
  const host = useRef<HTMLDivElement>(null)
  const tried = useRef(0)
  // Open directly: start the chat's agent as soon as the pane shows (once per chat; never loops on errors).
  useEffect(() => {
    if (!autoStart || busy || termId || tried.current === conv.id || agents.length === 0) return
    tried.current = conv.id
    start(conv.agent || 'claude', conv.sessionId > 0 ? 'resume' : 'new')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoStart, busy, termId, conv.id, agents.length])

  useEffect(() => {
    let live = true
    setTermId(''); setExited(null); setError(''); setInfo(null); setBusy(true)
    api.TermStatus(conv.id).then((t: any) => { if (live && t) setTermId(t.id) }).finally(() => live && setBusy(false))
    return () => { live = false }
  }, [conv.id])

  const start = async (agent: string, mode: 'new' | 'resume') => {
    setBusy(true); setError(''); setExited(null)
    try {
      const size = host.current ? { cols: 100, rows: 30 } : { cols: 100, rows: 30 }
      const r: any = await api.TermStart({ workspaceId: conv.workspaceId, conversationId: conv.id, agent, mode, ...size } as any)
      setInfo(r)
      if (r.error) setError(r.error + (r.fallback ? '\n' + r.fallback : ''))
      else { setTermId(r.id); onStateChange() }
    } catch (e) { setError(errMsg(e)) } finally { setBusy(false) }
  }

  useEffect(() => {
    if (!termId || !host.current) return
    const term = new Terminal({
      fontFamily: "'JetBrains Mono Variable', ui-monospace, monospace", fontSize: 13, cursorBlink: true, scrollback: 5000, allowProposedApi: true,
      theme: { background: '#0e0e11', foreground: '#e6e6ec', cursor: '#8b9cff', selectionBackground: '#33408a' },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(host.current)
    const doFit = () => { try { fit.fit(); api.TermResize(termId, term.cols, term.rows).catch(() => {}) } catch { /* hidden */ } }
    doFit()
    api.TermBuffer(termId).then((s: string) => { if (s) term.write(b64(s)) }).catch(() => {})
    const d = term.onData((data) => { api.TermWrite(termId, data).catch(() => {}) })
    const offData = onEvent('term:data', (p: { id: string; data: string }) => { if (p.id === termId) term.write(b64(p.data)) })
    const offExit = onEvent('term:exit', (p: { id: string; code: number }) => { if (p.id === termId) { setExited(p.code); onStateChange() } })
    const ro = new ResizeObserver(doFit)
    ro.observe(host.current)
    term.focus()
    return () => { d.dispose(); offData(); offExit(); ro.disconnect(); term.dispose() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [termId])

  const agentSel = (
    <select aria-label="Agent for this chat" value={conv.agent || 'claude'} onChange={(e) => onAgent(e.target.value)} className="rounded-lg border border-line bg-bg px-2 py-1.5 text-sm">
      {agents.map((a) => <option key={a.id} value={a.id}>{a.name}{a.state === 'Detected' ? '' : ` (${a.state.toLowerCase()})`}</option>)}
      <option value="shell">Plain shell</option>
    </select>
  )

  return (
    <aside aria-label="Agent terminal" className="flex h-full min-w-0 flex-col border-l border-line bg-[#0e0e11]">
      <div className="flex items-center justify-between gap-2 border-b border-line bg-panel px-3 py-2">
        <span className="flex items-center gap-2 text-sm font-bold"><FiTerminal aria-hidden /> Terminal</span>
        <span className="flex items-center gap-2">
          {termId && !exited && <Button variant="ghost" onClick={async () => { await api.TermClose(termId); setTermId(''); onStateChange() }} aria-label="Stop terminal"><FiX aria-hidden /> Stop</Button>}
          <Button variant="ghost" onClick={onClose} aria-label="Hide terminal">Hide</Button>
        </span>
      </div>
      {!termId && (
        <div className="m-auto max-w-sm space-y-4 p-6 text-center">
          {busy ? <Spinner label="Starting…" /> : (<>
            <FiTerminal className="mx-auto text-4xl text-accent" aria-hidden />
            <h3 className="text-lg">Run an AI CLI right here</h3>
            <p className="text-sm text-mute">The agent runs in a real terminal inside this chat. Messages you send go to it, and it can read your notes and skills through AICW tools.</p>
            <div className="flex justify-center">{agentSel}</div>
            <div className="flex flex-wrap justify-center gap-2">
              <Button variant="primary" onClick={() => start(conv.agent || 'claude', 'new')}><FiPlay aria-hidden /> Start {agents.find((a) => a.id === conv.agent)?.name ?? 'agent'}</Button>
              {conv.sessionId > 0 && conv.agent !== 'shell' && <Button onClick={() => start(conv.agent, 'resume')}><FiRotateCw aria-hidden /> Resume</Button>}
              <Button onClick={() => start('shell', 'new')}>Open shell</Button>
            </div>
            {error && <p role="alert" className="whitespace-pre-wrap rounded-lg border border-bad/40 bg-bad/10 p-2 text-left text-xs text-bad">{error}</p>}
            {info?.notes?.length > 0 && <ul className="list-disc space-y-1 pl-5 text-left text-xs text-mute">{info.notes.map((n: string, i: number) => <li key={i}>{n}</li>)}</ul>}
          </>)}
        </div>
      )}
      {termId && <div ref={host} className="min-h-0 flex-1 p-2" data-testid="xterm-host" />}
      {exited !== null && <div role="status" className="flex items-center justify-between border-t border-line bg-panel px-3 py-2 text-sm"><span>Process exited (code {exited}).</span><Button onClick={() => { setTermId(''); setExited(null) }}>Start again</Button></div>}
    </aside>
  )
}
