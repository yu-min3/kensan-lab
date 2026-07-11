import { useState, type ReactNode } from "react";
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
  Menu,
  X,
} from "lucide-react";
import clsx from "clsx";
import { FloatingMemoButton } from "./FloatingMemoButton";

// patterns.md 00. App Shell — Variant A: Side Nav + Main（kensan 既定）
// active は bg-accent + text-accent-foreground（border の左帯は使わない）
// モバイル（< md）はオーバーレイ表示（hamburger → drawer）。ナビ選択で自動的に閉じる。

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
  const [navOpen, setNavOpen] = useState(false);
  const close = () => setNavOpen(false);

  return (
    <div className="min-h-screen md:grid md:grid-cols-[200px_1fr]">
      {/* モバイル topbar（md 以上では sidebar が常設なので出さない） */}
      <header className="md:hidden sticky top-0 z-40 flex items-center gap-2 border-b border-border bg-background px-3 py-2">
        <button
          className="p-2 -m-1 rounded-md hover:bg-accent text-foreground"
          aria-label="メニューを開く"
          aria-expanded={navOpen}
          onClick={() => setNavOpen(true)}
        >
          <Menu size={20} />
        </button>
        <span className="h-serif text-base font-bold text-brand">研鑽</span>
        <span className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">kensan</span>
      </header>

      {/* drawer の backdrop（モバイルのみ） */}
      {navOpen && <div className="fixed inset-0 bg-black/20 z-40 md:hidden" onClick={close} />}

      <aside
        className={clsx(
          "border-r border-border flex-col bg-background",
          // モバイル: 左からのオーバーレイ drawer / md 以上: grid の 1 列目に常設
          "fixed inset-y-0 left-0 z-50 w-[200px] md:static md:z-auto",
          navOpen ? "flex" : "hidden md:flex",
        )}
      >
        <div className="px-4 py-5 flex items-start justify-between">
          <div>
            <div className="h-serif text-lg font-bold text-brand">研鑽</div>
            <div className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
              kensan
            </div>
          </div>
          <button
            className="md:hidden p-2 -m-1 rounded-md hover:bg-accent text-muted-foreground"
            aria-label="メニューを閉じる"
            onClick={close}
          >
            <X size={18} />
          </button>
        </div>
        <nav className="flex-1 px-2 ds-stack !gap-4">
          <NavLink
            to="/life"
            onClick={close}
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
                  onClick={close}
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
        </footer>
      </aside>
      <main className="ds-page min-w-0">{children}</main>
      <FloatingMemoButton />
    </div>
  );
}
