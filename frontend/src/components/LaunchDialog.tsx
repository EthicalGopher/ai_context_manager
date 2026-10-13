import { useEffect, useState } from 'react'
import { api, errMsg } from '../lib/api'
import { Badge, Button, ErrorBox, Modal, Spinner } from './ui'

export interface LaunchParams {
  workspaceId: number; agent?: string; mode: 'new' | 'resume'; sessionId?: number
  title?: string; task?: string; knownIssues?: string; nextSteps?: string
}

/** Shows exactly what will run (dry run), discloses data flow, then launches. */
export function LaunchDialog({ params, onClose, onLaunched }: { params: LaunchParams; onClose: () => void; onLaunched: () => void }) {
  const [plan, setPlan] = useState<any>(null)
  const [busy, setBusy] = useState(true)
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)

  const base = { workspaceId: params.workspaceId, agent: params.agent ?? '', mode: params.mode, sessionId: params.sessionId ?? 0, title: params.title ?? '', task: params.task ?? '', knownIssues: params.knownIssues ?? '', nextSteps: params.nextSteps ?? '' }

  useEffect(() => {
    api.Launch({ ...base, dryRun: true, force: false } as any)
      .then(setPlan).catch((e) => setError(errMsg(e))).finally(() => setBusy(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const go = async (force: boolean) => {
    setBusy(true); setError('')
    try {
      const r: any = await api.Launch({ ...base, dryRun: false, force } as any)
      setPlan(r)
      if (r.launched) { setDone(true); onLaunched() }
    } catch (e) { setError(errMsg(e)) } finally { setBusy(false) }
  }

  const spec = plan?.spec
  const blocked = !!plan?.error
  const running = blocked && /already/i.test(plan.error)
  return (
    <Modal title={params.mode === 'resume' ? 'Resume session' : 'Start new session'} onClose={onClose}
      footer={<>
        <Button onClick={onClose}>{done ? 'Close' : 'Cancel'}</Button>
        {running && <Button variant="danger" onClick={() => go(true)} disabled={busy}>Launch anyway</Button>}
        {!done && <Button variant="primary" onClick={() => go(false)} disabled={busy || (blocked && !running) || !spec}>Launch in terminal</Button>}
      </>}>
      <ErrorBox error={error} />
      {busy && !plan && <Spinner label="Preparing launch plan…" />}
      {plan && (
        <div className="space-y-3 text-sm">
          {plan.disclosure && <p className="rounded-md border border-warn/40 bg-warn/10 p-2 text-warn">{plan.disclosure}</p>}
          {plan.error && <ErrorBox error={plan.error} />}
          {plan.fallback && <p className="text-mute">{plan.fallback}</p>}
          {spec && (<>
            <div><span className="text-mute">Working directory:</span> <code>{spec.dir}</code></div>
            <div><span className="text-mute">Command (shell-free argument list):</span>
              <pre className="mt-1 overflow-auto rounded border border-line bg-bg p-2 text-xs whitespace-pre-wrap">{spec.display}</pre></div>
            <div className="flex gap-2"><Badge tone="accent">context handoff: {spec.handoff}</Badge>{plan.tokens > 0 && <Badge>~{plan.tokens} tokens</Badge>}</div>
            <ul className="list-disc space-y-1 pl-5 text-mute">{(spec.notes ?? []).map((n: string, i: number) => <li key={i}>{n}</li>)}</ul>
            {spec.display && <Button onClick={() => api.CopyToClipboard(spec.display)}>Copy command</Button>}
          </>)}
          {done && <p role="status" className="font-medium text-ok">Launched. Session recorded; AICW does not start any agent without this confirmation.</p>}
        </div>
      )}
    </Modal>
  )
}
