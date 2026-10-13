import { useState } from 'react'
import { api, errMsg } from '../lib/api'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Badge, Button, Card, Confirm, DiffView, ErrorBox, Spinner } from './ui'
import { NoWorkspace } from './NoWorkspace'

export default function SkillFiles() {
  const ws = useWorkspace()
  const [agent, setAgent] = useState('claude')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [install, setInstall] = useState<{ id: string; previews: any[] } | null>(null)
  const [src, setSrc] = useState('')
  const [imp, setImp] = useState<any>(null)
  const data = useAsync<any>(() => (ws ? api.SkillsOverview(ws.id, agent) : Promise.resolve(null)), [ws?.id, agent])
  if (!ws) return <NoWorkspace />
  const d = data.data

  const startInstall = async (id: string) => { try { setInstall({ id, previews: (await api.PreviewSkillInstall(ws.id, id, agent)) as any }) } catch (e) { setError(errMsg(e)) } }
  const doInstall = async () => {
    if (!install) return
    try { await api.ApplySkillInstall(ws.id, install.id, agent, true); setNotice(`Installed ${install.id}. Previous versions (if any) are kept in the backup history.`); setInstall(null); data.reload() } catch (e) { setError(errMsg(e)); setInstall(null) }
  }
  const preview = async () => { try { setImp(await api.PreviewSkillImport(src)) } catch (e) { setError(errMsg(e)) } }

  return (
    <>
      <p className="mb-3 text-sm text-mute">Skill files found in this project and your home folder (read-only by default). Nothing is installed or executed because it was discovered.</p>
      <ErrorBox error={error || data.error} onDismiss={() => setError('')} />
      {notice && <p role="status" className="mb-3 text-sm text-ok">{notice}</p>}
      <p className="mb-4 rounded-md border border-warn/40 bg-warn/10 px-3 py-2 text-sm text-warn">Skills and instruction files can contain untrusted commands or prompt-injection. Review them before use.</p>
      <div className="mb-4 flex items-center gap-2"><label htmlFor="sk-agent" className="text-sm text-mute">Target agent</label>
        <select id="sk-agent" value={agent} onChange={(e) => setAgent(e.target.value)} className="rounded-md border border-line bg-bg px-2 py-1.5">{['claude', 'codex', 'gemini', 'kilo', 'agy', 'freebuff'].map((a) => <option key={a}>{a}</option>)}</select></div>
      {data.loading && <Spinner />}
      {d && (<div className="grid gap-4 xl:grid-cols-2">
        <Card title="Discovered skills">
          {d.inventory.skills.length === 0 && <p className="text-sm text-mute">No skills found in this project or your user directories.</p>}
          <ul className="space-y-2">{d.inventory.skills.map((s: any) => (
            <li key={s.path} className="rounded-md border border-line p-2 text-sm">
              <div className="flex flex-wrap items-center gap-2"><strong>{s.name}</strong><Badge tone={s.source === 'project' ? 'accent' : 'neutral'}>{s.source}</Badge><Badge>{s.agent}</Badge>{s.managed && <Badge tone="ok">AICW-managed v{s.templateVersion}</Badge>}</div>
              <p className="text-mute">{s.description || 'No description.'}</p><code className="text-xs text-mute">{s.path}</code>
              {s.warnings.map((w: string) => <p key={w} className="text-xs text-warn">⚠ {w}</p>)}
            </li>))}</ul>
          {d.inventory.duplicates.map((x: string) => <p key={x} className="mt-2 text-sm text-warn">Conflict: {x}</p>)}
        </Card>
        <div className="space-y-4">
          <Card title="Recommended project skills">
            <ul className="space-y-3">{d.templates.map((t: any) => {
              const rec = d.recommendations.find((r: any) => r.template === t.id)
              return (<li key={t.id} className="text-sm">
                <div className="flex items-center justify-between gap-2"><strong>{t.name}</strong><span className="flex items-center gap-2"><Badge tone={rec.state === 'ok' ? 'ok' : 'warn'}>{rec.state}</Badge>
                  <Button disabled={rec.state === 'ok'} onClick={() => startInstall(t.id)}>{rec.state === 'stale' ? 'Review update…' : 'Preview install…'}</Button></span></div>
                <p className="text-mute">{t.description}</p></li>)
            })}</ul>
          </Card>
          <Card title="Import from a local directory">
            <div className="flex gap-2"><input aria-label="Skill directory" value={src} onChange={(e) => setSrc(e.target.value)} placeholder="~/skills/my-skill" className="flex-1 rounded-md border border-line bg-bg px-3 py-1.5" />
              <Button onClick={async () => { try { const p = await api.PickSkillDirectory(); if (p) setSrc(p) } catch (e) { setError(errMsg(e)) } }}>Browse…</Button><Button onClick={preview} disabled={!src}>Preview</Button></div>
            {imp && (<div className="mt-3 text-sm">
              <p className="mb-1">Would copy {imp.files.length} file(s) to the {agent === 'claude' ? '.claude' : '.agents'}/skills/{imp.name}/ folder:</p>
              <ul className="space-y-0.5 text-xs">{imp.files.map((f: any) => <li key={f.rel}><code>{f.rel}</code> {f.warnings.map((w: string) => <Badge key={w} tone="warn">{w}</Badge>)}</li>)}</ul>
              <Button className="mt-2" variant="primary" onClick={async () => { try { await api.ApplySkillImport(ws.id, src, agent, true); setNotice(`Imported ${imp.name}.`); setImp(null); data.reload() } catch (e) { setError(errMsg(e)) } }}>Import (files are backed up if replaced)</Button></div>)}
          </Card>
          <Card title="Agent compatibility">
            <table className="w-full text-xs"><tbody>{d.compat.map((c: any) => <tr key={c.agent} className="border-t border-line align-top"><td className="py-1 pr-2 font-medium">{c.agent}</td><td className="pr-2"><code>{c.dir}</code></td><td>{c.verified ? <Badge tone="ok">verified</Badge> : <Badge tone="warn">unverified</Badge>} <span className="text-mute">{c.note}</span></td></tr>)}</tbody></table>
          </Card>
        </div>
      </div>)}
      {install && <Confirm title={`Install ${install.id} for ${agent}`} confirmLabel="Install" onCancel={() => setInstall(null)} onConfirm={doInstall}>
        {install.previews.map((p: any) => <div key={p.relPath} className="mb-3"><p className="mb-1 text-sm">Destination: <code>{p.relPath}</code> {p.preview.exists ? <Badge tone="warn">replaces existing (backup kept)</Badge> : <Badge tone="ok">new file</Badge>}</p><DiffView diff={p.preview.diff} /></div>)}
      </Confirm>}
    </>
  )
}
