import { useEffect, useState } from 'react'
import { api, errMsg } from '../lib/api'

/** Per-agent model override (passed to the CLI as its --model option). Empty = the CLI's own default. */
export function ModelPicker({ agent, className = '', onError }: { agent: string; className?: string; onError?: (m: string) => void }) {
  const [value, setValue] = useState('')
  const [saved, setSaved] = useState('')
  const [hints, setHints] = useState<string[]>([])
  const [supported, setSupported] = useState(false)

  useEffect(() => {
    let live = true
    setSupported(false)
    if (!agent || agent === 'shell') return
    Promise.all([api.GetSettings() as Promise<any>, api.ListAgents() as Promise<any[]>]).then(([s, list]) => {
      const p = list.find((a) => a.profile.id === agent)?.profile
      if (!live || !p?.modelArgs?.length) return
      const m = s.agentModels?.[agent] ?? ''
      setSupported(true); setValue(m); setSaved(m); setHints(p.modelHints ?? [])
    }).catch(() => {})
    return () => { live = false }
  }, [agent])

  if (!supported) return null
  const commit = async () => {
    const v = value.trim()
    if (v === saved) return
    try {
      const s: any = await api.GetSettings()
      await api.SaveSettings({ ...s, agentModels: { ...(s.agentModels ?? {}), [agent]: v } })
      setSaved(v)
    } catch (e) { setValue(saved); onError?.(errMsg(e)) }
  }
  const id = `models-${agent}`
  return (
    <>
      <input aria-label="Model for this agent" list={id} value={value} placeholder="default model" spellCheck={false}
        title="Model passed to this agent's CLI (--model). Leave empty for its own default. Takes effect on the next message or terminal start."
        onChange={(e) => setValue(e.target.value)} onBlur={commit} onKeyDown={(e) => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur() }}
        className={`w-40 rounded-lg border border-line bg-bg px-2 py-1 text-xs ${className}`} />
      <datalist id={id}>{hints.map((h) => <option key={h} value={h} />)}</datalist>
    </>
  )
}
