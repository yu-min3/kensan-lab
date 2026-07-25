import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronRight } from "lucide-react";
import clsx from "clsx";
import { api, type Task } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { TaskBoard } from "../components/TaskBoard";
import { Card, CardBody } from "../components/ui/card";
import { ProjectBadge } from "../components/ProjectBadge";

// タスクページ = 時間軸バンドのかんばん（今日 / 今週 / 今月 / いつか）。
// タスクは project の ## タスク に住み、行内タグ（@today/@week/@month）でバンドが決まる。
// その下にマイルストーンを折りたたみで補助表示する。
export function TasksPage() {
  return (
    <>
      <PageHeader
        eyebrow="タスク"
        title="かんばん"
        sub="レーン間ドラッグ = バンドの張り替え（今日/今週/今月/中期）。行は project ファイルのまま動かない。"
      />
      <div className="ds-section">
        <TaskBoard />
        <Backlog />
      </div>
    </>
  );
}

function Backlog() {
  const board = useQuery({ queryKey: ["board"], queryFn: api.board });
  const milestones = board.data?.milestones ?? [];
  if (milestones.length === 0) return null;
  return <Collapsible title="マイルストーン" tasks={milestones} />;
}

function Collapsible({ title, tasks }: { title: string; tasks: Task[] }) {
  const [open, setOpen] = useState(false);
  if (tasks.length === 0) return null;
  const done = tasks.filter((t) => t.state === "done").length;
  return (
    <Card>
      <button
        className="ds-card w-full flex items-center justify-between text-left"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        <span className="h-serif text-base font-semibold flex items-center gap-2">
          {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          {title}
        </span>
        <span className="font-mono tnum text-xs text-muted-foreground">
          {done}/{tasks.length}
        </span>
      </button>
      {open && (
        <CardBody className="!pt-0 ds-stack !gap-0">
          {tasks.map((t) => (
            <div
              key={`${t.file}:${t.line}`}
              className="ds-row flex items-center gap-2 border-b border-border last:border-b-0 text-sm"
            >
              <span
                className={clsx(
                  "size-1.5 rounded-full shrink-0",
                  t.state === "done" ? "bg-success" : t.state === "skipped" ? "bg-muted-foreground" : "bg-warning",
                )}
              />
              <span className={clsx("flex-1", t.state !== "todo" && "text-muted-foreground line-through")}>
                {t.display}
              </span>
              {t.project && <ProjectBadge project={t.project} />}
            </div>
          ))}
        </CardBody>
      )}
    </Card>
  );
}
