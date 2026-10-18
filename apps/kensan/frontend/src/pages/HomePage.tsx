import { useEffect, useState, type ReactNode } from "react";
import { useLocation, useSearchParams } from "react-router-dom";
import { TodayDiary } from "../components/today/TodayDiary";
import { HomeActivity } from "../components/home/HomeActivity";
import { ProjectsPage } from "./ProjectsPage";
import { TasksPage } from "./TasksPage";
import { PortfolioPage } from "./PortfolioPage";

function HomeSection({ id, title, description, children }: { id: string; title: string; description: string; children: ReactNode }) {
  const [params] = useSearchParams();
  const location = useLocation();
  const requested = params.get("panel") === id || (id === "projects" && params.has("project"));
  const [open, setOpen] = useState(requested);
  const [loaded, setLoaded] = useState(requested);
  useEffect(() => {
    if (!requested) return;
    setOpen(true);
    setLoaded(true);
    const frame = requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ block: "start" }));
    return () => cancelAnimationFrame(frame);
  }, [requested, params.get("project")]);
  useEffect(() => {
    if (location.state?.homeSection !== id) return;
    setOpen(true);
    setLoaded(true);
    const frame = requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ block: "start" }));
    return () => cancelAnimationFrame(frame);
  }, [location.key, id]);
  return (
    <details id={id} open={open} onToggle={event => {
      setOpen(event.currentTarget.open);
      if (event.currentTarget.open) setLoaded(true);
    }} className="scroll-mt-16 rounded-lg border border-border bg-card">
      <summary className="ds-card cursor-pointer focus-visible:ring-2 focus-visible:ring-ring rounded-lg">
        <span className="h-serif font-semibold">{title}</span>
        <span className="ml-3 text-xs text-muted-foreground">{description}</span>
      </summary>
      {/* Keep editors mounted after the first expansion so folding preserves drafts. */}
      {loaded && <div className="ds-card border-t border-border min-w-0">{children}</div>}
    </details>
  );
}

export function HomePage() {
  return (
    <div className="ds-section">
      <h1 className="sr-only">ダッシュボード</h1>
      <section id="diary" className="scroll-mt-16"><TodayDiary /></section>
      <HomeSection id="activity" title="記録・習慣" description="やったことを残す、続けていることを確かめる"><HomeActivity /></HomeSection>
      <HomeSection id="projects" title="プロジェクト" description="目標・現在地・節目・ログ"><ProjectsPage embedded /></HomeSection>
      <HomeSection id="tasks" title="タスク" description="今日・今週・今月・いつか"><TasksPage embedded /></HomeSection>
      <HomeSection id="portfolio" title="ポートフォリオ" description="目指す姿・地図・技術・年報"><PortfolioPage /></HomeSection>
    </div>
  );
}
