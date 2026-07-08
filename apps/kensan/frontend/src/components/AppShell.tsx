import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  BookOpen,
  StickyNote,
  SquareKanban,
  ChartSpline,
  FileText,
  FolderKanban,
  Sparkles,
  Trash2,
} from "lucide-react";
import clsx from "clsx";
import { FloatingMemoButton } from "./FloatingMemoButton";

// patterns.md 00. App Shell — Variant A: Side Nav + Main（kensan 既定）
// active は bg-accent + text-accent-foreground（border の左帯は使わない）

const nav = [
  {
    group: "今日",
    items: [
      { to: "/", label: "ダッシュボード", icon: LayoutDashboard },
      { to: "/tasks", label: "タスク", icon: SquareKanban },
      { to: "/projects", label: "プロジェクト", icon: FolderKanban },
    ],
  },
  {
    group: "記録",
    items: [
      { to: "/daily", label: "日記", icon: BookOpen },
      { to: "/memos", label: "メモ", icon: StickyNote },
      { to: "/notes", label: "ノート", icon: FileText },
    ],
  },
  {
    group: "振り返り",
    items: [{ to: "/reviews", label: "レビュー", icon: ChartSpline }],
  },
];

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="grid grid-cols-[200px_1fr] min-h-screen">
      <aside className="border-r border-border flex flex-col">
        <div className="px-4 py-5">
          <div className="h-serif text-lg font-bold text-brand">研鑽</div>
          <div className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
            kensan
          </div>
        </div>
        <nav className="flex-1 px-2 ds-stack !gap-4">
          <NavLink
            to="/life"
            className={({ isActive }) =>
              clsx(
                "ds-row flex items-center gap-2 px-2 rounded-md text-sm",
                isActive
                  ? "bg-brand-muted text-accent-foreground font-semibold"
                  : "text-brand hover:bg-accent/60 font-medium",
              )
            }
          >
            <Sparkles size={16} />
            人生でやりたいこと
          </NavLink>
          {nav.map((g) => (
            <div key={g.group}>
              <div className="px-2 pb-1 text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                {g.group}
              </div>
              {g.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === "/"}
                  className={({ isActive }) =>
                    clsx(
                      "ds-row flex items-center gap-2 px-2 rounded-md text-sm",
                      isActive
                        ? "bg-accent text-accent-foreground font-medium"
                        : "text-foreground hover:bg-accent/60",
                    )
                  }
                >
                  <item.icon size={16} />
                  {item.label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <footer className="px-4 py-4 border-t border-border flex items-center gap-2">
          <div className="size-8 rounded-full bg-brand-muted text-accent-foreground flex items-center justify-center text-xs font-bold">
            Yu
          </div>
          <span className="text-sm">Yu</span>
          {/* ゴミ箱 — 普段は目立たせない（footer の小さなアイコンのみ） */}
          <NavLink
            to="/trash"
            className={({ isActive }) =>
              clsx(
                "ml-auto size-7 grid place-items-center rounded-md transition-colors",
                isActive ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:text-foreground hover:bg-accent/60",
              )
            }
            title="ゴミ箱 — 削除したタスクの確認・復元"
            aria-label="ゴミ箱"
          >
            <Trash2 size={15} />
          </NavLink>
        </footer>
      </aside>
      <main className="ds-page min-w-0">{children}</main>
      <FloatingMemoButton />
    </div>
  );
}
