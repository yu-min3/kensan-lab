import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { api, ApiError } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody, CardFoot } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { ErrorState, Skeleton } from "../components/ui/states";

// メモページ。旧 kensan の「メモが消える / モーダルが欄外に出る」バグの根絶が rebuild の動機。
// だからこのページは: モーダル無し・インライン追記・保存は楽観ロック・競合は隠さず伝える。
export function MemoPage() {
  const qc = useQueryClient();
  const memo = useQuery({ queryKey: ["file", "memos.md"], queryFn: () => api.file("memos.md") });

  // draft は「fork 元の mtime」と一緒に保持する（DailyPage と同じ理由 — refetch で
  // doc.mtime が進んでも、保存は編集開始時点の mtime で行い、外部編集を確実に 409 にする）
  const [draft, setDraft] = useState<{ content: string; baseMtime: string } | null>(null);
  const [quick, setQuick] = useState("");

  const save = useMutation({
    mutationFn: ({ content, baseMtime }: { content: string; baseMtime: string }) =>
      api.putFile("memos.md", content, baseMtime),
    onSuccess: () => {
      setDraft(null);
      setQuick("");
      qc.invalidateQueries({ queryKey: ["file", "memos.md"] });
    },
  });

  // クイック追記: ## Scratch セクションの末尾に 1 行足して保存。
  // draft が dirty ならその fork 元 mtime を、そうでなければ最新 fetch の mtime を使う
  function appendQuick() {
    if (!memo.data || !quick.trim()) return;
    const content = draft?.content ?? memo.data.content;
    const baseMtime = draft?.baseMtime ?? memo.data.doc.mtime;
    const updated = content.includes("## Scratch")
      ? content.replace(/(## Scratch[\s\S]*?)(\n## |$)/, (_, scratch, next) =>
          `${scratch.trimEnd()}\n\n${quick.trim()}\n${next === "\n## " ? "\n## " : next}`)
      : `${content.trimEnd()}\n\n## Scratch\n\n${quick.trim()}\n`;
    save.mutate({ content: updated, baseMtime });
  }

  const conflict = save.error instanceof ApiError && save.error.status === 409;
  const dirty = draft !== null && draft.content !== memo.data?.content;

  return (
    <>
      <PageHeader
        eyebrow="記録 · memos.md"
        title="メモ"
        sub="Pinned と Scratch。思いついたらここに落とす。整理は週次レビューで。"
        actions={
          memo.data && (
            <Button
              variant="primary"
              size="sm"
              disabled={!dirty}
              loading={save.isPending && !quick}
              onClick={() => draft && save.mutate(draft)}
            >
              保存
            </Button>
          )
        }
      />
      <div className="ds-section">
        <Card>
          <CardBody className="flex gap-2">
            <label htmlFor="quick-memo" className="sr-only">
              クイックメモ
            </label>
            <input
              id="quick-memo"
              className="ds-control flex-1 rounded-md border border-border bg-card px-3 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              placeholder="Scratch に追記…"
              value={quick}
              onChange={(e) => setQuick(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !e.nativeEvent.isComposing && appendQuick()}
            />
            <Button
              variant="secondary"
              loading={save.isPending && !!quick}
              disabled={!quick.trim() || !memo.data}
              onClick={appendQuick}
            >
              <Plus size={16} />
              追記
            </Button>
          </CardBody>
        </Card>

        <Card>
          {memo.isPending ? (
            <CardBody>
              <Skeleton className="h-64 w-full" />
            </CardBody>
          ) : memo.isError ? (
            <CardBody>
              <ErrorState error={memo.error} onRetry={() => memo.refetch()} />
            </CardBody>
          ) : (
            <>
              <CardBody>
                <textarea
                  className="w-full min-h-[24rem] resize-y bg-transparent text-sm leading-relaxed focus:outline-none font-sans"
                  value={draft?.content ?? memo.data.content}
                  onChange={(e) => {
                    const content = e.target.value;
                    setDraft((prev) => ({
                      content,
                      baseMtime: prev?.baseMtime ?? memo.data!.doc.mtime,
                    }));
                  }}
                  aria-label="メモ全文"
                />
              </CardBody>
              {(conflict || save.isError) && (
                <CardFoot>
                  <ErrorState
                    error={
                      conflict
                        ? new ApiError(409, "他のクライアントが先に保存しました。再読込してから編集し直してください。")
                        : save.error
                    }
                    onRetry={() => {
                      save.reset();
                      setDraft(null);
                      memo.refetch();
                    }}
                  />
                </CardFoot>
              )}
            </>
          )}
        </Card>
      </div>
    </>
  );
}
