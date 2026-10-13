import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, errMsg, fmtTime, type Scan } from '../lib/api'
import { useStore } from '../store'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Badge, Button, Card, Confirm, Empty, ErrorBox, PageHeader, Spinner, fmtSize } from '../components/ui'

export default function Workspaces() {
  const { open, setOpen } = useStore()
  const ws = useWorkspace()
  const navigate = useNavigate()
  const [path, setPath] = useState('')
  const [error, setError] = useState('')
  const [forget, setForget] = useState<any>(null)
  const recent = useAsync(() => api.RecentWorkspaces() as Promise<any[]>, [open?.workspace.id])
  const scan = useAsync<Scan>(() => (ws ? (api.Scan(ws.id) as any) : Promise.resolve(null as any)), [ws?.id])

  const openPath = async (p: string, wantNew = false) => {
    setError('')
    try {
      const r: any = await api.OpenWorkspace(p, wantNew)
      setOpen(r); navigate(wantNew ? '/fresh-start' : '/')
    } catch (e) { setError(errMsg(e)) }
  }
  const pick = async () => { try { const p = await api.PickDirectory(); if (p) openPath(p) } catch (e) { setError(errMsg(e)) } }

  return (
    <>
      <PageHeader title="Workspaces" subtitle="Opening a workspace restores its saved state. It never starts an AI session." />
      <ErrorBox error={error} onDismiss={() => setError('')} />
      <Card title="Open a project directory" className="mb-4">
        <form className="flex gap-2" onSubmit={(e) => { e.preventDefault(); if (path.trim()) openPath(path.trim()) }}>
          <label className="sr-only" htmlFor="ws-path">Directory path</label>
          <input id="ws-path" value={path} onChange={(e) => setPath(e.target.value)} placeholder="~/code/my-project" className="flex-1 rounded-md border border-line bg-bg px-3 py-1.5" />
          <Button type="submit" variant="primary" disabled={!path.trim()}>Open</Button>
          <Button onClick={pick}>Browse…</Button>
        </form>
      </Card>
      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Recent workspaces">
          {recent.loading && <Spinner />}
          {recent.data?.length === 0 && <Empty title="No workspaces yet" hint="Open a directory above or run `aicw .` in a project." />}
          <ul className="space-y-1">{(recent.data ?? []).map((w: any) => (
            <li key={w.id} className={`flex items-center justify-between gap-2 rounded-md border px-3 py-2 ${w.id === ws?.id ? 'border-accent/50 bg-accent/5' : 'border-line'}`}>
              <div className="min-w-0"><div className="font-medium">{w.name}</div><code className="block truncate text-xs text-mute">{w.path}</code><div className="text-xs text-mute">Opened {fmtTime(w.lastOpened)}</div></div>
              <div className="flex gap-1.5"><Button onClick={() => openPath(w.path)}>Open</Button><Button variant="ghost" onClick={() => setForget(w)} aria-label={`Forget ${w.name}`}>Forget</Button></div>
            </li>))}
          </ul>
        </Card>
        <Card title={ws ? `Files in ${ws.name}` : 'Files'}>
          {!ws && <p className="text-sm text-mute">Select a workspace.</p>}
          {scan.loading && ws && <Spinner label="Scanning (read-only)…" />}
          <ErrorBox error={scan.error} />
          {scan.data && (<>
            <div className="mb-2 flex flex-wrap gap-1.5">
              <Badge tone="accent">{scan.data.state}</Badge>
              {scan.data.git.isRepo ? <Badge>git: {scan.data.git.branch}{scan.data.git.dirty ? ` (+${scan.data.git.dirty})` : ''}</Badge> : <Badge>no git</Badge>}
              {scan.data.truncated && <Badge tone="warn">scan truncated</Badge>}
            </div>
            <pre className="mb-3 max-h-40 overflow-auto rounded-md border border-line bg-bg p-2 text-xs">{scan.data.tree.join('\n') || '(empty)'}</pre>
            <h3 className="mb-1 text-xs font-semibold uppercase text-mute">AI guidance inventory</h3>
            <ul className="space-y-1 text-sm">
              {scan.data.items.filter((i) => ['instruction', 'skill'].includes(i.category)).map((i) => (
                <li key={i.path} className="flex justify-between gap-2"><code className="truncate">{i.path}</code><span className="text-xs text-mute">{i.category} · {fmtSize(i.size)}</span></li>
              ))}
              {!scan.data.items.some((i) => ['instruction', 'skill'].includes(i.category)) && <li className="text-mute">None found.</li>}
            </ul>
          </>)}
        </Card>
      </div>
      {forget && (
        <Confirm title="Forget workspace?" danger confirmLabel="Forget" onCancel={() => setForget(null)}
          onConfirm={async () => { try { await api.ForgetWorkspace(forget.id, true); if (ws?.id === forget.id) setOpen(null); setForget(null); recent.reload() } catch (e) { setError(errMsg(e)); setForget(null) } }}>
          <p>This removes AICW's saved record and sessions for <code>{forget.path}</code>. Project files are not touched.</p>
        </Confirm>
      )}
    </>
  )
}
