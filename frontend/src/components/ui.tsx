import { useEffect, useRef, type ReactNode, type ButtonHTMLAttributes } from 'react'
import { useStore } from '../store'

type Tone = 'neutral' | 'ok' | 'warn' | 'bad' | 'accent'
const toneCls: Record<Tone, string> = {
  neutral: 'bg-raised text-mute border-line',
  ok: 'bg-ok/10 text-ok border-ok/30',
  warn: 'bg-warn/10 text-warn border-warn/30',
  bad: 'bg-bad/10 text-bad border-bad/30',
  accent: 'bg-accent/10 text-accent border-accent/30',
}

export function Badge({ tone = 'neutral', children }: { tone?: Tone; children: ReactNode }) {
  return <span className={`inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-semibold ${toneCls[tone]}`}>{children}</span>
}

interface BtnProps extends ButtonHTMLAttributes<HTMLButtonElement> { variant?: 'primary' | 'secondary' | 'danger' | 'ghost' }
export function Button({ variant = 'secondary', className = '', ...p }: BtnProps) {
  const v = {
    primary: 'bg-accent text-white hover:brightness-110 border-transparent font-semibold',
    secondary: 'bg-raised/80 hover:bg-line/60 border-line',
    danger: 'bg-bad/15 text-bad hover:bg-bad/25 border-bad/40',
    ghost: 'bg-transparent hover:bg-raised border-transparent',
  }[variant]
  return <button type="button" {...p} className={`inline-flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm font-semibold transition-all ${v} ${className}`} />
}

export function Card({ title, actions, children, className = '' }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`glass rounded-2xl border border-line ${className}`}>
      {(title || actions) && (
        <header className="flex items-center justify-between gap-2 border-b border-line px-4 py-2.5">
          <h2 className="text-[0.95rem] font-semibold">{title}</h2>
          <div className="flex gap-2">{actions}</div>
        </header>
      )}
      <div className="p-4">{children}</div>
    </section>
  )
}

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: ReactNode; actions?: ReactNode }) {
  const ws = useStore((st) => st.open?.workspace)
  return (
    <header className="glass mb-5 flex items-start justify-between gap-4 rounded-2xl border border-line px-6 py-5">
      <div className="min-w-0">
        <nav aria-label="Breadcrumb" className="mb-2 text-xs text-mute">Workspaces{ws ? <> <span aria-hidden>/</span> {ws.name}</> : null} <span aria-hidden>/</span> <span className="text-fg">{title}</span></nav>
        <h1 className="text-2xl font-bold leading-tight">{title}</h1>
        {subtitle && <p className="mt-1.5 max-w-3xl text-sm text-mute">{subtitle}</p>}
      </div>
      <div className="flex shrink-0 flex-wrap justify-end gap-2">{actions}</div>
    </header>
  )
}

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return <div role="status" className="flex items-center gap-2 text-sm text-mute"><span className="h-3 w-3 animate-spin rounded-full border-2 border-mute border-t-transparent" />{label}</div>
}

export function ErrorBox({ error, onDismiss }: { error: string; onDismiss?: () => void }) {
  if (!error) return null
  return (
    <div role="alert" className="mb-4 flex items-start justify-between gap-3 rounded-md border border-bad/40 bg-bad/10 px-3 py-2 text-sm text-bad">
      <span className="whitespace-pre-wrap break-words">{error}</span>
      {onDismiss && <button className="text-xs underline" onClick={onDismiss}>Dismiss</button>}
    </div>
  )
}

export function Empty({ title, hint, action }: { title: string; hint?: string; action?: ReactNode }) {
  return (
    <div className="rounded-md border border-dashed border-line px-6 py-10 text-center">
      <p className="font-medium">{title}</p>
      {hint && <p className="mt-1 text-sm text-mute">{hint}</p>}
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  )
}

