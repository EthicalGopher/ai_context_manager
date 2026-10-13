import { useEffect, useMemo, useState } from 'react'
import { api, errMsg, type Scan, type ScanItem } from '../lib/api'
import { useWorkspace, useAsync } from '../lib/hooks'
import { Badge, Button, Card, Confirm, DiffView, ErrorBox, PageHeader, Spinner, fmtSize } from '../components/ui'
import { NoWorkspace } from '../components/NoWorkspace'
import { LaunchDialog } from '../components/LaunchDialog'

const steps = ['Workspace', 'Scan', 'Review', 'Mode', 'Context', 'Launch']
type Mode = 'clean' | 'refresh' | 'archive' | 'bootstrap' | 'custom'
const modes: { id: Mode; title: string; text: string }[] = [
  { id: 'clean', title: 'A. Clean Session', text: 'Start a new AI session with the existing project instructions. No project files are changed.' },
  { id: 'refresh', title: 'B. Refresh Context', text: 'Generate or update a concise project summary. Source documentation is preserved; a diff is shown before anything is replaced.' },
  { id: 'archive', title: 'C. Archive Session Notes', text: 'Move old task/session notes into an archive folder, with confirmation and undo.' },
  { id: 'bootstrap', title: 'D. Bootstrap AI Instructions', text: 'Draft an AGENTS.md (or similar) from verified project facts. Existing files are never overwritten without a diff and confirmation.' },
  { id: 'custom', title: 'E. Custom', text: 'Pick individual files and actions yourself.' },
]

