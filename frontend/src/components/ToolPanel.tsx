import { useEffect, useState } from 'react'
import { FiFolder, FiImage, FiTool, FiHelpCircle, FiChevronRight, FiChevronDown, FiFile, FiRefreshCw, FiSearch, FiPlus } from 'react-icons/fi'
import { api, errMsg } from '../lib/api'
import { Badge, Modal, Spinner, fmtSize } from './ui'
import type { MediaInfo } from './RichMarkdown'

type Tab = 'files' | 'media' | 'tools' | 'help'
interface Entry { name: string; rel: string; dir: boolean; size: number; secret: boolean }

/** Right-hand panel (Files / Media / Tools / Help), like a project workbench. */
export function ToolPanel({ wsId, media, onInsert, onMediaChanged }: { wsId: number; media: MediaInfo[]; onInsert?: (m: MediaInfo) => void; onMediaChanged: () => void }) {
  const [tab, setTab] = useState<Tab>('files')
  const tabs: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: 'files', label: 'Files', icon: <FiFolder aria-hidden /> }, { id: 'media', label: 'Media', icon: <FiImage aria-hidden /> },
    { id: 'tools', label: 'Tools', icon: <FiTool aria-hidden /> }, { id: 'help', label: 'Help', icon: <FiHelpCircle aria-hidden /> }]
  return (
    <aside aria-label="Project tools" className="flex h-full w-[22rem] shrink-0 border-l border-line bg-panel">
      <div className="flex min-w-0 flex-1 flex-col">
        <h2 className="border-b border-line px-4 py-3 text-base">{tabs.find((t) => t.id === tab)?.label}</h2>
        <div className="min-h-0 flex-1 overflow-auto">
          {tab === 'files' && <Files wsId={wsId} onImported={onMediaChanged} />}
          {tab === 'media' && <MediaList wsId={wsId} media={media} onInsert={onInsert} onChanged={onMediaChanged} />}
          {tab === 'tools' && <Tools />}
          {tab === 'help' && <Help />}
        </div>
      </div>
      <nav aria-label="Panel sections" className="flex w-16 shrink-0 flex-col items-center gap-1 border-l border-line py-3">
        {tabs.map((t) => (
          <button key={t.id} aria-label={t.label} aria-pressed={tab === t.id} onClick={() => setTab(t.id)} className={`flex w-14 flex-col items-center gap-1 rounded-xl py-2 text-lg ${tab === t.id ? 'bg-raised text-fg' : 'text-mute hover:bg-raised/60 hover:text-fg'}`}>{t.icon}<span className="text-[0.625rem] font-semibold">{t.label}</span></button>))}
      </nav>
    </aside>
  )
}

function Files({ wsId, onImported }: { wsId: number; onImported: () => void }) {
  const [showAll, setShowAll] = useState(false)
  const [q, setQ] = useState('')
  const [n, setN] = useState(0)
  return (
    <div className="p-3">
      <div className="mb-3 flex gap-2">
        <label className="flex min-w-0 flex-1 items-center gap-2 rounded-lg border border-line bg-bg px-3 py-1.5 text-mute"><FiSearch aria-hidden />
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filter file names…" aria-label="Filter file names" className="min-w-0 flex-1 border-0 bg-transparent p-0 text-fg shadow-none" /></label>
        <button aria-label="Refresh files" className="rounded-lg border border-line p-2 text-mute hover:text-fg" onClick={() => setN(n + 1)}><FiRefreshCw aria-hidden /></button>
      </div>
      <Dir key={`${n}-${showAll}`} wsId={wsId} rel="" showAll={showAll} q={q.toLowerCase()} depth={0} onImported={onImported} />
      <label className="mt-3 flex items-center gap-2 text-xs text-mute"><input type="checkbox" checked={showAll} onChange={(e) => setShowAll(e.target.checked)} /> Show hidden and dependency folders</label>
      <p className="mt-2 text-xs text-mute">Read-only view of your project. Secret-looking files are marked and never previewed.</p>
    </div>
  )
}

