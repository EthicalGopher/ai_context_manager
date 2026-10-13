import { useCallback, useEffect, useRef, useState } from 'react'
import { FiPaperclip, FiMic, FiSquare, FiArrowUp, FiX, FiFile, FiVolume2 } from 'react-icons/fi'
import { api, errMsg } from '../lib/api'
import { OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'

export interface Attached { id: number; name: string; kind: string }
type Rec = 'idle' | 'recording' | 'transcribing'

const toB64 = (f: File) => new Promise<string>((res, rej) => { const r = new FileReader(); r.onload = () => res(String(r.result).split(',')[1] ?? ''); r.onerror = () => rej(r.error); r.readAsDataURL(f) })

/** ChatGPT-style composer: text, attachments (picker, drop, paste), and voice dictation. */
export function Composer({ wsId, busy, placeholder, onSend, autoFocus, onVoiceHelp, footerLeft, hint }: {
  wsId: number; busy?: boolean; placeholder?: string; autoFocus?: boolean; footerLeft?: React.ReactNode; hint?: string
  onSend: (text: string, media: Attached[]) => Promise<void> | void
  onVoiceHelp?: () => void
}) {
  const [text, setText] = useState('')
  const [files, setFiles] = useState<Attached[]>([])
  const [error, setError] = useState('')
  const [rec, setRec] = useState<Rec>('idle')
  const [kind, setKind] = useState<'dictate' | 'note'>('dictate')
  const [secs, setSecs] = useState(0)
  const ta = useRef<HTMLTextAreaElement>(null)

  const grow = () => { const el = ta.current; if (el) { el.style.height = 'auto'; el.style.height = Math.min(el.scrollHeight, 220) + 'px' } }
  useEffect(grow, [text])
  useEffect(() => { if (autoFocus) ta.current?.focus() }, [autoFocus])
  useEffect(() => { if (rec !== 'recording') return; setSecs(0); const t = setInterval(() => setSecs((s) => s + 1), 1000); return () => clearInterval(t) }, [rec])

  const addPaths = useCallback(async (paths: string[]) => {
    if (!paths.length) return
    try {
      const r: any = await api.ImportMedia(wsId, paths)
      setFiles((f) => [...f, ...r.added.map((m: any) => ({ id: m.id, name: m.name, kind: m.kind }))])
      if (r.errors.length) setError(r.errors.join('\n'))
    } catch (e) { setError(errMsg(e)) }
  }, [wsId])

  useEffect(() => {
    try { OnFileDrop((_x, _y, paths) => addPaths(paths ?? []), false) } catch { /* not in Wails */ }
    return () => { try { OnFileDropOff() } catch { /* ignore */ } }
  }, [addPaths])

  const submit = async () => {
    if (busy || (!text.trim() && files.length === 0)) return
    const t = text, f = files
    setText(''); setFiles([]); setError('')
    try { await onSend(t, f) } catch (e) { setText(t); setFiles(f); setError(errMsg(e)) }
  }

  const onPaste = async (e: React.ClipboardEvent) => {
    const items = Array.from(e.clipboardData.files)
    if (!items.length) return
    e.preventDefault()
    for (const f of items) {
      try { const m: any = await api.ImportMediaData(wsId, f.name || 'pasted.png', await toB64(f)); setFiles((x) => [...x, { id: m.id, name: m.name, kind: m.kind }]) } catch (err) { setError(errMsg(err)) }
    }
  }

  const mic = async () => {
    setError('')
    if (rec === 'recording') {
      setRec('transcribing')
      try {
        const t = await api.VoiceStop()
        setText((x) => (x ? x.replace(/\s*$/, ' ') : '') + t)
        ta.current?.focus()
      } catch (e) { setError(errMsg(e)) } finally { setRec('idle') }
      return
    }
    try {
      const v: any = await api.VoiceStatus()
      if (!v.ready) { setError(v.hint || 'Voice is not set up yet.'); onVoiceHelp?.(); return }
      await api.VoiceStart(); setKind('dictate'); setRec('recording')
    } catch (e) { setError(errMsg(e)) }
  }

  // Voice message: keep the recording as an audio attachment (with a transcript when possible).
  const note = async () => {
    setError('')
    if (rec === 'recording' && kind === 'note') {
      setRec('transcribing')
      try {
        const r: any = await api.VoiceStopAttach(wsId)
        setFiles((f) => [...f, { id: r.media.id, name: r.media.name, kind: 'audio' }])
        if (r.transcript) setText((x) => (x ? x.replace(/\s*$/, ' ') : '') + r.transcript)
        if (r.warning) setError(r.warning)
        ta.current?.focus()
      } catch (e) { setError(errMsg(e)) } finally { setRec('idle') }
      return
    }
    try {
      const v: any = await api.VoiceStatus()
      if (!v.recorder) { setError(v.hint || 'No audio recorder found.'); return }
      await api.VoiceStart(); setKind('note'); setRec('recording')
    } catch (e) { setError(errMsg(e)) }
  }
  const cancelRec = async () => { try { await api.VoiceCancel() } finally { setRec('idle') } }

  return (
    <div className="mx-auto w-full max-w-3xl px-4 pb-4">
      {error && <div role="alert" className="mb-2 flex justify-between gap-3 rounded-xl border border-bad/40 bg-bad/10 px-3 py-2 text-sm text-bad"><span className="whitespace-pre-wrap">{error}</span><button className="underline" onClick={() => setError('')}>Dismiss</button></div>}
      <div className="rounded-3xl border border-line bg-raised p-2 shadow-lg focus-within:border-accent/60">
        {files.length > 0 && (
          <ul className="flex flex-wrap gap-2 px-2 pb-2 pt-1" aria-label="Attachments">
            {files.map((f) => (
              <li key={f.id} className="relative">
                {f.kind === 'image' ? <img src={`/media/${f.id}`} alt={f.name} className="h-16 w-16 rounded-xl border border-line object-cover" />
                  : <span className="flex h-16 items-center gap-2 rounded-xl border border-line bg-bg px-3 text-xs"><FiFile aria-hidden />{f.name}</span>}
                <button aria-label={`Remove ${f.name}`} onClick={() => setFiles(files.filter((x) => x.id !== f.id))} className="absolute -right-1.5 -top-1.5 rounded-full bg-fg p-0.5 text-bg"><FiX aria-hidden size={12} /></button>
              </li>))}
          </ul>)}
        <textarea ref={ta} value={text} rows={1} placeholder={rec === 'recording' ? `${kind === 'note' ? 'Recording a voice message' : 'Listening'}… ${secs}s (click the square to finish)` : rec === 'transcribing' ? 'Transcribing…' : placeholder ?? 'Message the agent…'}
          aria-label="Message" disabled={rec !== 'idle'}
          onChange={(e) => setText(e.target.value)} onPaste={onPaste}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) { e.preventDefault(); submit() } else if (e.key === 'Escape' && rec === 'recording') cancelRec() }}
          className="block max-h-56 w-full resize-none border-0 bg-transparent px-3 py-2 text-[15px] text-fg shadow-none placeholder:text-mute" />
        <div className="flex items-center justify-between gap-2 px-1 pt-1">
          <div className="flex min-w-0 items-center gap-1">
            <button aria-label="Attach files" title="Attach images, audio, video, PDFs…" className="rounded-full p-2 text-mute hover:bg-line/60 hover:text-fg" onClick={async () => { try { addPaths((await api.PickMedia()) ?? []) } catch (e) { setError(errMsg(e)) } }}><FiPaperclip aria-hidden /></button>
            {footerLeft}
          </div>
          <div className="flex items-center gap-1.5">
            <button aria-label={rec === 'recording' && kind === 'note' ? 'Stop voice message' : 'Record a voice message'} title={rec === 'recording' && kind === 'note' ? 'Stop and attach the audio' : 'Record a voice message (sent as audio, with a transcript when available)'}
              disabled={rec === 'transcribing' || (rec === 'recording' && kind !== 'note')} onClick={note}
              className={`rounded-full p-2 ${rec === 'recording' && kind === 'note' ? 'animate-pulse bg-bad text-white' : 'text-mute hover:bg-line/60 hover:text-fg'}`}>{rec === 'recording' && kind === 'note' ? <FiSquare aria-hidden /> : <FiVolume2 aria-hidden />}</button>
            <button aria-label={rec === 'recording' && kind === 'dictate' ? 'Stop recording' : 'Dictate with your voice'} title={rec === 'recording' && kind === 'dictate' ? 'Stop and transcribe' : 'Dictate (speech to text)'}
              disabled={rec === 'transcribing' || (rec === 'recording' && kind !== 'dictate')} onClick={mic}
              className={`rounded-full p-2 ${rec === 'recording' && kind === 'dictate' ? 'animate-pulse bg-bad text-white' : 'text-mute hover:bg-line/60 hover:text-fg'}`}>{rec === 'recording' && kind === 'dictate' ? <FiSquare aria-hidden /> : <FiMic aria-hidden />}</button>
            <button aria-label="Send" onClick={submit} disabled={busy || rec !== 'idle' || (!text.trim() && files.length === 0)}
              className="rounded-full bg-fg p-2 text-bg disabled:opacity-30"><FiArrowUp aria-hidden /></button>
          </div>
        </div>
      </div>
      <p className="mt-2 text-center text-xs text-mute">{hint ?? 'Enter to send · Shift+Enter for a new line · paste or drop images, video, audio'}</p>
    </div>
  )
}
