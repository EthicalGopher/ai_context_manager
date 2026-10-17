import { useEffect, useState } from 'react'
import { api, errMsg } from '../lib/api'

/**
 * Local models downloaded by AICW, kept apart from the agent's own model list. They run through Claude Code,
 * so choosing one switches the chat to that agent; "None" hands the choice back to the agent's model list.
 */
export function LocalPicker({ agent, onAgent, onError }: { agent: string; onAgent: (id: string) => void; onError: (m: string) => void }) {
  const [files, setFiles] = useState<{ file: string; name: string }[] | null>(null)
  const [saved, setSaved] = useState('')
  useEffect(() => {
    let live = true
    Promise.all([api.LocalModels() as Promise<any>, api.GetSettings() as Promise<any>]).then(([st, s]) => {
      if (!live) return
      setFiles(st?.models ?? []); setSaved(s.agentModels?.claude ?? '')
    }).catch(() => { if (live) setFiles([]) })
    return () => { live = false }
  }, [])
  if (files === null) return null
  const active = agent === 'claude' && saved.startsWith('aicw:') ? saved : ''
  const choose = async (v: string) => {
    try {
      const s: any = await api.GetSettings()
      await api.SaveSettings({ ...s, agentModels: { ...(s.agentModels ?? {}), claude: v } })
      setSaved(v)
      if (v && agent !== 'claude') onAgent('claude')
    } catch (e) { onError(errMsg(e)) }
  }
  return (
    <div>
      <span className="mb-1 block text-xs font-bold uppercase tracking-wide text-mute">Local</span>
      {files.length === 0
        ? <p className="text-xs text-mute">No local model yet. Settings → Local models downloads one (no Ollama needed).</p>
        : <>
            <select aria-label="Local model" value={active} onChange={(e) => choose(e.target.value)} className="w-full rounded-lg border border-line bg-bg px-2.5 py-1.5 text-sm">
              <option value="">None (use the agent’s model)</option>
              {files.map((f) => <option key={f.file} value={'aicw:' + f.file}>{f.name}</option>)}
            </select>
            <p className="mt-1 text-xs text-mute">Runs on this computer through Claude Code.</p>
          </>}
    </div>
  )
}
