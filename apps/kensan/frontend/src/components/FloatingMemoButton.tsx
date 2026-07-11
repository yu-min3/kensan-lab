import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Lightbulb, X, Send, Loader2, Trash2 } from "lucide-react";
import clsx from "clsx";
import { api, ApiError } from "../lib/api";
import { parseMemos, serializeMemos } from "../lib/memosFile";

// 旧 kensan のメモ体験 — 右下の丸ボタンから、作成と一覧（編集・削除）をその場で行う。
// 実体は memos.md の ## Scratch（空行区切りブロック = 1 メモ）。
// ダッシュボードを汚さず、どの画面からでも素早く落とせるグローバル FAB。
export function FloatingMemoButton() {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [content, setContent] = useState("");
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editText, setEditText] = useState("");
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const memo = useQuery({
    queryKey: ["file", "memos.md"],
    queryFn: () => api.file("memos.md"),
    enabled: open,
  });

  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  const save = useMutation({
    mutationFn: (blocks: string[]) => {
      const p = parseMemos(memo.data!.content);
      return api.putFile("memos.md", serializeMemos(p, blocks), memo.data!.doc.mtime);
    },
    onSuccess: () => {
      setContent("");
      setEditingIndex(null);
      qc.invalidateQueries({ queryKey: ["file", "memos.md"] });
    },
  });

  const blocks = memo.data ? parseMemos(memo.data.content).blocks : [];
  const pinned = memo.data ? parseMemos(memo.data.content).pinned : [];
  const conflict = save.error instanceof ApiError && save.error.status === 409;

  const addMemo = () => {
    if (!content.trim() || save.isPending) return;
    save.mutate([...blocks, content.trim()]);
  };
  const removeMemo = (i: number) => save.mutate(blocks.filter((_, idx) => idx !== i));
  const commitEdit = () => {
    if (editingIndex === null) return;
    const next = blocks.map((b, idx) => (idx === editingIndex ? editText.trim() : b)).filter(Boolean);
    save.mutate(next);
  };

  // Ctrl/Cmd+Enter で保存、Escape で入力クリア → 閉じる
  const onNewKey = (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") addMemo();
    else if (e.key === "Escape") (content.trim() ? setContent("") : setOpen(false));
  };

  return (
    <>
      {open && <div className="fixed inset-0 bg-black/20 z-40" onClick={() => setOpen(false)} />}

      {/* パネル */}
      <div
        className={clsx(
          "fixed bottom-24 right-6 z-50 transition-all duration-fast ease-out",
          open ? "opacity-100 translate-y-0" : "opacity-0 translate-y-4 pointer-events-none",
        )}
      >
        <div className="bg-card border border-border rounded-lg shadow-lg w-96 max-h-[70vh] flex flex-col">
          {/* ヘッダ */}
          <div className="flex items-center justify-between p-3 border-b border-border">
            <div className="flex items-center gap-2 text-sm font-medium h-serif">
              <Lightbulb className="size-4 text-brand" />
              メモ
            </div>
            <button
              className="size-7 grid place-items-center rounded-md hover:bg-accent/60 text-muted-foreground"
              onClick={() => setOpen(false)}
              aria-label="閉じる"
            >
              <X className="size-4" />
            </button>
          </div>

          {/* 一覧 */}
          <div className="flex-1 overflow-y-auto max-h-[44vh]">
            {memo.isPending ? (
              <div className="p-6 text-center text-sm text-muted-foreground">
                <Loader2 className="size-5 animate-spin mx-auto mb-2" />
                読み込み中…
              </div>
            ) : conflict ? (
              <div className="p-4 text-sm text-destructive">
                他のクライアントが先に保存しました。
                <button
                  className="ml-1 underline"
                  onClick={() => {
                    save.reset();
                    memo.refetch();
                  }}
                >
                  再読込
                </button>
              </div>
            ) : blocks.length === 0 && pinned.length === 0 ? (
              <div className="p-6 text-center">
                <Lightbulb className="size-8 text-muted-foreground/50 mx-auto mb-2" />
                <p className="text-sm text-muted-foreground">メモがありません</p>
                <p className="text-xs text-muted-foreground mt-1">下の入力欄から追加</p>
              </div>
            ) : (
              <div className="divide-y divide-border">
                {pinned.map((text, i) => (
                  <div key={`pin-${i}`} className="p-3 flex items-start gap-2 bg-brand-muted/20">
                    <Lightbulb className="size-3.5 text-brand mt-0.5 shrink-0" />
                    <p className="text-sm whitespace-pre-wrap break-words flex-1">{text.replace(/^-\s+/, "")}</p>
                  </div>
                ))}
                {blocks.map((text, i) => (
                  <div key={`${i}-${text.slice(0, 12)}`} className="p-3 hover:bg-accent/40 transition-colors group">
                    {editingIndex === i ? (
                      <textarea
                        name="quick-memo-edit"
                        className="w-full min-h-[60px] resize-none rounded-md border border-border bg-card p-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        value={editText}
                        autoFocus
                        disabled={save.isPending}
                        onChange={(e) => setEditText(e.target.value)}
                        onBlur={commitEdit}
                        onKeyDown={(e) => {
                          if ((e.metaKey || e.ctrlKey) && e.key === "Enter") commitEdit();
                          else if (e.key === "Escape") setEditingIndex(null);
                        }}
                        aria-label="メモを編集"
                      />
                    ) : (
                      <p
                        className="text-sm whitespace-pre-wrap break-words cursor-text hover:bg-accent/40 rounded px-1 -mx-1"
                        onClick={() => {
                          setEditingIndex(i);
                          setEditText(text);
                        }}
                      >
                        {text}
                      </p>
                    )}
                    <div className="flex items-center justify-end mt-1">
                      <button
                        className={clsx(
                          "size-6 grid place-items-center rounded-md text-destructive hover:bg-destructive/10",
                          "opacity-0 group-hover:opacity-100 transition-opacity duration-fast",
                          editingIndex === i && "!opacity-0",
                        )}
                        disabled={save.isPending}
                        onClick={() => removeMemo(i)}
                        title="削除"
                        aria-label="削除"
                      >
                        <Trash2 className="size-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* 入力欄（常時下部） */}
          <div className="border-t border-border p-3 bg-accent/20">
            <textarea
              ref={inputRef}
              id="quick-memo-input"
              name="quick-memo-input"
              aria-label="新しいメモを入力"
              className="w-full min-h-[60px] max-h-[100px] resize-none rounded-md border border-border bg-card p-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              onKeyDown={onNewKey}
              placeholder="新しいメモを入力…"
              rows={2}
            />
            <div className="flex items-center justify-between mt-2">
              <span className="text-xs text-muted-foreground">⌘/Ctrl+Enter で保存</span>
              <button
                className={clsx(
                  "inline-flex items-center gap-1 rounded-md px-3 h-8 text-sm font-medium",
                  "bg-[hsl(var(--brand))] text-[hsl(var(--brand-foreground))]",
                  "disabled:opacity-50 disabled:pointer-events-none",
                )}
                onClick={addMemo}
                disabled={!content.trim() || save.isPending}
              >
                {save.isPending && !!content ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
                保存
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* FAB 本体 */}
      <button
        onClick={() => setOpen(!open)}
        className={clsx(
          "fixed bottom-6 right-6 z-50 size-14 rounded-full shadow-lg grid place-items-center",
          "bg-[hsl(var(--brand))] text-[hsl(var(--brand-foreground))] transition-transform duration-fast",
          "hover:scale-105",
          open && "rotate-45",
        )}
        aria-label={open ? "メモを閉じる" : "メモを開く"}
        title={open ? "メモを閉じる" : "メモ — どの画面からでも素早く書き留める"}
      >
        {open ? <X className="size-6" /> : <Lightbulb className="size-6" />}
      </button>
    </>
  );
}
