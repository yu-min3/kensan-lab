import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Trash2, Undo2 } from "lucide-react";
import { api, type TrashEntry } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Empty, ErrorState, SkeletonRows } from "../components/ui/states";

// ゴミ箱 — app で削除したタスク・マイルストーンの退避先（.kensan/trash.md）。
// 普段は目立たない（サイドバー最下部の小さなアイコンからだけ来られる）。
// 「復元」で元のファイル・セクションへ戻す。元が消えていれば todo.md ## Now へ。
export function TrashPage() {
  const qc = useQueryClient();
  const trash = useQuery({ queryKey: ["trash"], queryFn: api.trash });
  const [lastError, setLastError] = useState<string | null>(null);

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["trash"] });
    qc.invalidateQueries({ queryKey: ["board"] });
    qc.invalidateQueries({ queryKey: ["projects"] });
  };
  const onErr = (e: unknown) => setLastError(String(e instanceof Error ? e.message : e));
  const restore = useMutation({
    mutationFn: (e: TrashEntry) => api.trashRestore(e),
    onSuccess: () => setLastError(null),
    onError: onErr,
    onSettled: invalidate,
  });
  const purge = useMutation({
    mutationFn: (e: TrashEntry) => api.trashPurge(e),
    onSuccess: () => setLastError(null),
    onError: onErr,
    onSettled: invalidate,
  });
  const busy = restore.isPending || purge.isPending;
  const items = trash.data?.items ?? [];

  return (
    <>
      <PageHeader
        eyebrow="ゴミ箱 · .kensan/trash.md"
        title="ゴミ箱"
        sub="app で削除したタスクの退避先。復元すると元のファイル・セクションに戻る（元が無い場合は todo.md ## Now）。"
      />
      {lastError && (
        <div className="mb-4 text-sm text-destructive border border-destructive/30 bg-destructive/5 rounded-md px-3 py-2">
          {lastError}
        </div>
      )}
      <Card>
        <CardBody className={items.length > 0 ? "!p-2" : undefined}>
          {trash.isPending ? (
            <SkeletonRows rows={4} />
          ) : trash.isError ? (
            <ErrorState error={trash.error} onRetry={() => trash.refetch()} />
          ) : items.length === 0 ? (
            <Empty
              icon={<Trash2 />}
              title="ゴミ箱は空です"
              desc="タスクボードやプロジェクトで削除したものがここに積まれます。"
            />
          ) : (
            <ul className="ds-stack !gap-0">
              {items.map((e) => (
                <li
                  key={`${e.line}:${e.raw.slice(0, 20)}`}
                  className="flex items-center gap-2 px-2 py-2 border-b border-border last:border-b-0 group"
                >
                  <span className="flex-1 text-sm min-w-0">
                    <span className={e.state !== "todo" ? "line-through text-muted-foreground" : undefined}>
                      {e.display}
                    </span>
                  </span>
                  {e.from && (
                    <Badge variant="outline">
                      {projectName(e.from) ?? e.from}
                      {e.section ? ` ▸ ${e.section}` : ""}
                    </Badge>
                  )}
                  {e.deleted && (
                    <span className="font-mono tnum text-[11px] text-muted-foreground shrink-0">{e.deleted}</span>
                  )}
                  <Button variant="ghost" size="sm" disabled={busy} onClick={() => restore.mutate(e)} title="元の場所へ復元">
                    <Undo2 size={14} />
                    復元
                  </Button>
                  <button
                    className="size-6 grid place-items-center rounded-md text-destructive hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                    disabled={busy}
                    onClick={() => purge.mutate(e)}
                    title="完全に削除（復元不可）"
                    aria-label="完全に削除"
                  >
                    <Trash2 size={13} />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>
    </>
  );
}

// projects/<name>/README.md → <name>（それ以外は null でパス表示のまま）
function projectName(file: string): string | null {
  const m = file.match(/^projects\/([^/]+)\/README\.md$/);
  return m ? m[1] : null;
}
