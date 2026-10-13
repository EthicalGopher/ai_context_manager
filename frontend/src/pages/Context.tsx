import { useState } from 'react'
import { api, errMsg, type Scan } from '../lib/api'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Badge, Button, Card, Confirm, DiffView, Empty, ErrorBox, PageHeader, Spinner, fmtSize } from '../components/ui'
import { NoWorkspace } from '../components/NoWorkspace'

export default function ContextPage() {
  const ws = useWorkspace()
  const [error, setError] = useState('')
  const [file, setFile] = useState('')
  const [text, setText] = useState('')
  const [notice, setNotice] = useState('')
  const [excl, setExcl] = useState<string[] | null>(null)

  const pkg = useAsync<any>(() => (ws ? api.BuildContext({ workspaceId: ws.id } as any) : Promise.resolve(null)), [ws?.id, excl?.join('|')])
  const scan = useAsync<Scan>(() => (ws ? (api.Scan(ws.id) as any) : Promise.resolve(null as any)), [ws?.id])
  const state = useAsync<any>(() => (ws ? api.GetWorkspaceState(ws.id) : Promise.resolve(null)), [ws?.id])

  if (!ws) return <><PageHeader title="Context" /><NoWorkspace /></>
  const excluded: string[] = excl ?? state.data?.excluded ?? []
  const toggle = async (p: string) => {
    const next = excluded.includes(p) ? excluded.filter((x) => x !== p) : [...excluded, p]
    try { await api.SaveWorkspaceState(ws.id, { ...state.data, excluded: next } as any, ws.agent); setExcl(next) } catch (e) { setError(errMsg(e)) }
  }
  const view = async (rel: string) => { setFile(rel); setText(''); try { setText(await api.ReadFile(ws.id, rel)) } catch (e) { setText(''); setError(errMsg(e)) } }
  const md = (scan.data?.items ?? []).filter((i) => i.path.toLowerCase().endsWith('.md'))
  const p = pkg.data

  return (
    <>
      <PageHeader title="Context" subtitle="The package that would be handed to an agent. Generated summaries are hints; the agent is told to verify against the code."
        actions={<>
          <Button disabled={!p} onClick={async () => { await api.CopyToClipboard(p.markdown); setNotice('Copied to clipboard.') }}>Copy</Button>
          <Button variant="primary" disabled={!p} onClick={async () => { try { const d = await api.ExportContext({ workspaceId: ws.id } as any); if (d) setNotice(`Exported to ${d}`) } catch (e) { setError(errMsg(e)) } }}>Export .md…</Button>
        </>} />
      <ErrorBox error={error || pkg.error} onDismiss={() => setError('')} />
      {notice && <p role="status" className="mb-3 text-sm text-ok">{notice}</p>}
      <div className="grid gap-4 xl:grid-cols-2">
        <Card title="Generated package" actions={p && <Badge tone={p.tokens > p.budget ? 'warn' : 'accent'}>~{p.tokens} / {p.budget} tokens</Badge>}>
          {pkg.loading && <Spinner />}
          {p && <pre className="max-h-[32rem] overflow-auto whitespace-pre-wrap rounded-md border border-line bg-bg p-3 text-xs">{p.markdown}</pre>}
        </Card>
        <div className="space-y-4">
          <Card title="Markdown & guidance files">
            {scan.loading && <Spinner />}
            {md.length === 0 && !scan.loading && <Empty title="No Markdown files found" />}
            <ul className="space-y-1 text-sm">{md.map((i) => (
              <li key={i.path} className="flex items-center justify-between gap-2">
                <button className="truncate text-left text-accent underline-offset-2 hover:underline" onClick={() => view(i.path)}><code>{i.path}</code></button>
                <span className="flex shrink-0 items-center gap-2 text-xs text-mute">{fmtSize(i.size)}
                  <label className="flex items-center gap-1"><input type="checkbox" aria-label={`Include ${i.path} in context`} disabled={i.secret} checked={!i.secret && !excluded.includes(i.path)} onChange={() => toggle(i.path)} />include</label></span>
              </li>))}</ul>
          </Card>
          {file && <FileView wsId={ws.id} rel={file} text={text} setError={setError} onSaved={() => view(file)} />}
          <AuditPanel wsId={ws.id} />
        </div>
      </div>
    </>
  )
}

function FileView({ wsId, rel, text, setError, onSaved }: { wsId: number; rel: string; text: string; setError: (s: string) => void; onSaved: () => void }) {
  const [edit, setEdit] = useState<string | null>(null)
  const [pv, setPv] = useState<any>(null)
  const backups = useAsync<string[]>(() => api.Backups(wsId, rel) as any, [wsId, rel, text])
  const [restore, setRestore] = useState('')
  return (
    <Card title={<code>{rel}</code>} actions={edit === null ? <Button onClick={() => setEdit(text)} disabled={!text}>Edit</Button> : <Button onClick={() => setEdit(null)}>Cancel edit</Button>}>
      {edit === null ? <pre className="max-h-72 overflow-auto whitespace-pre-wrap text-xs">{text || 'Not displayed.'}</pre> : (<>
        <textarea aria-label="Edit file" value={edit} onChange={(e) => setEdit(e.target.value)} rows={12} className="mono w-full rounded-md border border-line bg-bg p-2 text-xs" />
        <Button className="mt-2" variant="primary" onClick={async () => { try { setPv(await api.PreviewWrite(wsId, rel, edit)) } catch (e) { setError(errMsg(e)) } }}>Preview diff…</Button>
      </>)}
      {(backups.data ?? []).length > 0 && (
        <div className="mt-3 border-t border-line pt-2"><h3 className="mb-1 text-xs font-semibold uppercase text-mute">Version history (AICW backups)</h3>
          <ul className="space-y-1 text-xs">{(backups.data ?? []).slice(0, 8).map((b) => <li key={b} className="flex justify-between gap-2"><code className="truncate">{b.split('__')[0]}</code><button className="text-accent underline" onClick={() => setRestore(b)}>Restore</button></li>)}</ul></div>)}
      {pv && edit !== null && <Confirm title="Apply changes?" confirmLabel="Back up & save" onCancel={() => setPv(null)}
        onConfirm={async () => { try { await api.ApplyWrite(wsId, rel, edit, pv.baseHash, true); setPv(null); setEdit(null); onSaved() } catch (e) { setError(errMsg(e)); setPv(null) } }}><DiffView diff={pv.diff} /></Confirm>}
      {restore && <Confirm title="Restore this version?" confirmLabel="Restore" onCancel={() => setRestore('')}
        onConfirm={async () => { try { await api.RestoreBackup(wsId, rel, restore, true); setRestore(''); onSaved() } catch (e) { setError(errMsg(e)); setRestore('') } }}><p className="text-sm">The current file is backed up first, then replaced by <code>{restore.split('__')[0]}</code>.</p></Confirm>}
    </Card>
  )
}

function AuditPanel({ wsId }: { wsId: number }) {
  const log = useAsync<any[]>(() => api.AuditLog(wsId) as any, [wsId])
  return (
    <Card title="File change log">
      {(log.data ?? []).length === 0 ? <p className="text-sm text-mute">AICW has not changed any project files.</p> : (
        <ul className="max-h-40 space-y-1 overflow-auto text-xs">{[...(log.data ?? [])].reverse().map((e, i) => <li key={i}><Badge>{e.action}</Badge> <code>{e.path}</code> <span className="text-mute">{new Date(e.time).toLocaleString()}</span></li>)}</ul>)}
    </Card>
  )
}
