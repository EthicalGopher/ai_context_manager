import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { OpenResult, Session } from '../lib/api'
import { fmtTime } from '../lib/api'
import { Badge, Button, Card, statusTone } from './ui'
import { LaunchDialog, type LaunchParams } from './LaunchDialog'

/** Resume-first entry point: never starts anything without a click and confirmation. */
export function ResumePanel({ open, onChanged }: { open: OpenResult; onChanged: () => void }) {
  const [launch, setLaunch] = useState<LaunchParams | null>(null)
  const ws = open.workspace
  const row = (s: Session) => (
    <li key={s.id} className="flex items-center justify-between gap-3 rounded-md border border-line bg-raised px-3 py-2">
      <div className="min-w-0">
        <div className="flex items-center gap-2"><strong>{s.title || `Session #${s.id}`}</strong><Badge>{s.agent}</Badge><Badge tone={statusTone(s.status)}>{s.status}</Badge></div>
        <div className="mt-0.5 truncate text-xs text-mute">Last active {fmtTime(s.lastActive)}{s.handoff ? ` · ${s.handoff.split('\n').find((l) => l.trim() && !l.startsWith('<!--') && !l.startsWith('#'))?.slice(0, 100) ?? ''}` : ''}</div>
        {!s.externalId && <div className="mt-0.5 text-xs text-warn">No saved CLI session ID: only AICW's own context can be restored, not the CLI's conversation.</div>}
      </div>
      <Button variant="primary" onClick={() => setLaunch({ workspaceId: ws.id, mode: 'resume', sessionId: s.id })}>Resume</Button>
    </li>
  )
  return (
    <Card title={open.action === 'choose' ? 'Choose a session to resume' : open.action === 'resume' ? 'Saved session found' : 'No saved session'}
      actions={<Link className="rounded-md border border-line bg-raised px-3 py-1.5 text-sm hover:bg-line/60" to="/fresh-start">New Session…</Link>}>
      <p className="mb-3 text-sm text-mute">{open.reason}. Nothing starts until you confirm.</p>
      {open.resumable.length > 0 ? <ul className="space-y-2">{open.resumable.map(row)}</ul>
        : <p className="text-sm">Use <strong>Fresh Start</strong> to prepare context and begin a session in this workspace.</p>}
      {launch && <LaunchDialog params={launch} onClose={() => setLaunch(null)} onLaunched={onChanged} />}
    </Card>
  )
}