function Dir({ wsId, rel, showAll, q, depth, onImported }: { wsId: number; rel: string; showAll: boolean; q: string; depth: number; onImported: () => void }) {
  const [items, setItems] = useState<Entry[] | null>(null)
  const [err, setErr] = useState('')
  const [open, setOpen] = useState<Record<string, boolean>>({})
  const [view, setView] = useState<Entry | null>(null)
  useEffect(() => { api.ListDir(wsId, rel, showAll).then((l: any) => setItems(l ?? [])).catch((e) => setErr(errMsg(e))) }, [wsId, rel, showAll])
  if (err) return <p className="px-2 text-xs text-bad">{err}</p>
  if (!items) return <Spinner label="" />
  return (
    <ul style={{ paddingLeft: depth ? 14 : 0 }}>
      {items.filter((e) => e.dir || !q || e.name.toLowerCase().includes(q)).map((e) => (
        <li key={e.rel}>
          {e.dir ? (
            <>
              <button className="flex w-full items-center gap-1.5 rounded-lg px-2 py-1 text-left hover:bg-raised" aria-expanded={!!open[e.rel]} onClick={() => setOpen({ ...open, [e.rel]: !open[e.rel] })}>
                {open[e.rel] ? <FiChevronDown aria-hidden /> : <FiChevronRight aria-hidden />}<FiFolder aria-hidden className="text-mute" />{e.name}</button>
              {open[e.rel] && <Dir wsId={wsId} rel={e.rel} showAll={showAll} q={q} depth={depth + 1} onImported={onImported} />}
            </>
          ) : (
            <button className="flex w-full items-center gap-2 rounded-lg px-2 py-1 text-left hover:bg-raised" onClick={() => setView(e)} title={e.rel}>
              <FiFile aria-hidden className="shrink-0 text-mute" /><span className="min-w-0 flex-1 truncate">{e.name}</span>
              {e.secret ? <Badge tone="warn">secret</Badge> : <span className="text-xs text-mute">{fmtSize(e.size)}</span>}</button>
          )}
        </li>))}
      {view && <FilePreview wsId={wsId} entry={view} onClose={() => setView(null)} onImported={onImported} />}
    </ul>
  )
}

function FilePreview({ wsId, entry, onClose, onImported }: { wsId: number; entry: Entry; onClose: () => void; onImported: () => void }) {
  const [text, setText] = useState('')
  const [msg, setMsg] = useState('')
  const isMedia = /\.(png|jpe?g|gif|webp|svg|bmp|avif|mp4|webm|mov|mkv|mp3|wav|ogg|m4a|flac|pdf)$/i.test(entry.name)
  useEffect(() => { if (!isMedia && !entry.secret) api.ReadFile(wsId, entry.rel).then(setText).catch((e) => setMsg(errMsg(e))) }, [entry.rel])
  return (
    <Modal wide title={entry.rel} onClose={onClose} footer={isMedia && !entry.secret ? <button className="rounded-lg bg-accent px-3 py-1.5 text-sm font-bold text-white" onClick={async () => { try { await api.ImportProjectFile(wsId, entry.rel); setMsg('Added to the media library. Embed it in a page from the Media tab.'); onImported() } catch (e) { setMsg(errMsg(e)) } }}><FiPlus aria-hidden className="mr-1 inline" />Add to media library</button> : undefined}>
      {entry.secret ? <p className="text-sm text-warn">This looks like a secrets file, so AICW will not display it.</p> : isMedia ? <p className="text-sm text-mute">Media file. Add it to the library to preview it and embed it in pages.</p> : <pre className="max-h-[60vh] overflow-auto whitespace-pre-wrap text-xs">{text || msg || 'Loading…'}</pre>}
      {msg && <p role="status" className="mt-3 text-sm text-mute">{msg}</p>}
    </Modal>
  )
}

