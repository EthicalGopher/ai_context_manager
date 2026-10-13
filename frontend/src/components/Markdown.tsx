import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

/** Renders assistant/user Markdown. Links are inert so the webview never navigates away. */
export function Markdown({ text }: { text: string }) {
  return (
    <div className="md">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={{
        a: ({ children, href }) => <span className="underline decoration-dotted" title={href}>{children}</span>,
        img: ({ alt }) => <span className="text-mute">[image: {alt}]</span>,
        pre: ({ children }) => <pre className="my-2 overflow-auto rounded-lg border border-line bg-bg p-3 text-xs">{children}</pre>,
        code: ({ children, className }) => <code className={className ? '' : 'rounded bg-raised px-1 py-0.5'}>{children}</code>,
        ul: ({ children }) => <ul className="my-2 list-disc space-y-1 pl-6">{children}</ul>,
        ol: ({ children }) => <ol className="my-2 list-decimal space-y-1 pl-6">{children}</ol>,
        p: ({ children }) => <p className="my-2 leading-7">{children}</p>,
        table: ({ children }) => <div className="my-2 overflow-auto"><table className="text-sm">{children}</table></div>,
      }}>{text}</ReactMarkdown>
    </div>
  )
}