export default function FreshStart() {
  const ws = useWorkspace()
  const [step, setStep] = useState(0)
  const [mode, setMode] = useState<Mode>('clean')
  const [error, setError] = useState('')
  const [excluded, setExcluded] = useState<string[]>([])
  const [task, setTask] = useState('')
  const [issues, setIssues] = useState('')
  const [next, setNext] = useState('')
  const [agent, setAgent] = useState('')
  const [launch, setLaunch] = useState(false)

  const scan = useAsync<Scan>(() => (ws ? (api.Scan(ws.id) as any) : Promise.resolve(null as any)), [ws?.id])
  const warnings = useAsync<string[]>(() => (ws ? (api.WorkspaceWarnings(ws.id) as any) : Promise.resolve([])), [ws?.id])
  const agents = useAsync(() => api.ListAgents() as Promise<any[]>, [])

  useEffect(() => {
    if (!ws) return
    api.GetWorkspaceState(ws.id).then((s: any) => { setExcluded(s.excluded ?? []); setTask(s.task ?? ''); setIssues(s.knownIssues ?? ''); setNext(s.nextSteps ?? '') }).catch(() => {})
    api.GetSettings().then((s: any) => setAgent((a) => a || ws.agent || s.defaultAgent)).catch(() => {})
  }, [ws?.id])

  const persist = (ex = excluded) => ws && api.SaveWorkspaceState(ws.id, { task, knownIssues: issues, nextSteps: next, excluded: ex, mode } as any, agent).catch((e) => setError(errMsg(e)))
  const toggleExclude = (p: string) => { const ex = excluded.includes(p) ? excluded.filter((x) => x !== p) : [...excluded, p]; setExcluded(ex); persist(ex) }

  if (!ws) return <><PageHeader title="Fresh Start" /><NoWorkspace /></>
  const s = scan.data

  return (
    <>
      <PageHeader title="Fresh Start" subtitle="Refresh the AI session and its working context. Nothing is deleted or overwritten without a preview and your confirmation." />
      <ol className="mb-5 flex flex-wrap gap-2" aria-label="Progress">
        {steps.map((n, i) => (
          <li key={n}><button onClick={() => setStep(i)} aria-current={i === step ? 'step' : undefined}
            className={`rounded-full border px-3 py-1 text-xs ${i === step ? 'border-accent bg-accent/15 text-accent' : i < step ? 'border-ok/40 text-ok' : 'border-line text-mute'}`}>{i + 1}. {n}</button></li>
        ))}
      </ol>
      <ErrorBox error={error || scan.error} onDismiss={() => setError('')} />

      {step === 0 && (
        <Card title="Step 1 · Select workspace">
          <dl className="grid grid-cols-[9rem_1fr] gap-y-1.5 text-sm">
            <dt className="text-mute">Path</dt><dd><code>{ws.path}</code></dd>
            <dt className="text-mute">Git</dt><dd>{s ? (s.git.isRepo ? <>branch <code>{s.git.branch}</code>, {s.git.dirty} changed</> : s.git.message) : <Spinner />}</dd>
            <dt className="text-mute">Directory state</dt><dd>{s?.state ?? '…'}</dd>
          </dl>
          {(warnings.data ?? []).map((w) => <p key={w} className="mt-3 rounded-md border border-warn/40 bg-warn/10 px-3 py-2 text-sm text-warn">{w}</p>)}
        </Card>
      )}

      {step === 1 && (
        <Card title="Step 2 · Scan and classify (read-only)">
          {scan.loading && <Spinner label="Scanning…" />}
          {s && (<>
            <div className="mb-3 flex flex-wrap gap-2">{Object.entries(s.items.reduce<Record<string, number>>((m, i) => ({ ...m, [i.category]: (m[i.category] ?? 0) + 1 }), {})).map(([c, n]) => <Badge key={c} tone="accent">{c}: {n}</Badge>)}</div>
            <p className="text-sm text-mute">Ignored by default: node_modules, .git, vendor, dist, build, caches, and .gitignore entries. Symlinks leaving the workspace are skipped.</p>
            {s.truncated && <p className="mt-2 text-sm text-warn">File-count limit reached; the inventory is incomplete.</p>}
            {s.warnings.map((w) => <p key={w} className="mt-1 text-xs text-warn">{w}</p>)}
            <Button className="mt-3" onClick={scan.reload}>Rescan</Button>
          </>)}
        </Card>
      )}

      {step === 2 && (
        <Card title="Step 3 · Review the current context">
          {!s ? <Spinner /> : s.items.length === 0 ? <p className="text-sm text-mute">No guidance or Markdown files found.</p> : (
            <div className="overflow-auto">
              <table className="w-full text-left text-sm">
                <thead className="text-xs uppercase text-mute"><tr><th className="py-1 pr-3">Include</th><th className="pr-3">Path</th><th className="pr-3">Class</th><th className="pr-3">Modified</th><th>Proposed</th></tr></thead>
                <tbody>{s.items.map((i: ScanItem) => (
                  <tr key={i.path} className="border-t border-line align-top">
                    <td className="py-1.5 pr-3"><input type="checkbox" aria-label={`Include ${i.path}`} disabled={i.secret} checked={!i.secret && !excluded.includes(i.path)} onChange={() => toggleExclude(i.path)} /></td>
                    <td className="pr-3"><code>{i.path}</code><div className="text-xs text-mute">{fmtSize(i.size)}</div>{i.warning && <div className="text-xs text-warn">⚠ {i.warning}</div>}</td>
                    <td className="pr-3"><Badge>{i.category}</Badge></td>
                    <td className="pr-3 text-xs text-mute">{new Date(i.modTime).toLocaleDateString()}</td>
                    <td><Badge tone={i.action === 'exclude' ? 'warn' : 'ok'}>{excluded.includes(i.path) ? 'exclude (you)' : i.action}</Badge></td>
                  </tr>))}</tbody>
              </table>
            </div>
          )}
          {s?.duplicates.map((d) => <p key={d} className="mt-2 text-sm text-warn">Possible duplicate instructions: {d}</p>)}
          <p className="mt-3 text-xs text-mute">Secret detection is heuristic, not perfect. Excluding a file only keeps it out of the context package; it is never deleted.</p>
        </Card>
      )}

      {step === 3 && (
        <Card title="Step 4 · Choose a mode">
          <div role="radiogroup" className="grid gap-2 md:grid-cols-2">
            {modes.map((m) => (
              <label key={m.id} className={`cursor-pointer rounded-md border p-3 ${mode === m.id ? 'border-accent bg-accent/10' : 'border-line hover:bg-raised'}`}>
                <input type="radio" name="mode" className="mr-2" checked={mode === m.id} onChange={() => setMode(m.id)} />
                <strong>{m.title}</strong><p className="mt-1 text-sm text-mute">{m.text}</p>
              </label>
            ))}
          </div>
        </Card>
      )}

      {step === 4 && ws && <ContextStep wsId={ws.id} mode={mode} items={s?.items ?? []} task={task} setTask={setTask} issues={issues} setIssues={setIssues} next={next} setNext={setNext} onSave={() => persist()} setError={setError} excluded={excluded} />}

      {step === 5 && (
        <Card title="Step 6 · Launch or export">
          <div className="mb-3 flex items-center gap-2">
            <label htmlFor="agent" className="text-sm text-mute">Agent</label>
            <select id="agent" value={agent} onChange={(e) => setAgent(e.target.value)} className="rounded-md border border-line bg-bg px-2 py-1.5">
              {(agents.data ?? []).map((a: any) => <option key={a.profile.id} value={a.profile.id}>{a.profile.displayName} — {a.status.state}{a.profile.verified ? '' : ' (unverified)'}</option>)}
            </select>
          </div>
          <p className="mb-3 text-sm text-mute">Launching opens the real CLI in a terminal. If it is not installed or cannot start, you will see why — AICW never simulates an AI response.</p>
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" onClick={() => { persist(); setLaunch(true) }}>Review &amp; launch…</Button>
            <Button onClick={async () => { try { const p: any = await api.BuildContext({ workspaceId: ws.id, task, knownIssues: issues, nextSteps: next } as any); await api.CopyToClipboard(p.markdown) } catch (e) { setError(errMsg(e)) } }}>Copy context</Button>
            <Button onClick={async () => { try { await api.ExportContext({ workspaceId: ws.id, task, knownIssues: issues, nextSteps: next } as any) } catch (e) { setError(errMsg(e)) } }}>Export as .md…</Button>
          </div>
        </Card>
      )}

      <div className="mt-5 flex justify-between">
        <Button onClick={() => setStep((x) => Math.max(0, x - 1))} disabled={step === 0}>Back</Button>
        <Button variant="primary" onClick={() => { persist(); setStep((x) => Math.min(steps.length - 1, x + 1)) }} disabled={step === steps.length - 1}>Next</Button>
      </div>
      {launch && <LaunchDialog params={{ workspaceId: ws.id, agent, mode: 'new', title: task.slice(0, 60), task, knownIssues: issues, nextSteps: next }} onClose={() => setLaunch(false)} onLaunched={() => {}} />}
    </>
  )
}

