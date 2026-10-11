import { useEffect, useState, type ReactNode } from "react";
import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  Sun,
  BookOpen,
  StickyNote,
  SquareKanban,
  ChartSpline,
  FolderKanban,
  Sparkles,
  Menu,
  X,
  Newspaper,
  Monitor,
  Moon,
} from "lucide-react";
import clsx from "clsx";
import { FloatingMemoButton } from "./FloatingMemoButton";
import { readTheme, saveTheme, watchSystem, type Theme } from "../lib/theme";

// patterns.md 00. App Shell — Variant A: Side Nav + Main（kensan 既定）
// active は bg-accent + text-accent-foreground（border の左帯は使わない）
// モバイル（< md）はオーバーレイ表示（hamburger → drawer）。ナビ選択で自動的に閉じる。

const nav = [
  {
    group: "今日",
    items: [
      { to: "/", label: "ダッシュボード", icon: LayoutDashboard },
      { to: "/today", label: "今日", icon: Sun },
      { to: "/tasks", label: "タスク", icon: SquareKanban },
      { to: "/projects", label: "プロジェクト", icon: FolderKanban },
      { to: "/feed", label: "フィード", icon: Newspaper },
    ],
  },
  {
    group: "記録",
    items: [
      { to: "/daily", label: "日記", icon: BookOpen },
      { to: "/memos", label: "メモ", icon: StickyNote },
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
        data-testid="app-sidebar"
        className={clsx(
          "border-r border-border flex-col bg-background",
          // モバイル: 左からのオーバーレイ drawer / md 以上: grid の 1 列目に常設。
          // 本文が長くても footer の切替が見えるよう、画面の高さで止める
          "fixed inset-y-0 left-0 z-50 w-[200px] md:sticky md:top-0 md:h-screen md:self-start md:z-auto",
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
        <nav className="flex-1 min-h-0 overflow-y-auto px-2 ds-stack !gap-4">
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
          <ThemeSwitch />
        </footer>
      </aside>
      {/* md 以上では 2 列目に明示配置する。暗黙の auto-placement に任せると、
          aside が（拡張機能・CSS 事故などで）フローから外れた瞬間に main が
          200px の 1 列目へ繰り上がり、全ページが判読不能に潰れる。 */}
      <main className="ds-page min-w-0 overflow-x-hidden md:col-start-2 md:row-start-1">
        {children}
      </main>
      <FloatingMemoButton />
    </div>
  );
}

const THEMES: { id: Theme; label: string; icon: typeof Sun }[] = [
  { id: "system", label: "自動（OS に合わせる）", icon: Monitor },
  { id: "light", label: "明るい表示", icon: Sun },
  { id: "dark", label: "暗い表示", icon: Moon },
];

function ThemeSwitch() {
  const [theme, setTheme] = useState<Theme>(readTheme);
  useEffect(() => watchSystem(theme), [theme]);
  return (
    <div role="radiogroup" aria-label="表示の明るさ" className="ml-auto flex rounded-md border border-border p-0.5">
      {THEMES.map((t) => (
        <button
          key={t.id}
          type="button"
          role="radio"
          aria-checked={theme === t.id}
          aria-label={t.label}
          title={t.label}
          onClick={() => {
            saveTheme(t.id);
            setTheme(t.id);
          }}
          className={clsx(
            "size-7 rounded flex items-center justify-center",
            theme === t.id ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:text-foreground",
          )}
        >
          <t.icon size={14} />
        </button>
      ))}
    </div>
  );
}
