import { renderToStaticMarkup } from 'react-dom/server'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import viewerJs from './mediaViewer.js?raw'
import viewerCss from './mediaViewer.css?raw'
import type { MediaInfo } from '../components/RichMarkdown'

interface ExportablePage { title: string; summary: string; body: string; updatedAt: string }

const refId = (src?: string) => { const m = /^media:(\d+)$/.exec(src ?? ''); return m ? Number(m[1]) : 0 }

// The shared page theme: same colour tokens as the app, light or dark by the reader's system setting.
const pageCss = `
:root{--bg:243 243 247;--panel:255 255 255;--raised:238 238 245;--line:221 221 232;--fg:24 24 32;--mute:106 106 125;--accent:62 80 210;color-scheme:light}
@media (prefers-color-scheme:dark){:root{--bg:17 17 20;--panel:25 25 29;--raised:35 35 41;--line:44 44 51;--fg:238 238 242;--mute:127 127 140;--accent:109 124 240;color-scheme:dark}}
*{box-sizing:border-box}
body{margin:0;background:rgb(var(--bg));color:rgb(var(--fg));font:16px/1.7 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
main{max-width:52rem;margin:0 auto;padding:2.5rem 1.25rem 5rem}
h1{font-size:2.6rem;line-height:1.15;margin:.2rem 0 .5rem}
h2{font-size:1.7rem;margin:2.2rem 0 .5rem}
h3{font-size:.85rem;text-transform:uppercase;letter-spacing:.08em;color:rgb(var(--accent));margin:1.6rem 0 .3rem}
.sum{font-size:1.15rem;color:rgb(var(--mute));margin:.2rem 0 0}
.meta{font-size:.75rem;text-transform:uppercase;letter-spacing:.12em;color:rgb(var(--mute));border-bottom:1px solid rgb(var(--line));padding-bottom:.6rem;margin-bottom:1.6rem}
blockquote{margin:1.2rem 0;padding:.6rem 1.1rem;border:1px solid rgb(var(--line));border-left:4px solid rgb(var(--accent));border-radius:.9rem;background:rgb(var(--raised))}
pre{overflow:auto;padding:1rem;border:1px solid rgb(var(--line));border-radius:.8rem;background:rgb(var(--panel));font-size:.8rem}
code{font-family:ui-monospace,Menlo,Consolas,monospace;font-size:.9em}
:not(pre)>code{background:rgb(var(--raised));padding:.1rem .3rem;border-radius:.3rem}
table{border-collapse:collapse;width:100%;font-size:.9rem}
th,td{border-bottom:1px solid rgb(var(--line));padding:.5rem .7rem;text-align:left}
th{font-size:.75rem;text-transform:uppercase;color:rgb(var(--mute))}
hr{border:0;border-top:1px solid rgb(var(--line));margin:2rem 0}
figure{margin:1.5rem 0}
audio{width:100%}
.file{padding:1rem 1.2rem;border:1px solid rgb(var(--line));border-radius:.9rem;background:rgb(var(--raised))}
footer{margin-top:3rem;font-size:.75rem;color:rgb(var(--mute))}
`

function Media({ id, caption, catalog }: { id: number; caption: string; catalog: Map<number, MediaInfo> }) {
  const m = catalog.get(id)
  const kind = m?.kind ?? 'image'
  const src = `/media/${id}`
  if (kind === 'image' || kind === 'video') return <figure><div className="mv" data-kind={kind} data-src={src} data-caption={caption || (kind === 'video' ? m?.name ?? '' : '')} /></figure>
  if (kind === 'audio') return <figure><audio controls preload="metadata" src={src} /></figure>
  return <p className="file">{m?.name ?? 'file'} (not included in this copy)</p>
}

/** The shareable copy of a page: markup with media as /media/<id>, plus the stylesheet and viewer script. The backend embeds the media. */
export function buildPageExport(page: ExportablePage, catalog: Map<number, MediaInfo>) {
  const comps = {
    img: ({ src, alt }: { src?: string; alt?: string }) => { const id = refId(src); return id ? <Media id={id} caption={alt ?? ''} catalog={catalog} /> : <span>[image: {alt}]</span> },
    a: ({ children, href }: { children?: React.ReactNode; href?: string }) => {
      const id = refId(href)
      return id ? <Media id={id} caption={String(children ?? '')} catalog={catalog} /> : <span title={href}>{children}</span>
    },
  }
  const body = renderToStaticMarkup(
    <main>
      <p className="meta">Shared page · updated {new Date(page.updatedAt).toLocaleDateString()}</p>
      <h1>{page.title}</h1>
      {page.summary && <p className="sum">{page.summary}</p>}
      <ReactMarkdown remarkPlugins={[remarkGfm]} urlTransform={(u) => u} components={comps as any}>{page.body}</ReactMarkdown>
      <footer>Made with AI Context Workspace. Zoom with Ctrl + scroll, + and −; space plays and pauses.</footer>
    </main>,
  )
  return { title: page.title, body, css: pageCss + viewerCss, js: viewerJs + '\nwindow.aicwMountAll(document);' }
}
