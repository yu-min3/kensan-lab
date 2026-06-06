import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Inbox, BookOpen, ArrowRight } from "lucide-react";
import { api, todayISO, type Task } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardHead, CardBody } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Empty, ErrorState, SkeletonRows, Skeleton } from "../components/ui/states";

const WEEKDAYS = ["日", "月", "火", "水", "木", "金", "土"];

export function Dashboard() {
  const today = new Date();
  return (
    <>
      <PageHeader
        eyebrow={`今日 · ${todayISO()}`}
        title={`${today.getMonth() + 1}月${today.getDate()}日（${WEEKDAYS[today.getDay()]}）`}
        sub="今日やることと直近の記録。タスクの配置は /morning が、振り返りは /reflection が行う。"
      />
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <TodayTasks />
        <div className="ds-section">
          <RecentDailies />
          <MemoPeek />
        </div>
      </div>
    </>
  );
}

function TodayTasks() {
  const qc = useQueryClient();
  const board = useQuery({ queryKey: ["board"], queryFn: api.board });
  const toggle = useMutation({
    mutationFn: ({ task, state }: { task: Task; state: Task["state"] }) =>
      api.setTaskState(task, state),
    onSettled: () => qc.invalidateQueries({ queryKey: ["board"] }),
  });

  const tasks = board.data?.today ?? [];
  const done = tasks.filter((t) => t.state === "done").length;

  return (
    <Card>
      <CardHead
        title="今日やる"
        sub="todo.md · Now"
        badge={
          tasks.length > 0 ? (
            <Badge variant={done === tasks.length ? "success" : "brand"}>
              <span className="font-mono tnum">
                {done}/{tasks.length}
              </span>
            </Badge>
          ) : undefined
        }
      />
      <CardBody>
        {board.isPending ? (
          <SkeletonRows rows={5} />
        ) : board.isError ? (
          <ErrorState error={board.error} onRetry={() => board.refetch()} />
        ) : tasks.length === 0 ? (
          <Empty
            icon={<Inbox />}
            title="今日のタスクがまだありません"
            desc="Claude Code で /morning を実行すると、ルーティンとプロジェクトのタスクがここに配置されます。"
          />
        ) : (
          <ul className="ds-stack !gap-0">
            {tasks.map((t) => (
              <li key={`${t.file}:${t.line}`} className="ds-row flex items-center gap-3 border-b border-border last:border-b-0">
                <input
                  type="checkbox"
                  className="size-4 accent-[hsl(var(--brand))]"
                  checked={t.state === "done"}
                  disabled={toggle.isPending}
                  onChange={() =>
                    toggle.mutate({ task: t, state: t.state === "done" ? "todo" : "done" })
                  }
                  aria-label={`${t.text} を${t.state === "done" ? "未完了" : "完了"}にする`}
                />
                <span
                  className={
                    t.state === "done" ? "text-sm line-through text-muted-foreground" : "text-sm"
                  }
                >
                  {t.text}
                </span>
              </li>
            ))}
          </ul>
        )}
      </CardBody>
    </Card>
  );
}

function RecentDailies() {
  const dailies = useQuery({ queryKey: ["dailies", 5], queryFn: () => api.dailyList(5) });
  return (
    <Card>
      <CardHead title="直近の日記" sub="daily/" />
      <CardBody>
        {dailies.isPending ? (
          <SkeletonRows rows={3} />
        ) : dailies.isError ? (
          <ErrorState error={dailies.error} onRetry={() => dailies.refetch()} />
        ) : dailies.data.files.length === 0 ? (
          <Empty
            icon={<BookOpen />}
            title="日記がまだありません"
            desc="今日の日記を書き始めましょう。日記ページから作成できます。"
            actions={
              <Link to="/daily">
                <Button variant="primary" size="sm">今日の日記を書く</Button>
              </Link>
            }
          />
        ) : (
          <ul className="ds-stack !gap-0">
            {dailies.data.files.map((d) => {
              const date = d.path.replace("daily/", "").replace(".md", "").replaceAll("/", "-");
              return (
                <li key={d.path} className="ds-row flex items-center justify-between border-b border-border last:border-b-0">
                  <Link
                    to={`/daily?date=${date}`}
                    className="text-sm hover:text-brand flex items-center gap-2"
                  >
                    <span className="font-mono tnum text-xs text-muted-foreground">{date}</span>
                  </Link>
                  <ArrowRight size={14} className="text-muted-foreground" />
                </li>
              );
            })}
          </ul>
        )}
      </CardBody>
    </Card>
  );
}

function MemoPeek() {
  const memo = useQuery({ queryKey: ["file", "memos.md"], queryFn: () => api.file("memos.md") });
  const scratch =
    memo.data?.content.split(/^## Scratch$/m)[1]?.trim().split("\n").slice(0, 6) ?? [];
  return (
    <Card>
      <CardHead title="メモ" sub="memos.md · Scratch" />
      <CardBody>
        {memo.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : memo.isError ? (
          <ErrorState error={memo.error} onRetry={() => memo.refetch()} />
        ) : (
          <>
            <pre className="text-sm whitespace-pre-wrap font-sans text-muted-foreground">
              {scratch.join("\n") || "（Scratch は空）"}
            </pre>
            <Link to="/memos" className="text-sm text-brand hover:underline mt-2 inline-block">
              メモを開く →
            </Link>
          </>
        )}
      </CardBody>
    </Card>
  );
}