/** Accessible modal; Escape cancels, focus moves into the dialog. */
export function Modal({ title, children, onClose, footer, wide }: { title: string; children: ReactNode; onClose: () => void; footer?: ReactNode; wide?: boolean }) {
  const ref = useRef<HTMLDivElement>(null)
  const closeRef = useRef(onClose)
  closeRef.current = onClose
  useEffect(() => {
    // Move focus into the dialog once, unless a field inside already took it (autoFocus).
    if (!ref.current?.contains(document.activeElement)) ref.current?.focus()
    const h = (e: KeyboardEvent) => e.key === 'Escape' && closeRef.current()
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [])
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/65 p-4 backdrop-blur-sm">
      <div ref={ref} tabIndex={-1} role="dialog" aria-modal="true" aria-label={title} className={`flex max-h-[88vh] w-full ${wide ? 'max-w-5xl' : 'max-w-2xl'} flex-col rounded-xl border border-line bg-panel shadow-2xl`}>
        <h2 className="border-b border-line px-4 py-3 font-semibold">{title}</h2>
        <div className="overflow-auto p-4">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-line px-4 py-3">{footer}</div>}
      </div>
    </div>
  )
}

export function Confirm({ title, children, confirmLabel = 'Confirm', danger, onConfirm, onCancel, busy }: { title: string; children: ReactNode; confirmLabel?: string; danger?: boolean; onConfirm: () => void; onCancel: () => void; busy?: boolean }) {
  return (
    <Modal title={title} onClose={onCancel} footer={<>
      <Button onClick={onCancel}>Cancel</Button>
      <Button variant={danger ? 'danger' : 'primary'} onClick={onConfirm} disabled={busy}>{confirmLabel}</Button>
    </>}>{children}</Modal>
  )
}

export function DiffView({ diff }: { diff: string }) {
  if (!diff) return <p className="text-sm text-mute">No changes.</p>
  return (
    <pre aria-label="diff" className="max-h-80 overflow-auto rounded-md border border-line bg-bg p-2 text-xs leading-5">
      {diff.split('\n').map((l, i) => (
        <div key={i} className={l.startsWith('+') && !l.startsWith('+++') ? 'diff-add' : l.startsWith('-') && !l.startsWith('---') ? 'diff-del' : 'text-mute'}>{l || ' '}</div>
      ))}
    </pre>
  )
}

export const fmtSize = (n: number) => (n < 1024 ? `${n} B` : n < 1048576 ? `${(n / 1024).toFixed(1)} KB` : `${(n / 1048576).toFixed(1)} MB`)

export const statusTone = (s: string): Tone =>
  ({ active: 'ok', paused: 'neutral', completed: 'accent', interrupted: 'warn', needs_attention: 'bad', external_session_unavailable: 'warn',
     Detected: 'ok', 'Not installed': 'neutral', 'Needs configuration': 'warn', 'Unsupported/unknown': 'bad' } as Record<string, Tone>)[s] ?? 'neutral'

/** Segmented tab control (Spreadsheet / Timeline / Kanban style). */
export function Seg<T extends string>({ value, onChange, options, label }: { value: T; onChange: (v: T) => void; options: { id: T; label: string; icon?: ReactNode }[]; label: string }) {
  return (
    <div className="seg" role="tablist" aria-label={label}>
      {options.map((o) => (
        <button key={o.id} role="tab" aria-selected={value === o.id} onClick={() => onChange(o.id)}>{o.icon}{o.label}</button>
      ))}
    </div>
  )
}

/** Overlapping initial avatars, like an assignee stack. */
export function Avatars({ names, max = 4 }: { names: string[]; max?: number }) {
  const colors = ['#e07a5f', '#3d9970', '#4d5ee0', '#c084fc', '#f59e42', '#22b8cf']
  return (
    <span className="inline-flex -space-x-2" aria-label={names.join(', ')}>
      {names.slice(0, max).map((n, i) => (
        <span key={n + i} title={n} className="flex h-6 w-6 items-center justify-center rounded-full border-2 border-panel text-[0.625rem] font-bold text-white" style={{ background: colors[i % colors.length] }}>{n.slice(0, 1).toUpperCase()}</span>
      ))}
      {names.length > max && <span className="flex h-6 w-6 items-center justify-center rounded-full border-2 border-panel bg-raised text-[0.625rem] text-mute">+{names.length - max}</span>}
    </span>
  )
}
