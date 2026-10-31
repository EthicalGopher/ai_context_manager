import { createContext, useContext, useMemo } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { FiFile, FiFileText } from 'react-icons/fi'
import { VideoPlayer } from './VideoPlayer'

export interface MediaInfo { id: number; name: string; kind: string; size: number }
/** Lets embedded `media:ID` references render as the right element (image, video, audio, file). */
export const MediaCatalog = createContext<Map<number, MediaInfo>>(new Map())

const refId = (src?: string) => { const m = /^media:(\d+)$/.exec(src ?? ''); return m ? Number(m[1]) : 0 }
const url = (id: number) => `/media/${id}`

function Embed({ id, caption }: { id: number; caption: string }) {
  const m = useContext(MediaCatalog).get(id)
  const kind = m?.kind ?? 'image'
  return (
    <figure className="relative my-6 overflow-hidden rounded-2xl border border-line bg-raised">
      {kind === 'image' && <img src={url(id)} alt={caption || m?.name || 'image'} loading="lazy" className="mx-auto block max-h-[34rem] w-full object-contain" />}
      {kind === 'video' && <VideoPlayer id={id} name={m?.name ?? 'video'} />}
      {kind === 'audio' && <div className="p-6"><audio src={url(id)} controls className="w-full" /></div>}
      {(kind === 'pdf' || kind === 'file') && <div className="flex items-center gap-3 p-6">{kind === 'pdf' ? <FiFileText aria-hidden /> : <FiFile aria-hidden />}{m?.name ?? 'file'}</div>}
      {caption && <figcaption className="absolute bottom-3 left-3 rounded-md bg-black/70 px-2 py-1 text-[11px] font-semibold text-white">{caption}</figcaption>}
    </figure>
  )
}

/** Renders a page body: headings, text, callouts and embedded media. Links are inert. */
export function RichMarkdown({ text }: { text: string }) {
  const comps = useMemo(() => ({
    img: ({ src, alt }: { src?: string; alt?: string }) => { const id = refId(src); return id ? <Embed id={id} caption={alt ?? ''} /> : <span className="text-mute">[image: {alt}]</span> },
    a: ({ children, href }: { children?: React.ReactNode; href?: string }) => {
      const id = refId(href)
      if (id) return <Embed id={id} caption={String(children ?? '')} />
      return <span className="underline decoration-dotted" title={href}>{children}</span>
    },
    p: ({ children }: { children?: React.ReactNode }) => <p className="my-3 leading-7 text-fg/90">{children}</p>,
    h1: ({ children }: { children?: React.ReactNode }) => <h2 className="mb-2 mt-10 text-3xl font-bold">{children}</h2>,
    h2: ({ children }: { children?: React.ReactNode }) => <h2 className="mb-2 mt-10 text-2xl font-bold">{children}</h2>,
    h3: ({ children }: { children?: React.ReactNode }) => <h3 className="mb-1 mt-7 text-sm font-bold uppercase tracking-wider text-accent">{children}</h3>,
    blockquote: ({ children }: { children?: React.ReactNode }) => <blockquote className="my-5 rounded-2xl border border-line border-l-4 border-l-accent2 bg-raised/60 px-5 py-3 text-fg/90">{children}</blockquote>,
    ul: ({ children }: { children?: React.ReactNode }) => <ul className="my-3 list-disc space-y-1 pl-6">{children}</ul>,
    ol: ({ children }: { children?: React.ReactNode }) => <ol className="my-3 list-decimal space-y-1 pl-6">{children}</ol>,
    hr: () => <hr className="my-8 border-line" />,
    pre: ({ children }: { children?: React.ReactNode }) => <pre className="my-4 overflow-auto rounded-xl border border-line bg-bg p-4 text-xs">{children}</pre>,
    code: ({ children, className }: { children?: React.ReactNode; className?: string }) => <code className={className ? '' : 'rounded bg-raised px-1 py-0.5'}>{children}</code>,
    table: ({ children }: { children?: React.ReactNode }) => <div className="my-4 overflow-auto"><table className="w-full text-left text-sm">{children}</table></div>,
    th: ({ children }: { children?: React.ReactNode }) => <th className="border-b border-line px-3 py-2 text-xs uppercase text-mute">{children}</th>,
    td: ({ children }: { children?: React.ReactNode }) => <td className="border-b border-line/60 px-3 py-2">{children}</td>,
  }), [])
  return <ReactMarkdown remarkPlugins={[remarkGfm]} urlTransform={(u) => (u.startsWith('media:') ? u : u)} components={comps as any}>{text}</ReactMarkdown>
}
