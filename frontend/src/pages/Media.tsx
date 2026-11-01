import { MediaViewer } from '../components/MediaViewer'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { FiFilm, FiImage, FiMusic, FiFileText, FiFile, FiUploadCloud, FiTrash2, FiCopy } from 'react-icons/fi'
import { api, errMsg, fmtTime } from '../lib/api'
import { useAsync, useWorkspace } from '../lib/hooks'
import { Badge, Button, Card, Confirm, Empty, ErrorBox, Modal, PageHeader, Spinner, fmtSize } from '../components/ui'
import { NoWorkspace } from '../components/NoWorkspace'
import { OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'

interface Item { id: number; name: string; kind: string; mime: string; size: number; note: string; inContext: boolean; addedAt: string }
const FILTERS = ['all', 'image', 'video', 'audio', 'pdf', 'file'] as const
const url = (id: number) => `/media/${id}`
const KindIcon = ({ kind }: { kind: string }) => {
  const C = kind === 'image' ? FiImage : kind === 'video' ? FiFilm : kind === 'audio' ? FiMusic : kind === 'pdf' ? FiFileText : FiFile
  return <C aria-hidden />
}

export default function Media() {
  const ws = useWorkspace()
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [filter, setFilter] = useState<(typeof FILTERS)[number]>('all')
  const [view, setView] = useState<Item | null>(null)
  const [del, setDel] = useState<Item | null>(null)
  const [busy, setBusy] = useState(false)
  const list = useAsync<Item[]>(() => (ws ? (api.ListMedia(ws.id) as any) : Promise.resolve([])), [ws?.id])

  const importPaths = useCallback(async (paths: string[]) => {
    if (!ws || paths.length === 0) return
    setBusy(true); setError('')
    try {
      const r: any = await api.ImportMedia(ws.id, paths)
      if (r.added.length) setNotice(`Added ${r.added.length} file(s). Originals were copied, not moved.`)
      if (r.errors.length) setError(r.errors.join('\n'))
      list.reload()
    } catch (e) { setError(errMsg(e)) } finally { setBusy(false) }
  }, [ws?.id])

  // Files dropped on the window (Wails passes absolute paths).
  useEffect(() => {
    try { OnFileDrop((_x, _y, paths) => importPaths(paths ?? []), false) } catch { /* not running inside Wails */ }
    return () => { try { OnFileDropOff() } catch { /* ignore */ } }
  }, [importPaths])

  const items = list.data ?? []
  const shown = useMemo(() => items.filter((i) => filter === 'all' || i.kind === filter), [items, filter])
  const counts = useMemo(() => items.reduce<Record<string, number>>((m, i) => ({ ...m, [i.kind]: (m[i.kind] ?? 0) + 1 }), {}), [items])
  const total = items.reduce((n, i) => n + i.size, 0)

  if (!ws) return <><PageHeader title="Media" /><NoWorkspace /></>
  const pick = async () => { try { importPaths((await api.PickMedia()) ?? []) } catch (e) { setError(errMsg(e)) } }
  const toggleCtx = async (i: Item) => { try { await api.UpdateMedia(i.id, i.note, !i.inContext); list.reload() } catch (e) { setError(errMsg(e)) } }

  return (
    <>
      <PageHeader title="Media" subtitle="Keep screenshots, recordings, audio, PDFs and reference files with this workspace. AICW stores private copies (originals untouched) and can hand their paths to your agent."
        actions={<Button variant="primary" onClick={pick} disabled={busy}><FiUploadCloud aria-hidden /> Add files…</Button>} />
      <ErrorBox error={error} onDismiss={() => setError('')} />
      {notice && <p role="status" className="mb-3 text-sm text-ok">{notice}</p>}

      <div className="drop-zone mb-5 flex items-center justify-center gap-3 rounded-2xl border-2 border-dashed border-line px-6 py-8 text-center text-mute transition-colors">
        <FiUploadCloud className="text-3xl text-accent2" aria-hidden />
        <div><p className="font-medium text-fg">{busy ? 'Importing…' : 'Drop images, videos, audio or PDFs anywhere in this window'}</p>
          <p className="text-xs">Stored locally in the app data folder. Up to 2 GB per file. Secret-looking files (.env, keys) are refused.</p></div>
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2" role="tablist" aria-label="Filter by type">
        {FILTERS.map((f) => (
          <button key={f} role="tab" aria-selected={filter === f} onClick={() => setFilter(f)}
            className={`rounded-full border px-3 py-1 text-xs font-medium capitalize ${filter === f ? 'grad-bg border-transparent text-white' : 'border-line bg-raised/60 text-mute hover:text-fg'}`}>
            {f}{f !== 'all' && counts[f] ? ` · ${counts[f]}` : f === 'all' ? ` · ${items.length}` : ''}
          </button>))}
        <span className="ml-auto text-xs text-mute">{fmtSize(total)} stored</span>
      </div>

      {list.loading && <Spinner />}
      {!list.loading && items.length === 0 && <Empty title="No media yet" hint="Add a screenshot of a bug or a screen recording to give the agent visual context." action={<Button variant="primary" onClick={pick}>Add files…</Button>} />}
      {!list.loading && items.length > 0 && shown.length === 0 && <Empty title={`No ${filter} files`} />}

      <ul className="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
        {shown.map((i) => (
          <li key={i.id} className="glass card-hover overflow-hidden rounded-xl border border-line">
            <button className="block aspect-video w-full overflow-hidden bg-raised" onClick={() => setView(i)} aria-label={`Open ${i.name}`}>
              {i.kind === 'image' ? <img src={url(i.id)} alt={i.name} loading="lazy" className="h-full w-full object-cover" />
                : i.kind === 'video' ? <video src={`${url(i.id)}#t=0.1`} preload="metadata" muted className="h-full w-full object-cover" />
                : <div className="flex h-full items-center justify-center text-4xl text-accent"><KindIcon kind={i.kind} /></div>}
            </button>
            <div className="space-y-1.5 p-3">
              <div className="truncate font-medium" title={i.name}>{i.name}</div>
              <div className="flex items-center gap-1.5 text-xs text-mute"><Badge>{i.kind}</Badge>{fmtSize(i.size)} · {new Date(i.addedAt).toLocaleDateString()}</div>
              {i.note && <p className="line-clamp-2 text-xs text-mute">{i.note}</p>}
              <label className="flex items-center gap-2 pt-1 text-xs">
                <input type="checkbox" checked={i.inContext} onChange={() => toggleCtx(i)} aria-label={`Include ${i.name} in context`} /> Tell the agent about this file
              </label>
            </div>
          </li>))}
      </ul>

      {view && <Viewer item={view} onClose={() => { setView(null); list.reload() }} onDelete={() => { setDel(view); setView(null) }} setError={setError} />}
      {del && <Confirm title="Delete from AICW?" danger confirmLabel="Delete" onCancel={() => setDel(null)}
        onConfirm={async () => { try { await api.DeleteMedia(del.id, true); setDel(null); list.reload() } catch (e) { setError(errMsg(e)); setDel(null) } }}>
        <p className="text-sm">Removes AICW's stored copy of <strong>{del.name}</strong>. The original file you imported is not touched.</p>
      </Confirm>}
    </>
  )
}

function Viewer({ item, onClose, onDelete, setError }: { item: Item; onClose: () => void; onDelete: () => void; setError: (s: string) => void }) {
  const [note, setNote] = useState(item.note)
  const [inCtx, setInCtx] = useState(item.inContext)
  const [saved, setSaved] = useState(false)
  const save = async () => { try { await api.UpdateMedia(item.id, note, inCtx); setSaved(true) } catch (e) { setError(errMsg(e)) } }
  return (
    <Modal wide title={item.name} onClose={onClose} footer={<>
      <Button variant="danger" onClick={onDelete}><FiTrash2 aria-hidden /> Delete</Button>
      <Button onClick={async () => { try { await api.CopyToClipboard(await api.MediaPath(item.id)) } catch (e) { setError(errMsg(e)) } }}><FiCopy aria-hidden /> Copy file path</Button>
      <Button variant="primary" onClick={save}>Save</Button>
    </>}>
      <div className="grid gap-4 md:grid-cols-[1fr_16rem]">
        <div className="flex min-h-[16rem] items-center justify-center overflow-hidden rounded-lg border border-line bg-bg">
          {(item.kind === 'image' || item.kind === 'video') && <div className="w-full"><MediaViewer kind={item.kind} src={url(item.id)} caption="" /></div>}
          {item.kind === 'audio' && <audio src={url(item.id)} controls className="w-full p-6" />}
          {(item.kind === 'pdf' || item.kind === 'file') && <div className="p-8 text-center text-mute"><div className="mb-2 text-5xl text-accent"><KindIcon kind={item.kind} /></div><p>No inline preview for this type.</p><p className="text-xs">Use “Copy file path” to open it with another app or give it to an agent.</p></div>}
        </div>
        <div className="space-y-3 text-sm">
          <dl className="grid grid-cols-[4.5rem_1fr] gap-y-1"><dt className="text-mute">Type</dt><dd>{item.mime}</dd><dt className="text-mute">Size</dt><dd>{fmtSize(item.size)}</dd><dt className="text-mute">Added</dt><dd>{fmtTime(item.addedAt)}</dd></dl>
          <label className="block"><span className="text-mute">Note for the agent</span>
            <textarea value={note} onChange={(e) => { setNote(e.target.value); setSaved(false) }} rows={4} maxLength={2000} className="mt-1 w-full rounded-lg border border-line bg-bg px-3 py-2" placeholder="What is this? What should the agent look at?" /></label>
          <label className="flex items-center gap-2"><input type="checkbox" checked={inCtx} onChange={(e) => { setInCtx(e.target.checked); setSaved(false) }} /> Include path and note in the context package</label>
          {saved && <p role="status" className="text-ok">Saved.</p>}
        </div>
      </div>
    </Modal>
  )
}
