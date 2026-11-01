import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { FiEdit2, FiClock, FiTrash2, FiPlus, FiEye, FiImage, FiDownload } from 'react-icons/fi'
import { api, errMsg, fmtTime } from '../lib/api'
import { useWorkspace } from '../lib/hooks'
import { MediaCatalog, RichMarkdown, type MediaInfo } from '../components/RichMarkdown'
import { ToolPanel } from '../components/ToolPanel'
import { Badge, Button, Confirm, DiffView, ErrorBox, Modal } from '../components/ui'
import { Composer } from '../components/Composer'
import { TextSize } from '../components/TextSize'
import { buildPageExport } from '../lib/exportPage'

export interface PageT { id: number; workspaceId: number; title: string; summary: string; body: string; sort: number; updatedAt: string; updatedBy: string }

export const usePages = () => {
  const ws = useWorkspace()
  const [pages, setPages] = useState<PageT[]>([])
  const seen = useRef('')
  // Only replace state when the pages really changed, so the 4 s poll doesn't re-render (and stutter) playing media.
  const reload = useCallback(async () => {
    if (!ws) return
    const next: PageT[] = ((await api.ListPages(ws.id)) as any) ?? []
    const sig = JSON.stringify(next)
    if (sig !== seen.current) { seen.current = sig; setPages(next) }
  }, [ws?.id])
  useEffect(() => { reload(); const t = setInterval(() => { if (!document.hidden) reload() }, 4000); return () => clearInterval(t) }, [reload])
  return { pages, reload }
}