function ContextStep(p: { wsId: number; mode: Mode; items: ScanItem[]; task: string; setTask: (s: string) => void; issues: string; setIssues: (s: string) => void; next: string; setNext: (s: string) => void; onSave: () => void; setError: (s: string) => void; excluded: string[] }) {
  const [pkg, setPkg] = useState<any>(null)
  const [busy, setBusy] = useState(false)
  const build = async () => {
    setBusy(true)
    try { p.onSave(); setPkg(await api.BuildContext({ workspaceId: p.wsId, task: p.task, knownIssues: p.issues, nextSteps: p.next } as any)) } catch (e) { p.setError(errMsg(e)) } finally { setBusy(false) }
  }
  useEffect(() => { build() /* eslint-disable-next-line */ }, [p.excluded.join('|')])
  const field = (label: string, v: string, set: (s: string) => void) => (
    <label className="block text-sm"><span className="text-mute">{label}</span>
      <textarea value={v} onChange={(e) => set(e.target.value)} rows={2} className="mt-1 w-full rounded-md border border-line bg-bg px-3 py-1.5" /></label>
  )
  return (
    <div className="space-y-4">
      <Card title="Step 5 · Task and constraints">
        <div className="grid gap-3 md:grid-cols-3">{field('Current task & constraints', p.task, p.setTask)}{field('Known issues', p.issues, p.setIssues)}{field('Next steps', p.next, p.setNext)}</div>
        <Button className="mt-3" onClick={build} disabled={busy}>Rebuild preview</Button>
      </Card>
      {(p.mode === 'refresh' || p.mode === 'custom') && pkg && <SaveFile wsId={p.wsId} title="Save project summary into the project" defaultRel=".aicw/context/project-summary.md" content={pkg.markdown} setError={p.setError} />}
      {(p.mode === 'bootstrap' || p.mode === 'custom') && <Bootstrap wsId={p.wsId} setError={p.setError} />}
      {(p.mode === 'archive' || p.mode === 'custom') && <Archive wsId={p.wsId} items={p.items} setError={p.setError} />}
      <Card title="Context package preview" actions={pkg && <><Badge tone="accent">~{pkg.tokens} / {pkg.budget} tokens</Badge></>}>
        {busy && <Spinner />}
        {pkg && (<>
          <div className="mb-3 grid gap-3 md:grid-cols-2 text-xs">
            <div><h3 className="mb-1 font-semibold uppercase text-mute">Included</h3>{pkg.included.map((f: any) => <div key={f.path}><code>{f.path}</code> <span className="text-mute">~{f.tokens}</span></div>)}{!pkg.included.length && <span className="text-mute">none</span>}</div>
            <div><h3 className="mb-1 font-semibold uppercase text-mute">Excluded</h3>{pkg.excluded.map((f: any) => <div key={f.path}><code>{f.path}</code> <span className="text-warn">{f.reason}</span></div>)}{!pkg.excluded.length && <span className="text-mute">none</span>}</div>
          </div>
          <pre className="max-h-96 overflow-auto rounded-md border border-line bg-bg p-3 text-xs whitespace-pre-wrap">{pkg.markdown}</pre>
        </>)}
      </Card>
    </div>
  )
}

