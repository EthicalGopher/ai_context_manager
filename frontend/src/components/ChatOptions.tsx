import { useEffect, useRef, useState } from 'react'
import { FiChevronDown, FiSliders } from 'react-icons/fi'
import { ModelPicker } from './ModelPicker'

interface AgentOpt { id: string; name: string }

/** One compact "Chat options" menu: which agent answers, which model it uses, and whether chat replies may edit files. */
export function ChatOptions({ agent, agents, edits, onAgent, onEdits, onError }: {
  agent: string; agents: AgentOpt[]; edits: boolean
  onAgent: (id: string) => void; onEdits: (on: boolean) => void; onError: (m: string) => void
}) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const down = (e: MouseEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(false) }
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', down); document.addEventListener('keydown', key)
    return () => { document.removeEventListener('mousedown', down); document.removeEventListener('keydown', key) }
  }, [open])
  const label = agent === 'shell' ? 'Plain shell' : agents.find((a) => a.id === agent)?.name ?? agent

  return (
    <div ref={root} className="relative">
      <button type="button" aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen(!open)}
        title="Agent, model and file-edit permission for this chat"
        className="inline-flex items-center gap-2 whitespace-nowrap rounded-lg border border-line bg-raised/80 px-3 py-1.5 text-sm font-semibold hover:bg-line/60">
        <FiSliders aria-hidden /> {label} <FiChevronDown aria-hidden className={`transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && (
        <div role="dialog" aria-label="Chat options" className="glass absolute right-0 top-full z-30 mt-2 w-80 space-y-4 rounded-2xl border border-line p-4 shadow-2xl">
          <div>
            <label htmlFor="co-agent" className="mb-1 block text-xs font-bold uppercase tracking-wide text-mute">Agent</label>
            <select id="co-agent" aria-label="Agent for this chat" value={agent} onChange={(e) => onAgent(e.target.value)} className="w-full rounded-lg border border-line bg-bg px-2.5 py-1.5 text-sm">
              {agents.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}<option value="shell">Plain shell</option>
            </select>
          </div>
          {agent !== 'shell' && (
            <div>
              <span className="mb-1 block text-xs font-bold uppercase tracking-wide text-mute">Model</span>
              <ModelPicker agent={agent} onError={onError} />
              <p className="mt-1 text-xs text-mute">Applies from the next message. Nothing chosen means the agent picks its own.</p>
            </div>)}
          <label className="flex cursor-pointer items-start gap-3">
            <input type="checkbox" aria-label="Allow file edits" checked={edits} onChange={(e) => onEdits(e.target.checked)} className="mt-0.5" />
            <span className="text-sm"><b>Allow file edits</b><span className="mt-0.5 block text-xs text-mute">Off: chat replies can read and answer but not change files. The terminal always uses the agent’s own permission prompts.</span></span>
          </label>
        </div>)}
    </div>
  )
}
