// Thin typed layer over the generated Wails bindings. Components import from
// here so tests can mock one module.
import * as S from '../../wailsjs/go/service/Service'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export const api = S
export const onEvent = (name: string, cb: (data: any) => void): (() => void) => {
  try {
    return EventsOn(name, cb)
  } catch {
    return () => {}
  }
}

// Plain-object shapes used by the UI (Wails models are structurally compatible).
export interface Workspace { id: number; path: string; name: string; agent: string; settings: string; lastOpened: string }
export interface Session {
  id: number; workspaceId: number; agent: string; externalId: string; status: string
  title: string; handoff: string; pid: number; createdAt: string; lastActive: string
}
export interface OpenResult { workspace: Workspace; sessions: Session[]; resumable: Session[]; action: 'resume' | 'choose' | 'new'; isNew: boolean; requestedNew: boolean; page?: string; reason: string }
export interface ScanItem { path: string; category: string; size: number; modTime: string; secret: boolean; injection: boolean; tooLarge: boolean; action: string; warning?: string }
export interface Scan { root: string; items: ScanItem[]; git: { isRepo: boolean; branch: string; dirty: number; message?: string }; state: string; warnings: string[]; truncated: boolean; duplicates: string[]; tree: string[] }

export const errMsg = (e: unknown): string => (e instanceof Error ? e.message : typeof e === 'string' ? e : JSON.stringify(e))
export const fmtTime = (t: string | Date | undefined): string => {
  if (!t) return '—'
  const d = new Date(t)
  return isNaN(d.getTime()) ? '—' : d.toLocaleString()
}
