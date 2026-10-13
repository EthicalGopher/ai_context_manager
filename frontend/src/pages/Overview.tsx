import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { FiActivity, FiFolder, FiTag, FiUsers, FiTarget, FiShare2, FiSearch, FiList, FiImage, FiClock } from 'react-icons/fi'
import { api, fmtTime, type Scan, type ScanItem } from '../lib/api'
import { useStore } from '../store'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Avatars, Badge, Button, Card, ErrorBox, PageHeader, Seg, Spinner, fmtSize, statusTone } from '../components/ui'
import { NoWorkspace } from '../components/NoWorkspace'
import { ResumePanel } from '../components/ResumePanel'

type Tab = 'sessions' | 'inventory' | 'media'

export default function Overview() {
  const open = useStore((s) => s.open)
  const setOpen = useStore((s) => s.setOpen)
  const ws = useWorkspace()
  const navigate = useNavigate()
  const [tab, setTab] = useState<Tab>('sessions')
  const [q, setQ] = useState('')
  const scan = useAsync<Scan>(() => (ws ? (api.Scan(ws.id) as any) : Promise.resolve(null as any)), [ws?.id])
  const agents = useAsync<any[]>(() => api.ListAgents() as any, [])
  const media = useAsync<any[]>(() => (ws ? (api.ListMedia(ws.id) as any) : Promise.resolve([])), [ws?.id])
  const pkg = useAsync<any>(() => (ws ? api.BuildContext({ workspaceId: ws.id } as any) : Promise.resolve(null)), [ws?.id])
  const refresh = async () => { if (ws) setOpen((await api.OpenWorkspace(ws.path, false)) as any) }

  const items = useMemo(() => (scan.data?.items ?? []).filter((i) => i.path.toLowerCase().includes(q.toLowerCase())), [scan.data, q])

  if (!open || !ws) {
    return (<><PageHeader title="Overview" subtitle="Open a project to prepare a clean, relevant AI-coding context." /><NoWorkspace /></>)
  }
  const s = scan.data
  const latest = open.sessions[0]
  const detected = (agents.data ?? []).filter((a) => a.status.state === 'Detected')
  const pct = pkg.data ? Math.min(100, Math.round((pkg.data.tokens / pkg.data.budget) * 100)) : 0
  const lastHandoff = open.sessions.find((x) => x.handoff)

  const row = (icon: React.ReactNode, label: string, body: React.ReactNode) => (
    <div className="grid grid-cols-[8.5rem_1fr] items-center gap-3 py-1.5"><dt className="flex items-center gap-2 text-mute">{icon}{label}</dt><dd className="min-w-0">{body}</dd></div>
  )

  return (
    <>
      <header className="glass mb-5 rounded-2xl border border-line px-6 py-5">
        <div className="mb-3 flex items-start justify-between gap-4">
          <div className="min-w-0">
            <nav aria-label="Breadcrumb" className="mb-2 text-xs text-mute">Workspaces <span aria-hidden>/</span> <span className="text-fg">{ws.name}</span></nav>
            <h1 className="flex items-center gap-3 text-3xl font-bold"><FiFolder className="text-accent" aria-hidden />{ws.name}</h1>
          </div>
          <div className="flex shrink-0 gap-2">
            <Button onClick={async () => { try { await api.ExportContext({ workspaceId: ws.id } as any) } catch { /* cancelled */ } }}><FiShare2 aria-hidden /> Export</Button>
            <Button onClick={() => navigate('/media')}><FiImage aria-hidden /> Media</Button>
            <Button variant="primary" onClick={() => navigate('/fresh-start')}>Fresh Start</Button>
          </div>
        </div>
        <dl className="text-sm">
          {row(<FiActivity aria-hidden />, 'Status', latest ? <Badge tone={statusTone(latest.status)}>● {latest.status.replace(/_/g, ' ')}</Badge> : <Badge>● no session yet</Badge>)}
          {row(<FiClock aria-hidden />, 'Path', <code className="truncate text-fg">{ws.path}</code>)}
          {row(<FiTag aria-hidden />, 'Tags', <span className="flex flex-wrap gap-2">
            {s?.git.isRepo ? <><Badge tone="neutral">⎇ {s.git.branch || 'no branch'}</Badge>{s.git.dirty > 0 && <Badge tone="warn">{s.git.dirty} changed</Badge>}</> : <Badge>no git</Badge>}
            {s && <Badge tone={s.state === 'ai-configured' ? 'ok' : 'neutral'}>{s.state}</Badge>}
            {s && <Badge tone="accent">{s.items.filter((i) => i.category === 'instruction').length} instructions</Badge>}
            {s && <Badge tone="accent">{s.items.filter((i) => i.category === 'skill').length} skill files</Badge>}
          </span>)}
          {row(<FiUsers aria-hidden />, 'Agents', detected.length ? <span className="flex items-center gap-3"><Avatars names={detected.map((a) => a.profile.displayName)} max={6} /><span className="text-xs text-mute">{detected.map((a) => a.profile.displayName).join(', ')}</span></span> : <span className="text-warn">No agent CLI detected. See AI Agents.</span>)}
          {row(<FiTarget aria-hidden />, 'Context', pkg.data ? (
            <span className="flex items-center gap-3"><span className="h-1.5 w-56 overflow-hidden rounded-full bg-raised" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} aria-label="Context budget used"><span className="block h-full bg-accent" style={{ width: `${pct}%` }} /></span><span className="text-xs text-mute">~{pkg.data.tokens} / {pkg.data.budget} tokens</span></span>) : <Spinner label="Building…" />)}
        </dl>
      </header>

      <section className="glass rounded-2xl border border-line">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-5 py-3">
          <Seg<Tab> label="Overview sections" value={tab} onChange={setTab} options={[
            { id: 'sessions', label: 'Sessions', icon: <FiClock aria-hidden /> },
            { id: 'inventory', label: 'Inventory', icon: <FiList aria-hidden /> },
            { id: 'media', label: `Media${media.data?.length ? ` · ${media.data.length}` : ''}`, icon: <FiImage aria-hidden /> },
          ]} />
          {tab === 'inventory' && (
            <label className="flex items-center gap-2 rounded-lg border border-line bg-bg px-3 py-1.5 text-mute"><FiSearch aria-hidden />
              <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search files" aria-label="Search files" className="w-48 border-0 bg-transparent p-0 text-fg shadow-none" /></label>)}
        </div>
        <div className="p-5">
          <ErrorBox error={scan.error} />
          {tab === 'sessions' && (<div className="space-y-4">
            <ResumePanel open={open} onChanged={refresh} />
            <Card title="Last handoff summary">
              {lastHandoff ? <><div className="mb-1 text-xs text-mute">{lastHandoff.title || `Session #${lastHandoff.id}`} · {fmtTime(lastHandoff.lastActive)}</div><pre className="max-h-48 overflow-auto whitespace-pre-wrap text-xs">{lastHandoff.handoff}</pre></>
                : <p className="text-sm text-mute">No handoff notes yet. Generate one from Session History when you finish a task.</p>}
            </Card>
          </div>)}
          {tab === 'inventory' && (scan.loading ? <Spinner label="Scanning (read-only)…" /> : (
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-mute"><tr><th className="py-1.5 pr-3">File</th><th className="pr-3">Class</th><th className="pr-3">Size</th><th>Modified</th></tr></thead>
              <tbody>{items.map((i: ScanItem) => (
                <tr key={i.path} className="border-t border-line"><td className="py-2 pr-3"><code>{i.path}</code>{i.warning && <div className="text-xs text-warn">⚠ {i.warning}</div>}</td>
                  <td className="pr-3"><Badge>{i.category}</Badge></td><td className="pr-3 text-mute">{fmtSize(i.size)}</td><td className="text-mute">{new Date(i.modTime).toLocaleDateString()}</td></tr>))}
                {items.length === 0 && <tr><td colSpan={4} className="py-6 text-center text-mute">No matching files.</td></tr>}</tbody>
            </table>))}
          {tab === 'media' && ((media.data ?? []).length === 0
            ? <p className="py-6 text-center text-sm text-mute">No media yet. <button className="text-accent underline" onClick={() => navigate('/media')}>Add images and videos</button></p>
            : <ul className="grid grid-cols-2 gap-3 md:grid-cols-4">{(media.data ?? []).slice(0, 8).map((m: any) => (
              <li key={m.id} className="overflow-hidden rounded-xl border border-line bg-bg">
                <div className="aspect-video bg-raised">{m.kind === 'image' ? <img src={`/media/${m.id}`} alt={m.name} className="h-full w-full object-cover" loading="lazy" /> : m.kind === 'video' ? <video src={`/media/${m.id}#t=0.1`} preload="metadata" muted className="h-full w-full object-cover" /> : null}</div>
                <div className="truncate px-3 py-2 text-xs">{m.name}</div></li>))}</ul>)}
        </div>
      </section>
    </>
  )
}
