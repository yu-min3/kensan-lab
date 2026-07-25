import ReactMarkdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";

function safeUrlTransform(url: string): string {
  try {
    const parsed = new URL(url);
    if (parsed.protocol === "http:" || parsed.protocol === "https:") {
      return defaultUrlTransform(url);
    }
  } catch {
    // Relative URL and invalid URL are intentionally not rendered as links.
  }
  return "";
}

export function MarkdownView({ content }: { content: string }) {
  return (
    <div className="min-w-0 overflow-hidden break-words [overflow-wrap:anywhere] text-sm leading-relaxed text-foreground">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        urlTransform={safeUrlTransform}
        components={{
          h2: ({ children }) => (
            <h2 className="h-serif text-xl font-bold mt-8 mb-3 first:mt-0 border-b border-border pb-2">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className="h-serif text-base font-semibold mt-6 mb-2 [overflow-wrap:anywhere]">{children}</h3>
          ),
          p: ({ children }) => <p className="my-3 leading-7 [overflow-wrap:anywhere]">{children}</p>,
          ul: ({ children }) => <ul className="my-3 pl-5 list-disc space-y-1.5">{children}</ul>,
          ol: ({ children }) => <ol className="my-3 pl-5 list-decimal space-y-1.5">{children}</ol>,
          li: ({ children }) => <li className="pl-1">{children}</li>,
          strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
          blockquote: ({ children }) => (
            <blockquote className="my-4 border-l-2 border-brand pl-4 text-muted-foreground">{children}</blockquote>
          ),
          code: ({ children }) => (
            <code className="font-mono text-xs rounded-sm bg-muted px-1.5 py-0.5">{children}</code>
          ),
          a: ({ href, children }) =>
            href ? (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="text-brand underline underline-offset-2 hover:no-underline break-all"
              >
                {children}
              </a>
            ) : (
              <span>{children}</span>
            ),
          img: () => null,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
