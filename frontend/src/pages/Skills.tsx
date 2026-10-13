import { useEffect, useState } from 'react'
import { FiPlus, FiTrash2, FiCheck } from 'react-icons/fi'
import { api, errMsg, fmtTime } from '../lib/api'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Badge, Button, Card, Confirm, DiffView, Empty, ErrorBox, PageHeader, Seg, Spinner } from '../components/ui'
import { NoWorkspace } from '../components/NoWorkspace'
import SkillFiles from '../components/SkillFiles'

type Tab = 'skills' | 'notes' | 'files'
interface Skill { id: number; name: string; description: string; body: string; alwaysOn: boolean; updatedBy: string; updatedAt: string }
interface Note { name: string; body: string; updatedBy: string; updatedAt: string }

export default function Skills() {
  const ws = useWorkspace()
  const [tab, setTab] = useState<Tab>('skills')
  if (!ws) return <><PageHeader title="Skills & memory" /><NoWorkspace /></>
  return (
    <>
      <PageHeader title="Skills & memory" subtitle="Rules your AI must always follow, and living notes it reads at the start and keeps updated. The AI can add to both itself through AICW's tools."
        actions={<Seg<Tab> label="Sections" value={tab} onChange={setTab} options={[{ id: 'skills', label: 'Always-on skills' }, { id: 'notes', label: 'Notes' }, { id: 'files', label: 'Project skill files' }]} />} />
      {tab === 'skills' && <AlwaysOn wsId={ws.id} />}
      {tab === 'notes' && <Notes wsId={ws.id} />}
      {tab === 'files' && <SkillFiles />}
    </>
  )
}

