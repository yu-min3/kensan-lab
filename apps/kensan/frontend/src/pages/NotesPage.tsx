import { useState } from "react";
import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { FileText, Search } from "lucide-react";
import clsx from "clsx";
import { api } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Empty, ErrorState, SkeletonRows, Skeleton } from "../components/ui/states";

// ノート閲覧 + 全文検索。patterns.md 06. List · Detail。
// 編集はしない（ノートの編集は Claude Code / VSCode の領分。app は読む場所）。

// conventions.md の type のうち閲覧価値のあるもの（daily は専用ページがあるため除外）
const TYPE_CHIPS = ["note", "book", "project", "goal", "memo", "review"] as const;

export function NotesPage() {
  const [params, setParams] = useSearchParams();
  const selected = params.get("path");
  const [type, setType] = useState<string>("note");
  const [q, setQ] = useState("");
  const [submitted, setSubmitted] = useState("");

  const list = useQuery({
    queryKey: ["files", type],
    queryFn: () => api.files({ type }),
    placeholderData: keepPreviousData,
  });
  // 1 文字でも検索できる（日本語 1 字のクエリは普通にある）
  const search = useQuery({
    queryKey: ["search", submitted],
    queryFn: () => api.search(submitted),
    enabled: submitted.length > 0,
  });
  const detail = useQuery({
    queryKey: ["file", selected],
    queryFn: () => api.file(selected!),
    enabled: !!selected,
  });

  const searching = submitted.length > 0;

  return (
    <>
      <PageHeader
        eyebrow="記録 · notes/ books/ ほか"
        title="ノート"
        sub="技術ノート・読書記録の閲覧と全文検索。編集は Claude Code / VSCode で。"
      />
      <div className="grid grid-cols-[320px_1fr] gap-6 items-start">
        <div className="ds-section">
          <Card>
            <CardBody className="ds-stack">
              <div className="flex gap-2">
                <label htmlFor="note-search" className="sr-only">全文検索</label>
                <div className="relative flex-1">
                  <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
                  <input
                    id="note-search"
                    className="ds-control w-full rounded-md border border-border bg-card pl-8 pr-3 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    placeholder="全文検索（Enter）"
                    value={q}
                    onChange={(e) => setQ(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.nativeEvent.isComposing) setSubmitted(q.trim());
                      if (e.key === "Escape") { setQ(""); setSubmitted(""); }
                    }}
                  />
                </div>
              </div>
              {!searching && (
                <div className="flex flex-wrap gap-1.5">
                  {TYPE_CHIPS.map((t) => (
                    <button
                      key={t}
                      onClick={() => setType(t)}
                      className={clsx(
                        "px-2 py-0.5 rounded-full text-xs border",
                        type === t
                          ? "bg-brand-muted text-accent-foreground border-transparent font-semibold"
                          : "border-border-strong text-muted-foreground hover:bg-accent",
                      )}
                    >
                      {t}
                    </button>
                  ))}
                </div>
              )}
            </CardBody>
          </Card>

          <Card>
            <CardBody className="!p-2 max-h-[60vh] overflow-y-auto">
              {searching ? (
                <SearchResults
                  query={submitted}
                  result={search}
                  onOpen={(p) => setParams({ path: p })}
                  selected={selected}
                />
              ) : list.isPending ? (
                <SkeletonRows rows={8} />
              ) : list.isError ? (
                <ErrorState error={list.error} onRetry={() => list.refetch()} />
              ) : list.data.files.length === 0 ? (
                <Empty
                  icon={<FileText />}
                  title={`type: ${type} のファイルがありません`}
                  desc="別の type チップを選ぶか、Claude Code でノートを作成してください。"
                />
              ) : (
                <ul>
                  {list.data.files.map((f) => (
                    <li key={f.path}>
                      <button
                        onClick={() => setParams({ path: f.path })}
                        className={clsx(
                          "w-full px-2 py-1.5 rounded-md text-left",
                          selected === f.path ? "bg-accent text-accent-foreground" : "hover:bg-accent/60",
                        )}
                      >
                        <div className="text-sm truncate">
                          {f.meta.title || f.path.split("/").pop()?.replace(".md", "")}
                        </div>
                        <div className="font-mono tnum text-[11px] text-muted-foreground truncate">
                          {f.path}
                        </div>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </CardBody>
          </Card>
        </div>

        <Card className="min-h-[60vh]">
          {!selected ? (
            <CardBody>
              <Empty
                icon={<FileText />}
                title="ノートを選択してください"
                desc="左の一覧か検索結果から開くと、ここに本文が表示されます。"
              />
            </CardBody>
          ) : detail.isPending ? (
            <CardBody><Skeleton className="h-96 w-full" /></CardBody>
          ) : detail.isError ? (
            <CardBody><ErrorState error={detail.error} onRetry={() => detail.refetch()} /></CardBody>
          ) : (
            <CardBody>
              <div className="flex items-center gap-2 pb-3 mb-3 border-b border-border">
                <span className="font-mono tnum text-xs text-muted-foreground flex-1 truncate">
                  {detail.data.doc.path}
                </span>
                {detail.data.doc.meta.type && <Badge variant="outline">{detail.data.doc.meta.type}</Badge>}
                {detail.data.doc.meta.tags?.slice(0, 4).map((t) => (
                  <Badge key={t} variant="muted">{t}</Badge>
                ))}
              </div>
              <pre className="whitespace-pre-wrap font-sans text-sm leading-relaxed">
                {detail.data.content}
              </pre>
            </CardBody>
          )}
        </Card>
      </div>
    </>
  );
}

function SearchResults({
  query,
  result,
  onOpen,
  selected,
}: {
  query: string;
  result: ReturnType<typeof useQuery<{ hits: { path: string; line: number; snippet: string }[]; total: number; truncated: boolean }, Error>>;
  onOpen: (path: string) => void;
  selected: string | null;
}) {
  if (result.isPending) return <SkeletonRows rows={6} />;
  if (result.isError) return <ErrorState error={result.error} onRetry={() => result.refetch()} />;
  if (result.data.hits.length === 0) {
    return (
      <Empty
        icon={<Search />}
        title={`「${query}」は見つかりませんでした`}
        desc="表記ゆれ（カタカナ/英語）を変えて試すか、Esc で一覧に戻れます。"
      />
    );
  }
  return (
    <ul>
      {result.data.truncated && (
        <li className="px-2 py-1 text-[11px] text-muted-foreground">上位 100 件のみ表示</li>
      )}
      {result.data.hits.map((h, i) => (
        <li key={`${h.path}:${h.line}:${i}`}>
          <button
            onClick={() => onOpen(h.path)}
            className={clsx(
              "w-full px-2 py-1.5 rounded-md text-left",
              selected === h.path ? "bg-accent text-accent-foreground" : "hover:bg-accent/60",
            )}
          >
            <div className="font-mono tnum text-[11px] text-muted-foreground truncate">
              {h.path}:{h.line}
            </div>
            <div className="text-sm truncate">{h.snippet}</div>
          </button>
        </li>
      ))}
    </ul>
  );
}
