import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkBreaks from 'remark-breaks'
import rehypeRaw from 'rehype-raw'
import type { Components } from 'react-markdown'

interface MarkdownContentProps {
  content: string
}

const components: Components = {
  p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed">{children}</p>,
  h1: ({ children }) => (
    <h3 className="text-base font-bold mt-3 mb-1.5 first:mt-0 border-b border-muted-foreground/15 pb-1">
      {children}
    </h3>
  ),
  h2: ({ children }) => (
    <h4 className="text-[0.9rem] font-bold mt-3 mb-1 first:mt-0">{children}</h4>
  ),
  h3: ({ children }) => (
    <h5 className="text-sm font-semibold mt-2.5 mb-1 first:mt-0">{children}</h5>
  ),
  ul: ({ children }) => <ul className="mb-2 ml-4 list-disc last:mb-0 space-y-0.5">{children}</ul>,
  ol: ({ children }) => <ol className="mb-2 ml-4 list-decimal last:mb-0 space-y-0.5">{children}</ol>,
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  strong: ({ children }) => <strong className="font-semibold text-foreground">{children}</strong>,
  em: ({ children }) => <em className="text-muted-foreground italic">{children}</em>,
  code: ({ children, className }) => {
    const isBlock = className?.includes('language-')
    if (isBlock) {
      return (
        <code className="block my-2 rounded-md bg-muted-foreground/10 px-3 py-2 text-xs leading-relaxed overflow-x-auto">
          {children}
        </code>
      )
    }
    return (
      <code className="rounded bg-muted-foreground/15 px-1 py-0.5 text-xs font-mono">{children}</code>
    )
  },
  pre: ({ children }) => <pre className="my-2">{children}</pre>,
  a: ({ href, children }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-primary underline underline-offset-2 hover:text-primary/80"
    >
      {children}
    </a>
  ),
  blockquote: ({ children }) => (
    <blockquote className="my-2 border-l-2 border-primary/40 pl-3 text-muted-foreground italic">
      {children}
    </blockquote>
  ),
  hr: () => <hr className="my-3 border-muted-foreground/20" />,
  table: ({ children }) => (
    <div className="my-2 overflow-x-auto rounded-md border border-muted-foreground/20">
      <table className="w-full text-xs">{children}</table>
    </div>
  ),
  thead: ({ children }) => (
    <thead className="bg-muted-foreground/10">{children}</thead>
  ),
  th: ({ children }) => (
    <th className="px-2 py-1.5 text-left font-semibold border-b border-muted-foreground/20">
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td className="px-2 py-1.5 border-b border-muted-foreground/10">{children}</td>
  ),
}

export function MarkdownContent({ content }: MarkdownContentProps) {
  return (
    <div className="markdown-chat">
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkBreaks]} rehypePlugins={[rehypeRaw]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  )
}
