import { useMemo, useState } from 'react'
import { FiColumns, FiList, FiCalendar, FiSearch, FiMoreVertical, FiFilter } from 'react-icons/fi'
import { api, errMsg, fmtTime } from '../lib/api'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Avatars, Badge, Button, Confirm, Empty, ErrorBox, PageHeader, Seg, Spinner, statusTone } from '../components/ui'
import { NoWorkspace } from '../components/NoWorkspace'
import { LaunchDialog, type LaunchParams } from '../components/LaunchDialog'

const STATUSES = ['active', 'paused', 'completed', 'interrupted', 'needs_attention', 'external_session_unavailable']
type View = 'list' | 'timeline' | 'board'
const dayKey = (d: Date) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
const weekDays = () => {
  const now = new Date(); const mon = new Date(now); mon.setDate(now.getDate() - ((now.getDay() + 6) % 7)); mon.setHours(0, 0, 0, 0)
  return Array.from({ length: 7 }, (_, i) => { const d = new Date(mon); d.setDate(mon.getDate() + i); return d })
}

export default function Sessions() {
  const ws = useWorkspace()
  const [error, setError] = useState('')
  const [view, setView] = useState<View>('board')
  const [q, setQ] = useState('')
  const [agent, setAgent] = useState('all')
  const [sort, setSort] = useState<'recent' | 'oldest' | 'title'>('recent')
  const [launch, setLaunch] = useState<LaunchParams | null>(null)
  const [del, setDel] = useState<any>(null)
  const [handoff, setHandoff] = useState<any>(null)
  const list = useAsync<any[]>(() => (ws ? (api.Sessions(ws.id) as any) : Promise.resolve([])), [ws?.id])

  const shown = useMemo(() => {
    const l = (list.data ?? []).filter((x) => (agent === 'all' || x.agent === agent) && (`${x.title} ${x.agent} ${x.handoff}`).toLowerCase().includes(q.toLowerCase()))
    return [...l].sort((a, b) => sort === 'title' ? (a.title || '').localeCompare(b.title || '') : (new Date(a.lastActive).getTime() - new Date(b.lastActive).getTime()) * (sort === 'recent' ? -1 : 1))
  }, [list.data, q, agent, sort])
  const agentsInUse = useMemo(() => Array.from(new Set((list.data ?? []).map((x) => x.agent))), [list.data])

  if (!ws) return <><PageHeader title="Session History" /><NoWorkspace /></>

  const setStatus = async (id: number, status: string) => { try { await api.SetSessionStatus(id, status); list.reload() } catch (x) { setError(errMsg(x)) } }
  const card = (s: any) => (
    <article key={s.id} className="rounded-xl border border-line bg-raised/60 p-3.5 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 gap-2.5"><span className="mt-0.5 w-1 shrink-0 rounded bg-line" aria-hidden /><div className="min-w-0">
          <h3 className="truncate text-[0.95rem]">{s.title || `Session #${s.id}`}</h3>
          <p className="text-xs text-mute">{fmtTime(s.lastActive)}</p></div></div>
        <details className="relative"><summary className="cursor-pointer list-none rounded p-1 text-mute hover:bg-line/60" aria-label={`Actions for session ${s.id}`}><FiMoreVertical aria-hidden /></summary>
          <div className="absolute right-0 z-10 mt-1 w-40 rounded-lg border border-line bg-panel p-1 shadow-xl">
            <button className="block w-full rounded px-2 py-1.5 text-left hover:bg-raised disabled:opacity-40" disabled={s.status === 'completed'} onClick={() => setLaunch({ workspaceId: ws.id, mode: 'resume', sessionId: s.id })}>Resume…</button>
            <button className="block w-full rounded px-2 py-1.5 text-left hover:bg-raised" onClick={() => setHandoff(s)}>Handoff note…</button>
            <button className="block w-full rounded px-2 py-1.5 text-left text-bad hover:bg-raised" onClick={() => setDel(s)}>Delete record…</button>
          </div></details>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2">
        <span className="flex flex-wrap items-center gap-1.5"><Badge tone="accent">{s.agent}</Badge><Badge tone={statusTone(s.status)}>● {s.status.replace(/_/g, ' ')}</Badge></span>
        <Avatars names={[s.agent]} />
      </div>
      {!s.externalId && <p className="mt-2 text-xs text-warn">No CLI session ID saved; only AICW's own context can be restored.</p>}
      <label className="mt-2 flex items-center gap-2 text-xs text-mute">Status
        <select aria-label={`Status of session ${s.id}`} value={s.status} className="max-w-[9rem] rounded-md border border-line bg-bg px-1.5 py-0.5 text-fg" onChange={(e) => setStatus(s.id, e.target.value)}>{STATUSES.map((x) => <option key={x}>{x}</option>)}</select></label>
      {s.handoff && <pre className="mt-2 max-h-24 overflow-auto whitespace-pre-wrap rounded-lg border border-line bg-bg p-2 text-xs">{s.handoff}</pre>}
    </article>
  )

  const days = weekDays()
  const todayKey = dayKey(new Date())
  return (
    <>
      <PageHeader title="Session History" subtitle="AICW keeps session records and handoff summaries, not conversation transcripts." />
      <ErrorBox error={error || list.error} onDismiss={() => setError('')} />
      <section className="glass rounded-2xl border border-line">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-5 py-3">
          <Seg<View> label="Session views" value={view} onChange={setView} options={[
            { id: 'list', label: 'List', icon: <FiList aria-hidden /> }, { id: 'timeline', label: 'Timeline', icon: <FiCalendar aria-hidden /> }, { id: 'board', label: 'Board', icon: <FiColumns aria-hidden /> }]} />
          <div className="flex flex-wrap items-center gap-2">
            <label className="flex items-center gap-2 rounded-lg border border-line bg-bg px-3 py-1.5 text-mute"><FiSearch aria-hidden />
              <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search sessions" aria-label="Search sessions" className="w-44 border-0 bg-transparent p-0 text-fg shadow-none" /></label>
            <label className="flex items-center gap-1.5 rounded-lg border border-line bg-bg px-2 py-1.5 text-mute"><FiFilter aria-hidden />
              <select aria-label="Filter by agent" value={agent} onChange={(e) => setAgent(e.target.value)} className="border-0 bg-transparent p-0 text-fg shadow-none"><option value="all">All agents</option>{agentsInUse.map((a) => <option key={a}>{a}</option>)}</select></label>
            <select aria-label="Sort" value={sort} onChange={(e) => setSort(e.target.value as any)} className="rounded-lg border border-line bg-bg px-2 py-1.5"><option value="recent">Newest first</option><option value="oldest">Oldest first</option><option value="title">Title</option></select>
          </div>
        </div>
        <div className="p-5">
          {list.loading && <Spinner />}
          {!list.loading && (list.data?.length ?? 0) === 0 && <Empty title="No sessions yet" hint="Sessions are recorded when you launch an agent from Fresh Start." />}
          {!list.loading && (list.data?.length ?? 0) > 0 && shown.length === 0 && <Empty title="No sessions match your filters" />}

          {view === 'list' && <div className="grid gap-3 lg:grid-cols-2">{shown.map(card)}</div>}

          {view === 'board' && shown.length > 0 && (
            <div className="grid auto-cols-[17rem] grid-flow-col gap-4 overflow-x-auto pb-2">
              {STATUSES.filter((st) => shown.some((x) => x.status === st) || ['active', 'paused', 'completed'].includes(st)).map((st) => (
                <section key={st} aria-label={st} className="rounded-xl border border-line bg-bg/50 p-3">
                  <h3 className="mb-3 flex items-center justify-between text-xs uppercase tracking-wider text-mute"><span>{st.replace(/_/g, ' ')}</span><span>{shown.filter((x) => x.status === st).length}</span></h3>
                  <div className="space-y-3">{shown.filter((x) => x.status === st).map(card)}</div>
                </section>))}
            </div>)}

          {view === 'timeline' && shown.length > 0 && (<>
            <div className="grid grid-cols-7 overflow-hidden rounded-xl border border-line text-xs uppercase">
              {days.map((d) => (
                <div key={dayKey(d)} className={`border-l border-line px-3 py-2 first:border-l-0 ${dayKey(d) === todayKey ? 'bg-accent/15 text-fg' : 'bg-raised/50 text-mute'}`}>{d.toLocaleDateString(undefined, { weekday: 'short' })} <b className="text-fg">{d.getDate()}</b></div>))}
            </div>
            <div className="mt-3 grid min-h-[12rem] grid-cols-7 gap-3">
              {days.map((d) => (
                <div key={dayKey(d)} className={`space-y-3 ${dayKey(d) === todayKey ? 'rounded-xl border-x border-accent/40 px-1' : ''}`}>{shown.filter((x) => dayKey(new Date(x.lastActive)) === dayKey(d)).map(card)}</div>))}
            </div>
            {shown.filter((x) => !days.some((d) => dayKey(d) === dayKey(new Date(x.lastActive)))).length > 0 && (<>
              <h3 className="mb-3 mt-6 text-xs uppercase tracking-wider text-mute">Earlier</h3>
              <div className="grid gap-3 lg:grid-cols-3">{shown.filter((x) => !days.some((d) => dayKey(d) === dayKey(new Date(x.lastActive)))).map(card)}</div></>)}
          </>)}
        </div>
      </section>
      {launch && <LaunchDialog params={launch} onClose={() => setLaunch(null)} onLaunched={list.reload} />}
      {del && <Confirm title="Delete session record?" danger confirmLabel="Delete" onCancel={() => setDel(null)} onConfirm={async () => { try { await api.DeleteSession(del.id, true); setDel(null); list.reload() } catch (e) { setError(errMsg(e)); setDel(null) } }}><p className="text-sm">Removes AICW’s record of “{del.title || `#${del.id}`}”. The external CLI’s own history and project files are untouched.</p></Confirm>}
      {handoff && <HandoffDialog wsId={ws.id} session={handoff} onClose={() => { setHandoff(null); list.reload() }} setError={setError} />}
    </>
  )
}

function HandoffDialog({ wsId, session, onClose, setError }: { wsId: number; session: any; onClose: () => void; setError: (s: string) => void }) {
  const [f, setF] = useState({ task: session.title || '', progress: '', issues: '', nextSteps: '', tests: '' })
  const [files, setFiles] = useState<string[]>([])
  const [md, setMd] = useState('')
  const [saveFile, setSaveFile] = useState(false)
  const gen = async () => {
    try {
      const tests = f.tests.split('\n').filter(Boolean).map((l) => { const [command, ...r] = l.split('=>'); return { command: command.trim(), result: r.join('=>').trim() } })
      setMd(await api.GenerateHandoff({ task: f.task, agent: session.agent, sessionId: session.externalId, progress: f.progress, filesChanged: files, tests, issues: f.issues, nextSteps: f.nextSteps } as any))
    } catch (e) { setError(errMsg(e)) }
  }
  const fld = (k: keyof typeof f, label: string, rows = 2) => (
    <label className="block text-sm"><span className="text-mute">{label}</span><textarea value={f[k]} onChange={(e) => setF({ ...f, [k]: e.target.value })} rows={rows} className="mt-1 w-full rounded-md border border-line bg-bg px-2 py-1" /></label>)
  return (
    <Confirm title="Session handoff note" confirmLabel="Save handoff" onCancel={onClose} onConfirm={async () => {
      if (!md) return gen()
      try { await api.SaveHandoff(wsId, session.id, md, saveFile ? await api.HandoffFileName(f.task) : '', true); onClose() } catch (e) { setError(errMsg(e)) }
    }}>
      <p className="mb-3 text-sm text-mute">Record only what you or the agent verified. AICW does not run tests or invent results.</p>
      <div className="space-y-2">
        <label className="block text-sm"><span className="text-mute">Task</span><input value={f.task} onChange={(e) => setF({ ...f, task: e.target.value })} className="mt-1 w-full rounded-md border border-line bg-bg px-2 py-1" /></label>
        {fld('progress', 'Verified progress')}
        <div><Button onClick={async () => setFiles((await api.SuggestChangedFiles(wsId)) as any)}>Fill files changed from git</Button> <span className="text-xs text-mute">{files.length} file(s)</span></div>
        {fld('tests', 'Tests actually run — one per line: command => result')}
        {fld('issues', 'Unresolved issues')}{fld('nextSteps', 'Next steps')}
        <Button onClick={gen}>Preview note</Button>
        {md && <pre className="max-h-48 overflow-auto whitespace-pre-wrap rounded border border-line bg-bg p-2 text-xs">{md}</pre>}
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={saveFile} onChange={(e) => setSaveFile(e.target.checked)} /> Also write a file into the project under <code>.aicw/sessions/</code> (creates that folder)</label>
      </div>
    </Confirm>
  )
}
