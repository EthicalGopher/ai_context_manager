import { useState } from 'react'
import { api, errMsg } from '../lib/api'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Badge, Button, Card, ErrorBox, PageHeader, Spinner, statusTone } from '../components/ui'
import { LaunchDialog } from '../components/LaunchDialog'

const SAMPLE = `{
  "id": "my-agent",
  "displayName": "My Agent",
  "executables": ["my-agent"],
  "handoff": "file",
  "capabilities": { "newSession": true }
}`

export default function Agents() {
  const ws = useWorkspace()
  const agents = useAsync<any[]>(() => api.ListAgents() as any, [])
  const terms = useAsync<string[]>(() => api.ListTerminals() as any, [])
  const [error, setError] = useState('')
  const [paths, setPaths] = useState<Record<string, string>>({})
  const [test, setTest] = useState('')
  const [json, setJson] = useState(SAMPLE)

  const savePath = async (id: string) => {
    try { const s: any = await api.GetSettings(); await api.SaveSettings({ ...s, agentPaths: { ...s.agentPaths, [id]: paths[id] ?? '' } }); agents.reload() } catch (e) { setError(errMsg(e)) }
  }
  return (
    <>
      <PageHeader title="AI Agents" subtitle="Each CLI is an independent adapter. “Verified” means the flags were checked against the real CLI’s own help; unverified adapters are labelled and not claimed to work." />
      <ErrorBox error={error || agents.error} onDismiss={() => setError('')} />
      {agents.loading && <Spinner />}
      <div className="grid gap-4 xl:grid-cols-2">
        {(agents.data ?? []).map((a: any) => {
          const p = a.profile, st = a.status
          return (
            <Card key={p.id} title={<span className="flex items-center gap-2">{p.displayName} <Badge tone={statusTone(st.state)}>{st.state}</Badge>{p.verified ? <Badge tone="ok">adapter verified {p.verifiedVersion}</Badge> : <Badge tone="warn">unverified</Badge>}{!p.builtin && <Badge>custom</Badge>}</span>}
              actions={!p.builtin && <Button variant="ghost" onClick={async () => { try { await api.RemoveCustomAgent(p.id); agents.reload() } catch (e) { setError(errMsg(e)) } }}>Remove</Button>}>
              <dl className="grid grid-cols-[7rem_1fr] gap-y-1 text-sm">
                <dt className="text-mute">Executable</dt><dd>{st.path ? <code>{st.path}</code> : <span className="text-mute">{st.detail}</span>}</dd>
                <dt className="text-mute">Version</dt><dd>{st.version || '—'}</dd>
                <dt className="text-mute">Launch</dt><dd>context via <strong>{p.handoff}</strong>; {p.capabilities.resumeById ? 'resume by ID' : p.capabilities.continueLast ? 'continue last only' : 'no resume'}</dd>
                <dt className="text-mute">Access</dt><dd className="text-warn">Can read/edit files and run commands in the workspace; sends context to its provider.</dd>
              </dl>
              {p.notes && <p className="mt-2 text-xs text-mute">{p.notes}</p>}
              <div className="mt-3 flex gap-2">
                <label className="sr-only" htmlFor={`path-${p.id}`}>Executable path for {p.displayName}</label>
                <input id={`path-${p.id}`} placeholder="Override executable path" value={paths[p.id] ?? ''} onChange={(e) => setPaths({ ...paths, [p.id]: e.target.value })} className="flex-1 rounded-md border border-line bg-bg px-2 py-1 text-sm" />
                <Button onClick={() => savePath(p.id)}>Save path</Button>
                <Button disabled={!ws || st.state !== 'Detected'} onClick={() => setTest(p.id)}>Test launch…</Button>
              </div>
            </Card>)
        })}
      </div>
      <div className="mt-4 grid gap-4 xl:grid-cols-2">
        <Card title="Add a custom CLI profile">
          <p className="mb-2 text-xs text-mute">Profiles are validated data. They are never executed as scripts, arguments are passed as an argument array, and only environment variable <em>names</em> may be listed (never secret values).</p>
          <textarea aria-label="Custom agent JSON" value={json} onChange={(e) => setJson(e.target.value)} rows={9} className="mono w-full rounded-md border border-line bg-bg p-2 text-xs" />
          <Button className="mt-2" variant="primary" onClick={async () => { try { await api.AddCustomAgent(json); agents.reload() } catch (e) { setError(errMsg(e)) } }}>Validate &amp; add</Button>
        </Card>
        <Card title="Terminal emulators">
          {(terms.data ?? []).length ? <p className="text-sm">{terms.data!.join(', ')} <span className="text-mute">(first is used unless set in Settings)</span></p> : <p className="text-sm text-warn">None found. Launches will show a copyable command instead.</p>}
        </Card>
      </div>
      {test && ws && <LaunchDialog params={{ workspaceId: ws.id, agent: test, mode: 'new', title: 'Test launch' }} onClose={() => setTest('')} onLaunched={() => {}} />}
    </>
  )
}
