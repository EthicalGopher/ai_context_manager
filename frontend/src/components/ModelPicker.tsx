import { useCallback, useEffect, useState } from 'react'
import { FiRefreshCw } from 'react-icons/fi'
import { api, errMsg } from '../lib/api'

interface Model { id: string; label: string; group: string }
const CUSTOM = '__custom__'

/**
 * Model list for one agent: what the CLI itself offers (Kilo, AGY), Claude's aliases and models served
 * on this machine. Nothing is pre-selected; "Choose a model…" leaves the choice to the CLI. A name can also be typed.
 */
export function ModelPicker({ agent, className = '', onError }: { agent: string; className?: string; onError?: (m: string) => void }) {
  const [saved, setSaved] = useState('')
  const [models, setModels] = useState<Model[]>([])
  const [notes, setNotes] = useState<string[]>([])
  const [supported, setSupported] = useState(false)
  const [loading, setLoading] = useState(false)
  const [custom, setCustom] = useState<string | null>(null) // text being typed, or null while the list is used

  const load = useCallback(async (refresh: boolean) => {
    setLoading(true)
    try {
      const r: any = await api.ListModels(agent, refresh)
      setModels(r?.models ?? []); setNotes(r?.notes ?? [])
    } catch { setModels([]); setNotes([]) } finally { setLoading(false) }
  }, [agent])

  useEffect(() => {
    let live = true
    setSupported(false); setCustom(null)
    if (!agent || agent === 'shell') return
    Promise.all([api.GetSettings() as Promise<any>, api.ListAgents() as Promise<any[]>]).then(([s, list]) => {
      const p = list.find((a) => a.profile.id === agent)?.profile
      if (!live || !p?.modelArgs?.length) return
      setSupported(true); setSaved(s.agentModels?.[agent] ?? '')
      load(false)
    }).catch(() => {})
    return () => { live = false }
  }, [agent, load])

  if (!supported) return null

  const save = async (v: string) => {
    v = v.trim()
    if (v === saved) return
    try {
      const s: any = await api.GetSettings()
      await api.SaveSettings({ ...s, agentModels: { ...(s.agentModels ?? {}), [agent]: v } })
      setSaved(v)
    } catch (e) { onError?.(errMsg(e)) }
  }

  const base = `w-full rounded-lg border border-line bg-bg px-2.5 py-1.5 text-sm ${className}`
  const known = models.some((m) => m.id === saved)
  const typing = custom !== null || (!loading && models.length === 0)
  const groups = [...new Set(models.map((m) => m.group))]

  return (
    <div className="space-y-1.5">
      {typing ? (
        <div className="flex gap-2">
          <input aria-label="Model for this agent" value={custom ?? saved} placeholder="Type a model name" spellCheck={false} autoFocus={custom !== null}
            onChange={(e) => setCustom(e.target.value)} onBlur={async () => { if (custom !== null) { await save(custom); setCustom(null) } }}
            onKeyDown={(e) => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur(); if (e.key === 'Escape') setCustom(null) }} className={base} />
          {models.length > 0 && <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => setCustom(null)} className="shrink-0 text-xs text-mute underline">List</button>}
        </div>
      ) : (
        <div className="flex gap-2">
          <select aria-label="Model for this agent" disabled={loading} value={saved} className={base}
            onChange={(e) => { if (e.target.value === CUSTOM) setCustom(saved); else save(e.target.value) }}>
            {loading ? <option value="">Loading models…</option> : <option value="">Choose a model…</option>}
            {saved && !known && !loading && <option value={saved}>{saved}</option>}
            {groups.map((g) => (
              <optgroup key={g} label={g}>
                {models.filter((m) => m.group === g).map((m) => <option key={m.id} value={m.id} disabled={m.id.startsWith('aicw-get:')}>{m.label === m.id ? m.label : `${m.label} (${m.id})`}</option>)}
              </optgroup>))}
            <option value={CUSTOM}>Other… (type a name)</option>
          </select>
          <button type="button" aria-label="Refresh models" title="Refresh the list" disabled={loading} onClick={() => load(true)} className="shrink-0 rounded-lg border border-line px-2 text-mute hover:text-fg disabled:opacity-40"><FiRefreshCw aria-hidden className={loading ? 'animate-spin' : ''} /></button>
        </div>)}
      {notes.map((n) => <p key={n} className="text-xs text-mute">{n}</p>)}
    </div>
  )
}
