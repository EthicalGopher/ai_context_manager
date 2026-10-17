import { useCallback, useEffect, useState } from 'react'
import { api, errMsg, onEvent } from '../lib/api'
import { Button, Card, Confirm } from './ui'

interface Entry { id: string; name: string; file: string; size: number; note: string }
interface Local { file: string; name: string; size: number }
interface Status { engineInstalled: boolean; engineVersion: string; models: Local[]; catalog: Entry[]; running: string; dir: string }

const gb = (n: number) => (n >= 1e9 ? (n / 1e9).toFixed(1) + ' GB' : Math.round(n / 1e6) + ' MB')

/** Download and run a model on this computer: llama.cpp's server plus a GGUF file, kept in AICW's own folder. */
export function LocalModels() {
  const [st, setSt] = useState<Status | null>(null)
  const [error, setError] = useState('')
  const [progress, setProgress] = useState<{ message: string; done: number; total: number } | null>(null)
  const [confirm, setConfirm] = useState<{ kind: 'engine' } | { kind: 'model'; e: Entry } | { kind: 'delete'; l: Local } | null>(null)

  const load = useCallback(() => { Promise.resolve(api.LocalModels()).then((s: any) => setSt(s)).catch((e) => setError(errMsg(e))) }, [])
  useEffect(() => {
    load()
    const a = onEvent('llm:progress', (p: any) => setProgress(p))
    const b = onEvent('llm:done', (m: string) => { setProgress(null); setError(m); load() })
    return () => { a(); b() }
  }, [load])

  const start = async (fn: () => Promise<unknown>) => { setError(''); try { await fn(); setProgress({ message: 'Starting…', done: 0, total: 0 }) } catch (e) { setError(errMsg(e)) } }
  const busy = !!progress

  return (
    <Card title="Local models" className="mt-4">
      <p className="mb-3 text-sm text-mute">Run a model on this computer. AICW downloads llama.cpp’s own server and a model into its folder, starts it only when you use it, and needs no Ollama or other tool. Pick it afterwards in a chat under <b className="text-fg">Chat options → Model → Local</b>; it runs through Claude Code. Small models are much weaker than hosted ones.</p>
      {error && <p role="alert" className="mb-2 whitespace-pre-wrap text-sm text-bad">{error}</p>}
      <div className="mb-3 flex flex-wrap items-center gap-3 text-sm">
        <span className="w-24 text-mute">Engine</span>
        {st?.engineInstalled ? <span className="text-ok">Installed <span className="text-mute">({st.engineVersion})</span></span> : <span className="text-warn">Not installed</span>}
        {!st?.engineInstalled && <Button disabled={busy} onClick={() => setConfirm({ kind: 'engine' })}>Download engine</Button>}
        {st?.running && <><span className="text-mute">Running: {st.running}</span><Button variant="ghost" onClick={async () => { await api.LocalServerStop(); load() }}>Stop</Button></>}
      </div>
      <ul className="divide-y divide-line rounded-xl border border-line" aria-label="Models">
        {(st?.catalog ?? []).map((e) => {
          const have = st?.models.find((m) => m.file === e.file)
          return (
            <li key={e.id} className="flex items-center justify-between gap-3 px-3 py-2.5">
              <div className="min-w-0">
                <div className="text-sm font-bold">{e.name} <span className="font-normal text-mute">· {gb(e.size)}</span></div>
                <div className="text-xs text-mute">{e.note}</div>
              </div>
              {have ? <div className="flex shrink-0 items-center gap-2"><span className="text-xs text-ok">Downloaded</span><Button variant="ghost" disabled={busy} onClick={() => setConfirm({ kind: 'delete', l: have })}>Delete</Button></div>
                : <Button disabled={busy} onClick={() => setConfirm({ kind: 'model', e })}>Download</Button>}
            </li>)
        })}
      </ul>
      {progress && (
        <div className="mt-3" role="status">
          <div className="mb-1 text-xs text-mute">{progress.message}{progress.total > 0 && ` ${gb(progress.done)} of ${gb(progress.total)}`}</div>
          <div className="h-2 overflow-hidden rounded bg-line"><div className="h-full bg-accent transition-all" style={{ width: progress.total > 0 ? `${Math.min(100, (progress.done / progress.total) * 100)}%` : '100%', opacity: progress.total > 0 ? 1 : 0.4 }} /></div>
        </div>)}
      {st && <p className="mt-3 text-xs text-mute">Files are kept in <code>{st.dir}</code>. You can also drop your own <code>.gguf</code> files into its <code>models</code> folder.</p>}
      {confirm?.kind === 'engine' && <Confirm title="Download the local engine?" confirmLabel="Download" onCancel={() => setConfirm(null)} onConfirm={() => { setConfirm(null); start(() => api.LocalEngineInstall(true)) }}>
        <p className="text-sm">Downloads the latest <b>llama.cpp</b> server build for this computer (about 20–30 MB) from github.com/ggml-org/llama.cpp, checks its checksum and unpacks it inside AICW’s folder. Nothing else on your system is changed.</p></Confirm>}
      {confirm?.kind === 'model' && <Confirm title={`Download ${confirm.e.name}?`} confirmLabel="Download" onCancel={() => setConfirm(null)} onConfirm={() => { const e = confirm.e; setConfirm(null); start(() => api.LocalModelDownload(e.id, true)) }}>
        <p className="text-sm">This downloads <b>{gb(confirm.e.size)}</b> from huggingface.co into AICW’s folder. It can be resumed if interrupted. Make sure you have the disk space and enough memory to run it.</p></Confirm>}
      {confirm?.kind === 'delete' && <Confirm title={`Delete ${confirm.l.name}?`} confirmLabel="Delete" danger onCancel={() => setConfirm(null)} onConfirm={async () => { const l = confirm.l; setConfirm(null); try { await api.LocalModelDelete(l.file, true); load() } catch (e) { setError(errMsg(e)) } }}>
        <p className="text-sm">Removes the file ({gb(confirm.l.size)}) from disk. You can download it again later.</p></Confirm>}
    </Card>
  )
}
