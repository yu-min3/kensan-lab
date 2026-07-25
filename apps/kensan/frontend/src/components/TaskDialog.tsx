import { useEffect, useState } from "react";
import clsx from "clsx";
import { X, Trash2 } from "lucide-react";
import { type Band, type Task, type TaskSaveInput } from "../lib/api";
import { Button } from "./ui/button";

// タスクの作成・編集 共通ダイアログ。本文・プロジェクト・バンド・期限・マイルストーンを編集。
// project を変えると（編集時）ファイル間移動になる。優先度 @p はドラッグ管理なのでここには出さない。

export interface TaskDraft {
  file?: string;
  line?: number;
  text?: string;
  project: string;
  display: string;
  band: Band;
  due: string;
  milestone: string;
}

// タスクのバンドタグから draft のバンドを導く（today < week < month < later）。
export function bandOfTask(t: Task): Band {
  if (t.today) return "today";
  if (t.week) return "week";
  if (t.month) return "month";
  return "later";
}

export function taskToDraft(t: Task): TaskDraft {
  return {
    file: t.file,
    line: t.line,
    text: t.text,
    project: t.project ?? "",
    display: t.display,
    band: bandOfTask(t),
    due: t.due ?? "",
    milestone: t.milestone ?? "",
  };
}

export const BAND_LABELS: { value: Band; label: string }[] = [
  { value: "today", label: "今日" },
  { value: "week", label: "今週" },
  { value: "month", label: "今月" },
  { value: "later", label: "中期" },
];

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
      band: d.band,
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

            <Field label="いつやる（バンド）">
              <div className="inline-flex rounded-md border border-border overflow-hidden">
                {BAND_LABELS.map((b) => (
                  <button
                    key={b.value}
                    type="button"
                    className={clsx(
                      "px-3 h-9 text-sm border-l border-border first:border-l-0 transition-colors",
                      d.band === b.value ? "bg-brand text-brand-foreground" : "hover:bg-accent/60",
                    )}
                    aria-pressed={d.band === b.value}
                    onClick={() => set("band", b.value)}
                  >
                    {b.label}
                  </button>
                ))}
              </div>
            </Field>
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
