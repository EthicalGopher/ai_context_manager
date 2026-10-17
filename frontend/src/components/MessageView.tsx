import { FiFile, FiFileText, FiBookmark } from 'react-icons/fi'
import { Markdown } from './Markdown'
import { fmtSize } from './ui'

export interface Msg {
  id: number; role: 'user' | 'assistant' | 'system'; source: string; body: string; createdAt: string
  media: { id: number; name: string; kind: string; size: number }[]
}

const mediaUrl = (id: number) => `/media/${id}`

export function Attachment({ m }: { m: Msg['media'][number] }) {
  if (m.kind === 'image') return <a href="#" onClick={(e) => e.preventDefault()}><img src={mediaUrl(m.id)} alt={m.name} className="max-h-72 max-w-full rounded-xl border border-line object-contain" loading="lazy" /></a>
  if (m.kind === 'video') return <video src={mediaUrl(m.id)} controls preload="metadata" className="max-h-72 max-w-full rounded-xl border border-line" />
  if (m.kind === 'audio') return <audio src={mediaUrl(m.id)} controls className="w-72 max-w-full" />
  return <span className="inline-flex items-center gap-2 rounded-xl border border-line bg-raised px-3 py-2 text-sm">{m.kind === 'pdf' ? <FiFileText aria-hidden /> : <FiFile aria-hidden />}{m.name}<span className="text-xs text-mute">{fmtSize(m.size)}</span></span>
}

export function MessageView({ m, onSave }: { m: Msg; onSave?: (m: Msg) => void }) {
  const mine = m.role === 'user'
  if (m.role === 'system') return <p className="mx-auto max-w-[85%] whitespace-pre-wrap rounded-xl border border-line bg-panel px-3 py-2 text-center text-xs text-mute" role="note">{m.body}</p>
  return (
    <article className={`group flex ${mine ? 'justify-end' : 'justify-start'}`} aria-label={mine ? 'You' : m.source === 'agent' ? 'Agent' : 'Assistant'}>
      <div className={`max-w-[85%] space-y-2 ${mine ? 'rounded-3xl bg-raised px-4 py-2.5' : 'px-1'}`}>
        {!mine && <div className="text-xs font-bold text-mute">{m.source === 'agent' ? 'Agent (via AICW tools)' : m.source === 'terminal' ? 'Agent (terminal)' : 'Assistant'}</div>}
        {m.media.length > 0 && <div className="flex flex-wrap gap-2">{m.media.map((x) => <Attachment key={x.id} m={x} />)}</div>}
        {m.body.trim() && (mine ? <p className="whitespace-pre-wrap break-words">{m.body}</p> : <Markdown text={m.body} />)}
        {onSave && <button onClick={() => onSave(m)} className="flex items-center gap-1 text-xs font-bold text-mute opacity-0 transition-opacity hover:text-fg focus:opacity-100 group-hover:opacity-100"><FiBookmark aria-hidden /> Save to page</button>}
      </div>
    </article>
  )
}
