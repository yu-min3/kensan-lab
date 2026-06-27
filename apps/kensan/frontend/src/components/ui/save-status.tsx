import { Check, Loader2, AlertTriangle } from "lucide-react";
import clsx from "clsx";

// 自動保存の状態を一目で伝える小さなインジケータ。日記・ホワイトボード等の
// 自動保存 UI で共有する。アイコン付きで「ちゃんと保存されたか」を分かりやすく。
export type SaveState = "idle" | "dirty" | "saving" | "saved" | "error" | "conflict";

const MAP: Record<Exclude<SaveState, "idle">, { text: string; bad: boolean; icon: React.ReactNode }> = {
  dirty: { text: "未保存…", bad: false, icon: null },
  saving: { text: "保存中…", bad: false, icon: <Loader2 size={12} className="animate-spin" /> },
  saved: { text: "保存済", bad: false, icon: <Check size={12} /> },
  error: { text: "保存エラー", bad: true, icon: <AlertTriangle size={12} /> },
  conflict: { text: "競合 — 再読込が必要", bad: true, icon: <AlertTriangle size={12} /> },
};

// idle のときは idleText（既定: 何も出さない）。それ以外は状態に応じたラベルを出す。
export function SaveStatus({ state, idleText }: { state: SaveState; idleText?: string }) {
  if (state === "idle") {
    return idleText ? <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">{idleText}</span> : null;
  }
  const s = MAP[state];
  return (
    <span className={clsx("inline-flex items-center gap-1 text-xs", s.bad ? "text-destructive" : "text-muted-foreground")}>
      {s.icon}
      {s.text}
    </span>
  );
}
