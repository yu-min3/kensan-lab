import { useState, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { BookOpen } from "lucide-react";
import { api, ApiError, todayISO, dailyPath, dailySkeleton } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody, CardFoot } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Empty, ErrorState, Skeleton } from "../components/ui/states";

// 日記ページ。Markdown は当面プレーン編集（リッチ表示は Phase 4 で検討）。
// 保存は mtime 楽観ロック付き — Claude / VSCode が先に書いてたら 409 を素直に伝える。
export function DailyPage() {
  const [params] = useSearchParams();
  const date = params.get("date") ?? todayISO();
  const qc = useQueryClient();

  const daily = useQuery({
    queryKey: ["daily", date],
    queryFn: () => api.daily(date),
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  });

  // draft は「fork 元の mtime」と一緒に保持する。
  // refetch が daily.data.doc.mtime を進めても、保存は必ず編集開始時点の mtime で行う
  // （でないと外部編集を 409 にできず黙って上書きしてしまう）
  const [draft, setDraft] = useState<{ content: string; baseMtime: string } | null>(null);
  useEffect(() => setDraft(null), [date]);

  const save = useMutation({
    mutationFn: () => api.putFile(dailyPath(date), draft!.content, draft!.baseMtime),
    onSuccess: () => {
      setDraft(null);
      qc.invalidateQueries({ queryKey: ["daily", date] });
    },
  });

  const create = useMutation({
    mutationFn: () => api.createFile(dailyPath(date), dailySkeleton(date)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["daily", date] }),
  });

  const notFound = daily.error instanceof ApiError && daily.error.status === 404;
  const conflict = save.error instanceof ApiError && save.error.status === 409;
  const dirty = draft !== null && draft.content !== daily.data?.content;

  return (
    <>
      <PageHeader
        eyebrow={`記録 · ${dailyPath(date)}`}
        title={`${date} の日記`}
        sub="その日の出来事・感想・学び。完了タスクは /reflection がここに移してくる。"
        actions={
          daily.data && (
            <Button
              variant="primary"
              size="sm"
              disabled={!dirty}
              loading={save.isPending}
              onClick={() => save.mutate()}
            >
              保存
            </Button>
          )
        }
      />
      <Card>
        {daily.isPending ? (
          <CardBody>
            <Skeleton className="h-64 w-full" />
          </CardBody>
        ) : notFound ? (
          <CardBody>
            <Empty
              icon={<BookOpen />}
              title={`${date} の日記はまだありません`}
              desc="骨組み（日記セクション付き）を作成して書き始められます。"
              actions={
                <Button variant="primary" loading={create.isPending} onClick={() => create.mutate()}>
                  日記を作成
                </Button>
              }
            />
          </CardBody>
        ) : daily.isError ? (
          <CardBody>
            <ErrorState error={daily.error} onRetry={() => daily.refetch()} />
          </CardBody>
        ) : (
          <>
            <CardBody>
              <textarea
                className="w-full min-h-[28rem] resize-y bg-transparent text-sm leading-relaxed focus:outline-none font-sans"
                value={draft?.content ?? daily.data.content}
                onChange={(e) => {
                  const content = e.target.value;
                  setDraft((prev) => ({
                    content,
                    baseMtime: prev?.baseMtime ?? daily.data!.doc.mtime,
                  }));
                }}
                aria-label="日記本文"
              />
            </CardBody>
            {(conflict || save.isError) && (
              <CardFoot>
                <ErrorState
                  error={
                    conflict
                      ? new ApiError(409, "他のクライアント（Claude Code / VSCode）が先に保存しました。再読込してから編集し直してください。")
                      : save.error
                  }
                  onRetry={() => {
                    save.reset();
                    setDraft(null);
                    daily.refetch();
                  }}
                />
              </CardFoot>
            )}
          </>
        )}
      </Card>
    </>
  );
}
