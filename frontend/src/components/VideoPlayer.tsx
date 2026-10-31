import { useEffect, useRef, useState } from 'react'
import { FiDownload } from 'react-icons/fi'

// Only one video plays at a time across the page.
let playing: HTMLVideoElement | null = null

const SPEEDS = [0.5, 1, 1.25, 1.5, 2]

/**
 * Video that plays smoothly in the webview: metadata-only preload (no big
 * upfront read), Range-based seeking from the media server, a buffering
 * indicator, one-at-a-time playback, auto-pause when scrolled away, playback
 * speed, and a download fallback when the webview cannot decode the file.
 */
export function VideoPlayer({ id, name }: { id: number; name: string }) {
  const ref = useRef<HTMLVideoElement>(null)
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)
  const [rate, setRate] = useState(1)
  const src = `/media/${id}`

  useEffect(() => {
    const v = ref.current
    if (!v || typeof IntersectionObserver === 'undefined') return
    const io = new IntersectionObserver((es) => es.forEach((e) => { if (!e.isIntersecting && !v.paused) v.pause() }), { threshold: 0.1 })
    io.observe(v)
    return () => { io.disconnect(); if (playing === v) playing = null }
  }, [])

  if (failed) {
    return (
      <div className="flex flex-col items-center gap-3 p-8 text-center text-sm">
        <p>This video can't be played here (the format may not be supported by the app's web view).</p>
        <a href={`${src}?download=1`} download={name} className="inline-flex items-center gap-2 rounded-lg border border-line px-3 py-2 font-semibold text-accent"><FiDownload aria-hidden /> Download {name}</a>
      </div>
    )
  }
  return (
    <div className="relative bg-black">
      <video ref={ref} src={src} controls playsInline preload="metadata" aria-label={name} className="mx-auto block max-h-[34rem] w-full"
        onPlay={(e) => { const v = e.currentTarget; if (playing && playing !== v) playing.pause(); playing = v }}
        onWaiting={() => setBusy(true)} onStalled={() => setBusy(true)}
        onPlaying={() => setBusy(false)} onCanPlay={() => setBusy(false)} onPause={() => setBusy(false)}
        onError={() => setFailed(true)} />
      {busy && <div role="status" aria-label="Buffering" className="pointer-events-none absolute inset-0 flex items-center justify-center"><span className="h-10 w-10 animate-spin rounded-full border-4 border-white/30 border-t-white" /></div>}
      <label className="absolute right-2 top-2 rounded-md bg-black/70 px-1.5 py-0.5 text-[11px] font-semibold text-white">
        <span className="sr-only">Playback speed</span>
        <select value={rate} onChange={(e) => { const r = Number(e.target.value); setRate(r); if (ref.current) ref.current.playbackRate = r }} className="bg-transparent outline-none">
          {SPEEDS.map((s) => <option key={s} value={s} className="text-black">{s}×</option>)}
        </select>
      </label>
    </div>
  )
}
