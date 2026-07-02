import { useEffect, useState } from "react";
import { X, Trash2 } from "lucide-react";
import { type Task, type TaskSaveInput } from "../lib/api";
import { Button } from "./ui/button";

// タスクの作成・編集 共通ダイアログ。本文・プロジェクト・今日やる・期限・マイルストーンを編集。
// project を変えると（編集時）ファイル間移動になる。優先度 @p はドラッグ管理なのでここには出さない。

export interface TaskDraft {
  file?: string;
  line?: number;
  text?: string;
  project: string;
  display: string;
  today: boolean;
  due: string;
  milestone: string;
}

export function taskToDraft(t: Task): TaskDraft {
  return {
    file: t.file,
    line: t.line,
    text: t.text,
    project: t.project ?? "",
    display: t.display,
    today: t.today,
    due: t.due ?? "",
    milestone: t.milestone ?? "",
  };
}

export function TaskDialog({
  mode,
  initial,
  projects,
  milestonesByProject,
  busy,
  onSave,
  onDelete,
  onClose,
}: {
  mode: "create" | "edit";
  initial: TaskDraft;
  projects: string[];
  milestonesByProject: Record<string, string[]>;
  busy: boolean;
  onSave: (input: TaskSaveInput) => void;
  onDelete?: () => void;
  onClose: () => void;
}) {
  const [d, setD] = useState<TaskDraft>(initial);
  useEffect(() => setD(initial), [initial]);

  const set = <K extends keyof TaskDraft>(k: K, v: TaskDraft[K]) => setD((p) => ({ ...p, [k]: v }));
  const canSave = d.display.trim().length > 0 && !busy;

  // 選択中プロジェクトの定義済みマイルストーン（## マイルストーン）。自由入力させず散らかりを防ぐ。
  // 既存の @ms が一覧に無い場合（旧・自由入力分）は値を消さないよう先頭に残す。
  const msList = milestonesByProject[d.project] ?? [];
  const msOptions = d.milestone && !msList.includes(d.milestone) ? [d.milestone, ...msList] : msList;

  const submit = () => {
    if (!canSave) return;
    onSave({
      file: d.file,
      line: d.line,
      text: d.text,
      project: d.project,
      display: d.display.trim(),
      today: d.today,
      due: d.due,
      milestone: d.milestone.trim(),
    });
  };

  return (
    <>
      <div className="fixed inset-0 bg-black/30 z-50" onClick={onClose} />
      <div className="fixed inset-0 z-50 grid place-items-center p-4 pointer-events-none">
        <div
          className="bg-card border border-border rounded-lg shadow-lg w-full max-w-md pointer-events-auto"
          onKeyDown={(e) => {
            if (e.key === "Escape") onClose();
            if ((e.metaKey || e.ctrlKey) && e.key === "Enter") submit();
          }}
        >
          {/* ヘッダ */}
          <div className="flex items-center justify-between p-4 border-b border-border">
            <h3 className="h-serif text-base font-semibold">{mode === "create" ? "タスクを追加" : "タスクを編集"}</h3>
            <button
              className="size-7 grid place-items-center rounded-md hover:bg-accent/60 text-muted-foreground"
              onClick={onClose}
              aria-label="閉じる"
            >
              <X size={16} />
            </button>
          </div>

          {/* フォーム */}
          <div className="p-4 ds-stack !gap-3">
            <Field label="本文">
              <textarea
                autoFocus
                rows={2}
                className="w-full resize-y rounded-md border border-border bg-card px-3 py-1.5 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={d.display}
                placeholder="やること…"
                onChange={(e) => set("display", e.target.value)}
              />
            </Field>

            <div className="grid grid-cols-2 gap-3">
              <Field label="プロジェクト">
                <select
                  className="w-full rounded-md border border-border bg-card px-2 h-9 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  value={d.project}
                  onChange={(e) => {
                    const project = e.target.value;
                    // プロジェクトを変えたら、新 project に無いマイルストーンは外す
                    setD((p) => ({
                      ...p,
                      project,
                      milestone: (milestonesByProject[project] ?? []).includes(p.milestone) ? p.milestone : "",
                    }));
                  }}
                >
                  <option value="">（なし・今日やる/todo.md）</option>
                  {projects.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="期限">
                <input
                  type="date"
                  className="w-full rounded-md border border-border bg-card px-2 h-9 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  value={d.due}
                  onChange={(e) => set("due", e.target.value)}
                />
              </Field>
            </div>

            <Field label="マイルストーン（任意）">
              <select
                className="w-full rounded-md border border-border bg-card px-2 h-9 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
                value={d.milestone}
                disabled={!d.project}
                onChange={(e) => set("milestone", e.target.value)}
              >
                <option value="">（なし）</option>
                {msOptions.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
              {d.project && msList.length === 0 && (
                <p className="text-xs text-muted-foreground mt-1">
                  このプロジェクトの <code>## マイルストーン</code> に定義がありません。
                </p>
              )}
            </Field>

            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                className="size-4 accent-[hsl(var(--brand))]"
                checked={d.today}
                onChange={(e) => set("today", e.target.checked)}
              />
              今日やる（@today）
            </label>
          </div>

          {/* フッタ */}
          <div className="flex items-center justify-between p-4 border-t border-border">
            {mode === "edit" && onDelete ? (
              <Button variant="ghost" size="sm" disabled={busy} onClick={onDelete}>
                <Trash2 size={14} />
                削除
              </Button>
            ) : (
              <span />
            )}
            <div className="ds-inline flex gap-2">
              <Button variant="ghost" size="sm" onClick={onClose}>
                キャンセル
              </Button>
              <Button variant="primary" size="sm" disabled={!canSave} loading={busy} onClick={submit}>
                {mode === "create" ? "追加" : "保存"}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div className="mt-1">{children}</div>
    </label>
  );
}
