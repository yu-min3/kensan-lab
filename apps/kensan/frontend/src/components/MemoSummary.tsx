import { useQuery } from "@tanstack/react-query";
import { Pin, StickyNote } from "lucide-react";
import { api } from "../lib/api";
import { parseMemos } from "../lib/memosFile";
import { Card, CardBody } from "./ui/card";
import { Skeleton } from "./ui/states";

const MAX = 6; // ダッシュボードはここまで。残りは右下 FAB で全件。

// ダッシュボード右レール用のメモ省略表示。作成・編集・削除は右下の FAB
// （FloatingMemoButton）に任せ、ここは一覧をざっと見るだけの読み取り専用に絞る。
export function MemoSummary() {
  const memo = useQuery({ queryKey: ["file", "memos.md"], queryFn: () => api.file("memos.md") });

  if (memo.isPending) {
    return (
      <Card>
        <CardBody>
          <Skeleton className="h-20 w-full" />
        </CardBody>
      </Card>
    );
  }
  if (memo.isError || !memo.data) {
    return (
      <Card>
        <CardBody>
          <p className="text-sm text-muted-foreground">メモを読み込めませんでした。</p>
        </CardBody>
      </Card>
    );
  }

  const p = parseMemos(memo.data.content);
  const total = p.pinned.length + p.blocks.length;

  if (total === 0) {
    return (
      <Card>
        <CardBody className="flex items-center gap-2 text-sm text-muted-foreground">
          <StickyNote className="size-4 shrink-0" />
          メモはまだありません。右下のボタンから追加。
        </CardBody>
      </Card>
    );
  }

  // ピン留めを先頭に、最大 MAX 件だけ 1 行ずつ
  const items = [
    ...p.pinned.map((t) => ({ text: t.replace(/^-\s+/, ""), pinned: true })),
    ...p.blocks.map((t) => ({ text: t, pinned: false })),
  ];
  const shown = items.slice(0, MAX);
  const rest = total - shown.length;

  return (
    <Card>
      <CardBody className="!py-1.5">
        <ul>
          {shown.map((m, i) => (
            <li
              key={i}
              className="flex items-start gap-2 py-1.5 border-b border-border/60 last:border-0"
            >
              {m.pinned ? (
                <Pin className="size-3.5 text-brand mt-0.5 shrink-0" />
              ) : (
                <span className="mt-1.5 size-1 rounded-full bg-muted-foreground/40 shrink-0" />
              )}
              <p
                className={
                  m.pinned
                    ? "text-sm line-clamp-2 flex-1"
                    : "text-sm text-muted-foreground line-clamp-1 flex-1"
                }
              >
                {m.text}
              </p>
            </li>
          ))}
        </ul>
        {rest > 0 && (
          <p className="text-xs text-muted-foreground pt-2">
            ほか {rest} 件 — 右下のボタンで全件
          </p>
        )}
      </CardBody>
    </Card>
  );
}
