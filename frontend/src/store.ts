import { create } from 'zustand'
import type { OpenResult } from './lib/api'

export interface Conv { id: number; workspaceId: number; title: string; agent: string; sessionId: number; messages: number; updatedAt: string }

export type Theme = 'dark' | 'light'

interface State {
  open: OpenResult | null
  convs: Conv[]
  setConvs: (c: Conv[]) => void
  theme: Theme
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

export const useStore = create<State>((set) => ({
  open: null,
  convs: [],
  setConvs: (convs) => set({ convs }),
  theme: safeTheme(),
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