export default function PagesView() {
  const ws = useWorkspace()
  const { id } = useParams()
  const navigate = useNavigate()
  const { pages, reload } = usePages()
  const [media, setMedia] = useState<MediaInfo[]>([])
  const [error, setError] = useState('')
  const [edit, setEdit] = useState(false)
  const [draft, setDraft] = useState({ title: '', summary: '', body: '' })
  const [hist, setHist] = useState<any[] | null>(null)
  const [del, setDel] = useState(false)
  const mediaSeen = useRef('')
  const [notice, setNotice] = useState('')
  const [exporting, setExporting] = useState(false)
  const loadMedia = useCallback(async () => {
    if (!ws) return
    const next: MediaInfo[] = ((await api.ListMedia(ws.id)) as any) ?? []
    const sig = JSON.stringify(next)
    if (sig !== mediaSeen.current) { mediaSeen.current = sig; setMedia(next) }
  }, [ws?.id])
  useEffect(() => { loadMedia(); const t = setInterval(() => { if (!document.hidden) loadMedia() }, 5000); return () => clearInterval(t) }, [loadMedia])

  const page = pages.find((p) => String(p.id) === id) ?? (id ? undefined : pages[0])
  const catalog = useMemo(() => new Map(media.map((m) => [m.id, m])), [media])
  useEffect(() => { setEdit(false); if (page) setDraft({ title: page.title, summary: page.summary, body: page.body }) }, [page?.id, page?.updatedAt])

  if (!ws) return null
  const idx = page ? pages.findIndex((p) => p.id === page.id) : -1

  const create = async () => {
    const title = window.prompt?.('Page title') ?? ''
    if (!title.trim()) return
    try { const p: any = await api.SavePage(ws.id, title.trim(), '', ''); await reload(); navigate(`/p/${p.id}`); setEdit(true) } catch (e) { setError(errMsg(e)) }
  }
  const save = async () => {
    try {
      if (page && draft.title.trim() !== page.title) await api.RenamePage(page.id, draft.title.trim())
      await api.SavePage(ws.id, draft.title.trim(), draft.summary, draft.body)
      await reload(); setEdit(false)
    } catch (e) { setError(errMsg(e)) }
  }
  const exportHtml = async () => {
    if (!page) return
    setExporting(true); setNotice('')
    try {
      const dest = await api.ExportPageHTML(page.id, buildPageExport(page, catalog) as any)
      if (dest) setNotice(`Saved to ${dest}. It is one file with the media inside, so you can send it to anyone and it opens in any browser.`)
    } catch (e) { setError(errMsg(e)) } finally { setExporting(false) }
  }
  const insert = (m: MediaInfo) => setDraft((d) => ({ ...d, body: d.body.replace(/\s*$/, '') + `\n\n![${m.name}](media:${m.id})\n` }))

  return (
    <div className="flex h-full min-w-0">
      <section className="min-w-0 flex-1 overflow-auto" aria-label="Page">
        {error && <div className="p-4"><ErrorBox error={error} onDismiss={() => setError('')} /></div>}
        {notice && <p role="status" className="mx-auto mt-4 max-w-4xl px-8 text-sm text-ok">{notice}</p>}
        {!page ? (
          <div className="mx-auto flex h-full max-w-xl flex-col items-center justify-center gap-4 px-6 text-center">
            <h1 className="text-3xl font-bold">No pages yet</h1>
            <p className="text-mute">Pages are the project's saved documents: text, images, video and audio. Write one yourself, use <i>Save to page</i> on a chat message, or ask your AI to write pages for you (it has a <code>write_page</code> tool).</p>
            <Button variant="primary" onClick={create}><FiPlus aria-hidden /> New page</Button>
          </div>
        ) : (
          <article className="mx-auto max-w-4xl px-8 pb-24 pt-6">
            <header className="mb-8 flex items-center justify-between gap-3 border-b border-line pb-3 text-xs text-mute">
              <span className="uppercase tracking-widest">Project / {ws.name}</span>
              <span className="flex items-center gap-2"><TextSize /><Badge tone={page.updatedBy === 'agent' ? 'accent' : 'neutral'}>{page.updatedBy === 'agent' ? 'written by agent' : 'written by you'}</Badge><span>Updated {fmtTime(page.updatedAt)}</span></span>
            </header>
            {!edit ? (<>
              <p className="mb-2 text-[0.6875rem] font-bold uppercase tracking-[0.2em] text-mute">Page / {String(idx + 1).padStart(2, '0')}</p>
              <h1 className="text-5xl font-extrabold leading-tight">{page.title}</h1>
              {page.summary && <p className="mt-3 max-w-2xl text-lg text-mute">{page.summary}</p>}
              <div className="mt-6 flex flex-wrap gap-2">
                <Button onClick={() => setEdit(true)}><FiEdit2 aria-hidden /> Edit</Button>
                <Button onClick={async () => setHist(((await api.PageVersions(page.id)) as any) ?? [])}><FiClock aria-hidden /> History</Button>
                <Button onClick={exportHtml} disabled={exporting} title="Save this page as one HTML file you can send to friends"><FiDownload aria-hidden /> {exporting ? 'Preparing…' : 'Download HTML'}</Button>
                <Button variant="ghost" onClick={() => setDel(true)}><FiTrash2 aria-hidden /> Delete</Button>
              </div>
              <div className="mt-6">
                <MediaCatalog.Provider value={catalog}>
                  {page.body.trim() ? <RichMarkdown text={page.body} /> : <p className="text-mute">This page is empty. Click Edit to add text and media.</p>}
                </MediaCatalog.Provider>
              </div>
            </>) : (
              <div className="space-y-3">
                <input aria-label="Page title" value={draft.title} onChange={(e) => setDraft({ ...draft, title: e.target.value })} className="w-full rounded-xl border border-line bg-bg px-4 py-3 text-3xl font-extrabold" />
                <input aria-label="Page summary" placeholder="One-sentence summary shown under the title" value={draft.summary} onChange={(e) => setDraft({ ...draft, summary: e.target.value })} className="w-full rounded-xl border border-line bg-bg px-4 py-2 text-mute" />
                <div className="flex items-center justify-between text-xs text-mute"><span><FiImage aria-hidden className="mr-1 inline" />Use “Insert into page” in the Media tab, or write <code>![caption](media:ID)</code></span><span>Markdown</span></div>
                <textarea aria-label="Page body" value={draft.body} onChange={(e) => setDraft({ ...draft, body: e.target.value })} rows={22} className="mono w-full rounded-xl border border-line bg-bg p-4 text-xs leading-6" />
                <div className="flex gap-2"><Button variant="primary" onClick={save}>Save page</Button><Button onClick={() => { setEdit(false); if (page) setDraft({ title: page.title, summary: page.summary, body: page.body }) }}><FiEye aria-hidden /> Cancel</Button></div>
              </div>
            )}
          </article>
        )}
        {page && !edit && (
          <div className="sticky bottom-0 bg-gradient-to-t from-bg via-bg to-transparent pt-6">
            <Composer wsId={ws.id} placeholder={`Add text, images, video or audio to “${page.title}”…`} hint="Enter to add to this page · Shift+Enter for a new line · attach, paste or drop images, video, audio · record a voice message"
              onSend={async (text, files) => { await api.AddToPage(page.id, text, files.map((f) => f.id)); await reload(); await loadMedia() }} />
          </div>)}
      </section>
      <ToolPanel wsId={ws.id} media={media} onInsert={edit ? insert : undefined} onMediaChanged={loadMedia} />
      {hist && page && (
        <Modal wide title={`History: ${page.title}`} onClose={() => setHist(null)}>
          {hist.length === 0 ? <p className="text-sm text-mute">No earlier versions yet. A version is kept each time the page is replaced.</p> : (
            <ul className="space-y-4">{hist.map((v) => (
              <li key={v.id}><div className="mb-1 flex items-center justify-between text-xs text-mute"><span>{fmtTime(v.savedAt)} · {v.savedBy}</span>
                <button className="font-bold text-accent" onClick={() => { setDraft({ title: v.title, summary: v.summary, body: v.body }); setEdit(true); setHist(null) }}>Restore into editor</button></div>
                <DiffView diff={`--- version\n+++ current\n${v.body.split('\n').map((l: string) => '-' + l).join('\n')}\n${page.body.split('\n').map((l: string) => '+' + l).join('\n')}`} /></li>))}</ul>)}
        </Modal>)}
      {del && page && <Confirm title="Delete this page?" danger confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={async () => { try { await api.DeletePage(page.id, true); setDel(false); await reload(); navigate('/p') } catch (e) { setError(errMsg(e)); setDel(false) } }}><p className="text-sm">Deletes “{page.title}” and its history. Media files in the library are kept.</p></Confirm>}
    </div>
  )
}