function SaveFile({ wsId, title, defaultRel, content, setError }: { wsId: number; title: string; defaultRel: string; content: string; setError: (s: string) => void }) {
  const [rel, setRel] = useState(defaultRel)
  const [pv, setPv] = useState<any>(null)
  const [saved, setSaved] = useState('')
  const preview = async () => { setSaved(''); try { setPv(await api.PreviewWrite(wsId, rel, content)) } catch (e) { setError(errMsg(e)) } }
  const apply = async () => {
    try { await api.ApplyWrite(wsId, rel, content, pv.baseHash, true); setSaved(`Saved ${rel}${pv.exists ? ' (previous version backed up)' : ''}.`); setPv(null) } catch (e) { setError(errMsg(e)); setPv(null) }
  }
  return (
    <Card title={title}>
      <div className="flex gap-2">
        <label className="sr-only" htmlFor={`rel-${defaultRel}`}>Target file</label>
        <input id={`rel-${defaultRel}`} value={rel} onChange={(e) => setRel(e.target.value)} className="flex-1 rounded-md border border-line bg-bg px-3 py-1.5 font-mono" />
        <Button onClick={preview}>Preview diff</Button>
      </div>
      {saved && <p role="status" className="mt-2 text-sm text-ok">{saved}</p>}
      {pv && <Confirm title={pv.exists ? `Replace ${rel}?` : `Create ${rel}?`} confirmLabel={pv.exists ? 'Back up & replace' : 'Create file'} onConfirm={apply} onCancel={() => setPv(null)}>
        <p className="mb-2 text-sm text-mute">{pv.changed ? 'Review the changes below.' : 'The file already has this content.'}</p><DiffView diff={pv.diff} />
      </Confirm>}
    </Card>
  )
}

function Bootstrap({ wsId, setError }: { wsId: number; setError: (s: string) => void }) {
  const [draft, setDraft] = useState('')
  const [target, setTarget] = useState('AGENTS.md')
  useEffect(() => { api.InstructionDraft(wsId).then(setDraft).catch((e) => setError(errMsg(e))) }, [wsId])
  return (
    <div className="space-y-3">
      <Card title="Draft instruction file (edit before saving)">
        <select aria-label="Instruction file" value={target} onChange={(e) => setTarget(e.target.value)} className="mb-2 rounded-md border border-line bg-bg px-2 py-1.5">
          {['AGENTS.md', 'CLAUDE.md', 'GEMINI.md'].map((f) => <option key={f}>{f}</option>)}
        </select>
        <textarea aria-label="Draft content" value={draft} onChange={(e) => setDraft(e.target.value)} rows={12} className="mono w-full rounded-md border border-line bg-bg p-3 text-xs" />
      </Card>
      <SaveFile key={target} wsId={wsId} title={`Save ${target}`} defaultRel={target} content={draft} setError={setError} />
    </div>
  )
}

function Archive({ wsId, items, setError }: { wsId: number; items: ScanItem[]; setError: (s: string) => void }) {
  const candidates = useMemo(() => items.filter((i) => ['task-notes', 'session-summary'].includes(i.category) && !i.path.startsWith('.aicw/archive/')), [items])
  const [sel, setSel] = useState<string[]>([])
  const [plan, setPlan] = useState<any[] | null>(null)
  const [done, setDone] = useState<any[]>([])
  return (
    <Card title="Archive session notes">
      {candidates.length === 0 ? <p className="text-sm text-mute">No task or session notes found.</p> : (
        <ul className="space-y-1 text-sm">{candidates.map((c) => (
          <li key={c.path}><label><input type="checkbox" className="mr-2" checked={sel.includes(c.path)} onChange={() => setSel((s) => (s.includes(c.path) ? s.filter((x) => x !== c.path) : [...s, c.path]))} /><code>{c.path}</code></label></li>))}</ul>
      )}
      <div className="mt-3 flex gap-2">
        <Button disabled={!sel.length} onClick={async () => { try { setPlan((await api.ArchivePlan(wsId, sel)) as any) } catch (e) { setError(errMsg(e)) } }}>Plan archive…</Button>
        {done.length > 0 && <Button onClick={async () => { try { await api.ApplyMoves(wsId, done.map((m) => ({ from: m.to, to: m.from })) as any, true); setDone([]) } catch (e) { setError(errMsg(e)) } }}>Undo last archive</Button>}
      </div>
      {plan && <Confirm title="Archive these files?" confirmLabel="Move to archive" onCancel={() => setPlan(null)}
        onConfirm={async () => { try { await api.ApplyMoves(wsId, plan as any, true); setDone(plan); setPlan(null); setSel([]) } catch (e) { setError(errMsg(e)); setPlan(null) } }}>
        <p className="mb-2 text-sm text-mute">Files are moved (not deleted) and a backup copy is kept. You can undo this.</p>
        <ul className="space-y-1 text-sm">{plan.map((m) => <li key={m.from}><code>{m.from}</code> → <code>{m.to}</code></li>)}</ul>
      </Confirm>}
    </Card>
  )
}
