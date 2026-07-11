import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Pin, Pencil, Trash2, Check, X, StickyNote } from "lucide-react";
import { api, ApiError } from "../lib/api";
import { parseMemos, serializeMemos } from "../lib/memosFile";
import { Card, CardBody } from "./ui/card";
import { Button } from "./ui/button";
import { ErrorState, Skeleton, Empty } from "./ui/states";
import { useToast } from "./ui/toast";

// メモページ用のカード一覧。実体は memos.md の ## Scratch（lib/memosFile）。
// 素早い追加・編集・削除をフルページで。グローバル FAB と同じデータを共有する。
export function MemoCards() {
  const qc = useQueryClient();
  const memo = useQuery({ queryKey: ["file", "memos.md"], queryFn: () => api.file("memos.md") });

  const [quick, setQuick] = useState("");
  const [editing, setEditing] = useState<number | null>(null);
  const [editText, setEditText] = useState("");

  const save = useMutation({
    mutationFn: (blocks: string[]) => {
      const p = parseMemos(memo.data!.content);
      return api.putFile("memos.md", serializeMemos(p, blocks), memo.data!.doc.mtime);
    },
    onSuccess: () => {
      setQuick("");
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["file", "memos.md"] });
    },
  });

  // 削除の Undo（patterns.md 04: 取り消し可能な操作は Toast + Undo）。
  // 削除確定後に押されるため、closure の古い content ではなくファイルを取り直して
  // 元の位置に差し戻す（stale mtime による 409 を避ける）。
  const toast = useToast();
  const undoDelete = useMutation({
    mutationFn: async ({ block, at }: { block: string; at: number }) => {
      const fresh = await api.file("memos.md");
      const fp = parseMemos(fresh.content);
      const blocks = [...fp.blocks];
      blocks.splice(Math.min(at, blocks.length), 0, block);
      return api.putFile("memos.md", serializeMemos(fp, blocks), fresh.doc.mtime);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["file", "memos.md"] }),
  });

  if (memo.isPending) {
    return (
      <Card>
        <CardBody>
          <Skeleton className="h-40 w-full" />
        </CardBody>
      </Card>
    );
  }
  if (memo.isError) {
    return (
      <Card>
        <CardBody>
          <ErrorState error={memo.error} onRetry={() => memo.refetch()} />
        </CardBody>
      </Card>
    );
  }

  const p = parseMemos(memo.data.content);
  const conflict = save.error instanceof ApiError && save.error.status === 409;

  const add = () => {
    if (!quick.trim()) return;
    save.mutate([...p.blocks, quick.trim()]);
  };
  const remove = (i: number) => {
    const block = p.blocks[i];
    save.mutate(
      p.blocks.filter((_, idx) => idx !== i),
      {
        onSuccess: () =>
          toast({
            title: "メモを削除しました",
            desc: block.length > 40 ? `${block.slice(0, 40)}…` : block,
            durationMs: 8000,
            action: { label: "元に戻す", onClick: () => undoDelete.mutate({ block, at: i }) },
          }),
      },
    );
  };
  const commitEdit = () => {
    if (editing === null) return;
    const next = p.blocks.map((b, idx) => (idx === editing ? editText.trim() : b)).filter(Boolean);
    save.mutate(next);
  };

  return (
    <div className="ds-section">
      {/* クイック追加 */}
      <Card>
        <CardBody className="flex gap-2">
          <label htmlFor="memo-quick" className="sr-only">
            メモを追加
          </label>
          <input
            id="memo-quick"
            className="ds-control flex-1 rounded-md border border-border bg-card px-3 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            placeholder="思いついたことをメモ…"
            value={quick}
            onChange={(e) => setQuick(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && !e.nativeEvent.isComposing && add()}
          />
          <Button variant="primary" loading={save.isPending && !!quick} disabled={!quick.trim()} onClick={add}>
            <Plus size={16} />
            追加
          </Button>
        </CardBody>
      </Card>

      {conflict && (
        <Card>
          <CardBody>
            <ErrorState
              error={new ApiError(409, "他のクライアントが先に保存しました。再読込してください。")}
              onRetry={() => {
                save.reset();
                memo.refetch();
              }}
            />
          </CardBody>
        </Card>
      )}

      {/* ピン留め */}
      {p.pinned.map((text, i) => (
        <Card key={`pin-${i}`} className="border-brand/30 bg-brand-muted/20">
          <CardBody className="flex items-start gap-2">
            <Pin size={14} className="text-brand mt-0.5 shrink-0" />
            <p className="text-sm whitespace-pre-wrap flex-1">{text.replace(/^-\s+/, "")}</p>
          </CardBody>
        </Card>
      ))}

      {/* Scratch カード */}
      {p.blocks.length === 0 && p.pinned.length === 0 ? (
        <Card>
          <CardBody>
            <Empty icon={<StickyNote />} title="メモはまだありません" desc="上の入力欄から最初のメモを追加できます。" />
          </CardBody>
        </Card>
      ) : (
        p.blocks.map((text, i) => (
          <Card key={`${i}-${text.slice(0, 12)}`}>
            <CardBody>
              {editing === i ? (
                <div className="ds-stack">
                  <textarea
                    className="w-full min-h-[5rem] resize-y rounded-md border border-border bg-card p-2 text-sm leading-relaxed focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    value={editText}
                    autoFocus
                    onChange={(e) => setEditText(e.target.value)}
                    aria-label="メモを編集"
                  />
                  <div className="ds-inline flex justify-end gap-2">
                    <Button variant="ghost" size="sm" onClick={() => setEditing(null)}>
                      <X size={14} />
                      取消
                    </Button>
                    <Button variant="primary" size="sm" loading={save.isPending} onClick={commitEdit}>
                      <Check size={14} />
                      保存
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="flex items-start gap-2 group">
                  <p className="text-sm whitespace-pre-wrap flex-1 leading-relaxed">{text}</p>
                  <div className="ds-inline shrink-0 opacity-0 group-hover:opacity-100 transition-opacity duration-fast">
                    <Button
                      variant="ghost"
                      size="sm"
                      iconOnly
                      aria-label="編集"
                      onClick={() => {
                        setEditing(i);
                        setEditText(text);
                      }}
                    >
                      <Pencil size={14} />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      iconOnly
                      aria-label="削除"
                      disabled={save.isPending}
                      onClick={() => remove(i)}
                    >
                      <Trash2 size={14} />
                    </Button>
                  </div>
                </div>
              )}
            </CardBody>
          </Card>
        ))
      )}
    </div>
  );
}
