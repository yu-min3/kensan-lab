import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { FileText } from "lucide-react";
import { api } from "../lib/api";
import { splitFrontmatter } from "../hooks/useAutosaveFile";
import { resolveMarkdownPath, viewerURL } from "../lib/markdownViewer";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody } from "../components/ui/card";
import { Empty, ErrorState, Skeleton } from "../components/ui/states";

export function MarkdownViewerPage() {
  const [params] = useSearchParams();
  const path = params.get("path") ?? "";
  const detail = useQuery({
    queryKey: ["file", path],
    queryFn: () => api.file(path),
    enabled: path.endsWith(".md"),
  });

  if (!path.endsWith(".md")) {
    return <Empty icon={<FileText />} title="Markdownを指定してください" desc="URLの path にworkspace内の .md ファイルを指定します。" />;
  }

  const title = detail.data?.doc.meta.title || path.split("/").pop()?.replace(/\.md$/, "") || "Markdown";

  return (
    <>
      <PageHeader eyebrow="Markdown Viewer" title={title} sub={path} />
      <Card className="markdown-viewer mx-auto max-w-[1100px]">
        <CardBody>
          {detail.isPending ? (
            <Skeleton className="h-[70vh] w-full" />
          ) : detail.isError ? (
            <ErrorState error={detail.error} onRetry={() => detail.refetch()} />
          ) : (
            <article className="markdown-body">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  a: ({ href = "", children, ...props }) => {
                    const resolved = resolveMarkdownPath(path, href);
                    const external = /^https?:/i.test(href);
                    return (
                      <a
                        {...props}
                        href={resolved ? viewerURL(resolved) : href}
                        target={external ? "_blank" : undefined}
                        rel={external ? "noreferrer" : undefined}
                      >
                        {children}
                      </a>
                    );
                  },
                }}
              >
                {splitFrontmatter(detail.data.content).body}
              </ReactMarkdown>
            </article>
          )}
        </CardBody>
      </Card>
    </>
  );
}
