import { create } from 'zustand'
import type { OpenResult } from './lib/api'

export interface Conv { id: number; workspaceId: number; title: string; agent: string; sessionId: number; messages: number; updatedAt: string }

export type Theme = 'dark' | 'light'

/** Text size: scales the root font size, so everything sized in rem (text, spacing, widths) grows together. */
export const TEXT_SCALES = [0.8, 0.9, 1, 1.1, 1.25, 1.4, 1.6, 1.8, 2]
const nearestScale = (v: number) => TEXT_SCALES.reduce((a, b) => (Math.abs(b - v) < Math.abs(a - v) ? b : a), 1)
const safeScale = (): number => {
  try { const v = Number(localStorage.getItem('aicw-text-scale')); return v > 0 ? nearestScale(v) : 1 } catch { return 1 }
}
const applyScale = (v: number) => { document.documentElement.style.fontSize = `${16 * v}px` }
applyScale(safeScale())

interface State {
  open: OpenResult | null
  convs: Conv[]
  setConvs: (c: Conv[]) => void
  theme: Theme
  textScale: number
  stepText: (dir: -1 | 0 | 1) => void
  error: string
  notice: string
  setOpen: (o: OpenResult | null) => void
  setTheme: (t: Theme) => void
  setError: (e: string) => void
  setNotice: (e: string) => void
}

const safeTheme = (): Theme => {
  try { return localStorage.getItem('aicw-theme') === 'light' ? 'light' : 'dark' } catch { return 'dark' }
}

export const useStore = create<State>((set, get) => ({
  open: null,
  convs: [],
  setConvs: (convs) => set({ convs }),
  theme: safeTheme(),
  textScale: safeScale(),
  stepText: (dir) => {
    const i = TEXT_SCALES.indexOf(get().textScale)
    const next = dir === 0 ? 1 : TEXT_SCALES[Math.min(TEXT_SCALES.length - 1, Math.max(0, i + dir))]
    try { localStorage.setItem('aicw-text-scale', String(next)) } catch { /* storage unavailable */ }
    applyScale(next)
    set({ textScale: next })
  },
  error: '',
  notice: '',
  setOpen: (open) => set({ open }),
  setTheme: (theme) => {
    try { localStorage.setItem('aicw-theme', theme) } catch { /* storage unavailable */ }
    document.documentElement.classList.toggle('light', theme === 'light')
    set({ theme })
  },
  setError: (error) => set({ error }),
  setNotice: (notice) => set({ notice }),
}))