function AlwaysOn({ wsId }: { wsId: number }) {
  const list = useAsync<Skill[]>(() => api.ListAgentSkills(wsId) as any, [wsId])
  const status = useAsync<any[]>(() => api.InstructionsStatus(wsId) as any, [wsId])
  const starters = useAsync<Skill[]>(() => api.StarterSkills() as any, [])
  const [edit, setEdit] = useState<Partial<Skill> | null>(null)
  const [del, setDel] = useState<Skill | null>(null)
  const [apply, setApply] = useState<{ file: string; pv: any } | null>(null)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const reload = () => { list.reload(); status.reload() }
  const skills = list.data ?? []
  const missing = (starters.data ?? []).filter((s) => !skills.some((k) => k.name === s.name))

  const save = async () => {
    if (!edit) return
    try { await api.SaveAgentSkill(wsId, edit.name ?? '', edit.description ?? '', edit.body ?? '', edit.alwaysOn ?? true); setEdit(null); reload() } catch (e) { setError(errMsg(e)) }
  }
  const preview = async (file: string) => { try { setApply({ file, pv: await api.PreviewInstructions(wsId, file) }) } catch (e) { setError(errMsg(e)) } }

  return (
    <>
      <ErrorBox error={error || list.error} onDismiss={() => setError('')} />
      {notice && <p role="status" className="mb-3 text-sm text-ok">{notice}</p>}
      <div className="grid gap-4 xl:grid-cols-[1fr_22rem]">
        <Card title="Skills" actions={<Button variant="primary" onClick={() => setEdit({ alwaysOn: true })}><FiPlus aria-hidden /> New skill</Button>}>
          {list.loading && <Spinner />}
          {!list.loading && skills.length === 0 && <Empty title="No skills yet" hint="Skills are instructions the AI must follow every session, like “read the project memory first”." />}
          <ul className="space-y-3">
            {skills.map((k) => (
              <li key={k.id} className="rounded-xl border border-line bg-raised/50 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div><h3 className="flex flex-wrap items-center gap-2 text-base">{k.name} <Badge tone={k.alwaysOn ? 'ok' : 'neutral'}>{k.alwaysOn ? 'always on' : 'on demand'}</Badge>{k.updatedBy === 'agent' && <Badge tone="accent">written by agent</Badge>}</h3>
                    {k.description && <p className="text-sm text-mute">{k.description}</p>}</div>
                  <div className="flex gap-1"><Button variant="ghost" onClick={() => setEdit(k)}>Edit</Button><Button variant="ghost" aria-label={`Delete ${k.name}`} onClick={() => setDel(k)}><FiTrash2 aria-hidden /></Button></div>
                </div>
                <pre className="mt-2 max-h-28 overflow-auto whitespace-pre-wrap text-xs text-mute">{k.body}</pre>
                <p className="mt-1 text-xs text-mute">Updated {fmtTime(k.updatedAt)} by {k.updatedBy}</p>
              </li>))}
          </ul>
          {missing.length > 0 && (
            <div className="mt-4 rounded-xl border border-dashed border-line p-4">
              <p className="mb-2 text-sm font-bold">Suggested starter skills</p>
              <ul className="space-y-2">{missing.map((s) => (
                <li key={s.name} className="flex items-center justify-between gap-3 text-sm"><span><b>{s.name}</b> <span className="text-mute">— {s.description}</span></span>
                  <Button onClick={async () => { try { await api.SaveAgentSkill(wsId, s.name, s.description, s.body, s.alwaysOn); reload() } catch (e) { setError(errMsg(e)) } }}>Add</Button></li>))}</ul>
            </div>)}
        </Card>

        <div className="space-y-4">
          <Card title="Make the AI always read it">
            <p className="mb-3 text-sm text-mute">Install your always-on skills into the project's instruction file. Agents read these files at the start of every session. Only a clearly marked block is added or replaced; the rest of the file is untouched, and you see a diff first.</p>
            <ul className="space-y-2">
              {(status.data ?? []).map((f) => (
                <li key={f.file} className="flex items-center justify-between gap-2 text-sm">
                  <span><code>{f.file}</code> {!f.exists ? <span className="text-mute">(new)</span> : f.current ? <Badge tone="ok"><FiCheck aria-hidden /> up to date</Badge> : f.hasBlock ? <Badge tone="warn">outdated</Badge> : null}</span>
                  <Button disabled={f.current} onClick={() => preview(f.file)}>{f.hasBlock ? 'Update…' : 'Install…'}</Button></li>))}
            </ul>
            <p className="mt-3 text-xs text-mute">Claude Code also gets these rules and the AICW tools automatically when launched from the terminal panel.</p>
          </Card>
          <Card title="What the AI can do by itself">
            <ToolsInfo />
          </Card>
        </div>
      </div>

      {edit && (
        <Confirm title={edit.id ? `Edit ${edit.name}` : 'New skill'} confirmLabel="Save skill" onCancel={() => setEdit(null)} onConfirm={save}>
          <div className="space-y-3 text-sm">
            <label className="block"><span className="text-mute">Name</span><input aria-label="Skill name" value={edit.name ?? ''} disabled={!!edit.id} onChange={(e) => setEdit({ ...edit, name: e.target.value })} className="mt-1 w-full rounded-lg border border-line bg-bg px-3 py-2" placeholder="read-context-first" /></label>
            <label className="block"><span className="text-mute">Short description</span><input aria-label="Skill description" value={edit.description ?? ''} onChange={(e) => setEdit({ ...edit, description: e.target.value })} className="mt-1 w-full rounded-lg border border-line bg-bg px-3 py-2" /></label>
            <label className="block"><span className="text-mute">Instructions the AI must follow</span><textarea aria-label="Skill instructions" rows={8} value={edit.body ?? ''} onChange={(e) => setEdit({ ...edit, body: e.target.value })} className="mono mt-1 w-full rounded-lg border border-line bg-bg px-3 py-2 text-xs" /></label>
            <label className="flex items-center gap-2"><input type="checkbox" checked={edit.alwaysOn ?? true} onChange={(e) => setEdit({ ...edit, alwaysOn: e.target.checked })} /> Always on (written into every session's instructions)</label>
          </div>
        </Confirm>)}
      {del && <Confirm title="Delete skill?" danger confirmLabel="Delete" onCancel={() => setDel(null)} onConfirm={async () => { try { await api.DeleteAgentSkill(wsId, del.name, true); setDel(null); reload() } catch (e) { setError(errMsg(e)); setDel(null) } }}><p className="text-sm">Removes “{del.name}” from AICW. Instruction files you already installed keep their copy until you update them.</p></Confirm>}
      {apply && <Confirm title={`Update ${apply.file}?`} confirmLabel={apply.pv.exists ? 'Back up & update' : 'Create file'} onCancel={() => setApply(null)}
        onConfirm={async () => { try { await api.ApplyInstructions(wsId, apply.file, apply.pv.baseHash, true); setNotice(`${apply.file} updated. The previous version is in the backup history.`); setApply(null); reload() } catch (e) { setError(errMsg(e)); setApply(null) } }}>
        <p className="mb-2 text-sm text-mute">Only the block between the AICW markers changes.</p><DiffView diff={apply.pv.diff} /></Confirm>}
    </>
  )
}

function ToolsInfo() {
  const rows: [string, string][] = [
    ['read_context', 'load notes and skills at the start'],
    ['write_note / append_note', 'keep project memory current (versions kept)'],
    ['add_skill', 'propose a new rule for future sessions'],
    ['create_conversation / add_message', 'start chats and post updates, with images, audio, video, PDFs'],
    ['add_media / list_media', 'store files in the media library'],
  ]
  return (
    <div className="space-y-2 text-sm">
      <ul className="space-y-1.5">{rows.map(([n, d]) => <li key={n}><code>{n}</code> <span className="text-mute">— {d}</span></li>)}</ul>
      <p className="text-xs text-mute">Available to agents as an MCP server (<code>aicw mcp</code>) and as <code>aicw tool &lt;name&gt; '&lt;json&gt;'</code> in the terminal. Agents can only add or update AICW's own data. They cannot type into your terminal, start other programs through these tools, or forge your messages.</p>
    </div>
  )
}

function Notes({ wsId }: { wsId: number }) {
  const list = useAsync<Note[]>(() => api.ListNotes(wsId) as any, [wsId])
  const [sel, setSel] = useState('')
  const [body, setBody] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [del, setDel] = useState('')
  const versions = useAsync<any[]>(() => (sel ? (api.NoteVersions(wsId, sel) as any) : Promise.resolve([])), [wsId, sel, saved])
  const notes = list.data ?? []
  useEffect(() => { const n = notes.find((x) => x.name === sel); if (n && !saved) setBody(n.body) }, [sel, list.data])  // eslint-disable-line react-hooks/exhaustive-deps

  const save = async (n: string, b: string) => { try { await api.SaveNote(wsId, n, b); setSaved(true); setSel(n); list.reload(); setTimeout(() => setSaved(false), 1500) } catch (e) { setError(errMsg(e)) } }
  const current = notes.find((n) => n.name === sel)
  return (
    <>
      <ErrorBox error={error || list.error} onDismiss={() => setError('')} />
      <div className="grid gap-4 lg:grid-cols-[16rem_1fr]">
        <Card title="Notes">
          {list.loading && <Spinner />}
          <ul className="space-y-1">{notes.map((n) => (
            <li key={n.name}><button className={`w-full rounded-lg px-3 py-2 text-left ${sel === n.name ? 'bg-raised' : 'hover:bg-raised/60'}`} onClick={() => { setSel(n.name); setBody(n.body) }}><div className="truncate font-bold">{n.name}</div><div className="text-xs text-mute">by {n.updatedBy} · {fmtTime(n.updatedAt)}</div></button></li>))}</ul>
          {!list.loading && notes.length === 0 && <p className="text-sm text-mute">No notes yet. The AI creates <code>project-memory</code> when it follows the starter skills, or add one yourself.</p>}
          <form className="mt-3 flex gap-2" onSubmit={(e) => { e.preventDefault(); if (name.trim()) { save(name.trim(), '# ' + name.trim() + '\n'); setName(''); setBody('# ' + name.trim() + '\n') } }}>
            <input aria-label="New note name" value={name} onChange={(e) => setName(e.target.value)} placeholder="new-note-name" className="min-w-0 flex-1 rounded-lg border border-line bg-bg px-2 py-1.5" /><Button type="submit" disabled={!name.trim()}>Add</Button></form>
        </Card>
        <Card title={sel || 'Select a note'} actions={sel && <><Button variant="primary" onClick={() => save(sel, body)}>Save</Button><Button variant="ghost" aria-label="Delete note" onClick={() => setDel(sel)}><FiTrash2 aria-hidden /></Button></>}>
          {!sel ? <Empty title="Pick or create a note" hint="Notes are living documents the AI reads at the start of each session and updates as it works." /> : (<>
            <textarea aria-label="Note body" value={body} onChange={(e) => setBody(e.target.value)} rows={16} className="mono w-full rounded-lg border border-line bg-bg p-3 text-xs" />
            {saved && <p role="status" className="mt-1 text-sm text-ok">Saved.</p>}
            {current && <p className="mt-1 text-xs text-mute">Last updated by {current.updatedBy}, {fmtTime(current.updatedAt)}</p>}
            {(versions.data ?? []).length > 0 && (
              <details className="mt-3"><summary className="cursor-pointer text-sm text-mute">Version history ({versions.data!.length})</summary>
                <ul className="mt-2 space-y-1 text-xs">{versions.data!.map((v) => <li key={v.id} className="flex justify-between gap-2"><span>{fmtTime(v.savedAt)} · {v.savedBy}</span><button className="text-accent underline" onClick={() => setBody(v.body)}>Load into editor</button></li>)}</ul></details>)}
          </>)}
        </Card>
      </div>
      {del && <Confirm title="Delete note?" danger confirmLabel="Delete" onCancel={() => setDel('')} onConfirm={async () => { try { await api.DeleteNote(wsId, del, true); setDel(''); setSel(''); list.reload() } catch (e) { setError(errMsg(e)); setDel('') } }}><p className="text-sm">Deletes “{del}”. This cannot be undone.</p></Confirm>}
    </>
  )
}
