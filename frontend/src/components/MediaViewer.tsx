import { useEffect, useRef } from 'react'
import '../lib/mediaViewer.js'

/** Video player / image viewer with zoom and pan (see lib/mediaViewer.js). Mounted once per source so playback survives parent re-renders. */
export function MediaViewer({ kind, src, caption = '' }: { kind: 'video' | 'image'; src: string; caption?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const node = ref.current
    if (!node) return
    const v = window.aicwMountViewer(node)
    return () => v.destroy()
  }, [kind, src, caption])
  return <div ref={ref} className="mv" data-kind={kind} data-src={src} data-caption={caption} />
}