function MediaList({ wsId, media, onInsert, onChanged }: { wsId: number; media: MediaInfo[]; onInsert?: (m: MediaInfo) => void; onChanged: () => void }) {
  const [msg, setMsg] = useState('')
  const add = async () => { try { const r: any = await api.ImportMedia(wsId, ((await api.PickMedia()) as any) ?? []); if (r.errors?.length) setMsg(r.errors.join('\n')); onChanged() } catch (e) { setMsg(errMsg(e)) } }
  return (
    <div className="p-3">
      <button className="mb-3 w-full rounded-lg border border-line bg-raised py-2 text-sm font-bold hover:bg-line/60" onClick={add}><FiPlus aria-hidden className="mr-1 inline" />Add files…</button>
      {msg && <p role="alert" className="mb-2 whitespace-pre-wrap text-xs text-bad">{msg}</p>}
      {media.length === 0 && <p className="text-sm text-mute">No media yet. Add images, video, audio or PDFs, or let the AI add them.</p>}
      <ul className="grid grid-cols-2 gap-2">
        {media.map((m) => (
          <li key={m.id} className="overflow-hidden rounded-xl border border-line bg-bg">
            <div className="aspect-video bg-raised">{m.kind === 'image' ? <img src={`/media/${m.id}`} alt={m.name} loading="lazy" className="h-full w-full object-cover" /> : m.kind === 'video' ? <video src={`/media/${m.id}#t=0.1`} preload="metadata" muted className="h-full w-full object-cover" /> : <div className="flex h-full items-center justify-center text-2xl text-mute"><FiFile aria-hidden /></div>}</div>
            <div className="p-2"><div className="truncate text-xs">{m.name}</div>
              {onInsert ? <button className="mt-1 text-xs font-bold text-accent" onClick={() => onInsert(m)}>Insert into page</button> : <span className="text-[0.6875rem] text-mute">id {m.id}</span>}</div>
          </li>))}
      </ul>
    </div>
  )
}

const TOOLS: [string, string][] = [
  ['read_context', 'load notes and always-on skills'], ['write_note / append_note', 'keep project memory'], ['add_skill', 'propose a rule for future sessions'],
  ['write_page / append_page', 'save designs, plans, research with images/video/audio'], ['create_conversation / add_message', 'start chats and post updates'], ['add_media / list_media', 'store files in the library'],
]
function Tools() {
  return (
    <div className="space-y-4 p-4 text-sm">
      <p className="text-mute">What your AI can do inside this project, as an MCP server (<code>aicw mcp</code>) or from a shell with <code>aicw tool</code>:</p>
      <ul className="space-y-2">{TOOLS.map(([n, d]) => <li key={n}><code className="text-fg">{n}</code><div className="text-xs text-mute">{d}</div></li>)}</ul>
      <p className="rounded-xl border border-line bg-bg p-3 text-xs text-mute">Claude Code gets these automatically when started from AICW. For other CLIs, install your always-on skills into <code>AGENTS.md</code> (Skills &amp; memory) so they learn the <code>aicw tool</code> commands.</p>
    </div>
  )
}

function Help() {
  return (
    <div className="space-y-3 p-4 text-sm text-mute">
      <p><b className="text-fg">Chats</b> talk to your AI CLI. Replies appear as messages; use <i>Save to page</i> on any message to keep it.</p>
      <p><b className="text-fg">Pages</b> are the project's saved documents. You and the AI can write them, with images, video and audio.</p>
      <p><b className="text-fg">Terminal</b> (in a chat) runs the CLI itself for full control.</p>
      <p><b className="text-fg">Skills &amp; memory</b> are rules and notes the AI must always read and keeps updating.</p>
      <p><b className="text-fg">Voice</b>: the mic button dictates text locally. Set it up in Settings.</p>
    </div>
  )
}
