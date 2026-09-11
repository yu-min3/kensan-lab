import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Compass } from "lucide-react";
import { api, type Task } from "../lib/api";
import { useWorkspaceRefresh } from "../hooks/useWorkspaceRefresh";
import { PageHeader } from "../components/PageHeader";
import { DiaryButton } from "../components/DiaryCta";
import { MemoSummary } from "../components/MemoSummary";
import { Whiteboard } from "../components/Whiteboard";
import { GoalCard, TaskCheck } from "../components/today/GoalCard";
import { ActivityRail } from "../components/today/ActivityRail";
import { TriageCard, TaskRecovery } from "../components/today/TriageCard";
import { projectGroups, taskKey } from "../components/today/model";
import { Card, CardHead, CardBody } from "../components/ui/card";
import { Empty, ErrorState, SkeletonRows } from "../components/ui/states";

export function Dashboard() {
  const refresh = useWorkspaceRefresh();
  const data = useQuery({
    queryKey: ["today"],
    queryFn: api.today,
    refetchInterval: 60_000,
  });
  const [scratchOpen, setScratchOpen] = useState(false);
  const change = useMutation({
    mutationFn: (operation: () => Promise<unknown>) => operation(),
    onSettled: refresh,
  });
  if (data.isPending) return <SkeletonRows rows={6} />;
  if (data.isError)
    return <ErrorState error={data.error} onRetry={() => data.refetch()} />;

  const view = data.data;
  const groups = projectGroups(view);
  const today = view.board.today ?? [];
  const standalone = today.filter((t) => !t.project && t.state !== "skipped");
  const candidates = [
    ...(view.board.week ?? []),
    ...(view.board.month ?? []),
    ...(view.board.later ?? []),
  ];
  const weekday = new Date(`${view.date}T12:00:00+09:00`).toLocaleDateString(
    "ja-JP",
    {
      month: "long",
      day: "numeric",
      weekday: "short",
      timeZone: "Asia/Tokyo",
    },
  );
  const setState = (t: Task, state: Task["state"]) =>
    change.mutate(() => api.setTaskState(t, state));

  return (
    <div className="ds-section">
      <PageHeader
        eyebrow={`今日 · ${view.date}`}
        title={weekday}
        sub="目標の下に、今日の一歩を。"
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <Link to="/tasks" className="text-sm text-brand">
              全タスクを開く →
            </Link>
            <DiaryButton date={view.date} />
          </div>
        }
      />
      <div className="flex items-start gap-3 text-sm text-muted-foreground">
        <Compass size={18} className="text-brand shrink-0 mt-0.5" />
        <p>
          {view.goals.northStar ||
            "goals.md に North Star を置くと、ここに表示されます。"}
        </p>
      </div>
      {change.isError && (
        <ErrorState
          error={change.error}
          onRetry={() => {
            change.reset();
            data.refetch();
          }}
        />
      )}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
        <div className="lg:col-span-2 ds-section min-w-0">
          {groups.focus.length + groups.other.length === 0 && (
            <Card>
              <CardBody>
                <Empty
                  icon={<Compass />}
                  title="目標をひとつ置いてみましょう"
                  desc="プロジェクトに目標とタスクを登録すると、ここで今日の一歩を選べます。"
                  actions={
                    <Link
                      to="/projects"
                      className="ds-control inline-flex items-center rounded-lg border border-border-strong px-3 text-sm"
                    >
                      プロジェクトを作る
                    </Link>
                  }
                />
              </CardBody>
            </Card>
          )}
          {[
            { title: "North Star へ", items: groups.focus },
            { title: "生活・その他の取り組み", items: groups.other },
          ].map(
            (group) =>
              group.items.length > 0 && (
                <section key={group.title} className="ds-section">
                  <h2 className="h-serif text-base font-semibold flex items-center gap-3">
                    {group.title}
                    <span className="h-px flex-1 bg-border" />
                  </h2>
                  {group.items.map((p) => (
                    <GoalCard
                      key={p.name}
                      project={p}
                      date={view.date}
                      forecast={view.forecasts[p.name]}
                      phase={view.phases[p.name]}
                      tasks={today.filter((t) => t.project === p.name)}
                      candidates={candidates.filter(
                        (t) => t.project === p.name,
                      )}
                      routines={view.routines.filter(
                        (r) => r.project === p.name,
                      )}
                      busy={change.isPending}
                      onState={setState}
                      onChoose={(t) =>
                        change.mutate(() => api.setBand(t, "today"))
                      }
                      onRoutine={(r) =>
                        change.mutate(() =>
                          api.routineState(r, view.date, !r.doneToday),
                        )
                      }
                    />
                  ))}
                </section>
              ),
          )}
          {standalone.length > 0 && (
            <Card>
              <CardHead
                title="プロジェクト外の今日の一歩"
                sub="従来の今日ボードの項目もここに残ります"
              />
              <CardBody>
                {standalone.map((t) => (
                  <TaskCheck
                    key={taskKey(t)}
                    task={t}
                    date={view.date}
                    busy={change.isPending}
                    onState={setState}
                  />
                ))}
              </CardBody>
            </Card>
          )}
          <TriageCard
            task={view.triage}
            count={(view.board.later ?? []).length}
            reviewed={view.deferredToday}
            busy={change.isPending}
            onChoose={(t, action) =>
              change.mutate(() => api.triageTask(t, action))
            }
          />
          <TaskRecovery
            tasks={view.skipped}
            busy={change.isPending}
            onRestore={(t) => setState(t, "todo")}
          />
          <details onToggle={(e) => setScratchOpen(e.currentTarget.open)}>
            <summary className="text-sm text-muted-foreground cursor-pointer">
              思考の置き場 · メモ／ホワイトボード
            </summary>
            {scratchOpen && (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mt-4">
                <MemoSummary />
                <Whiteboard />
              </div>
            )}
          </details>
        </div>
        <ActivityRail view={view} />
      </div>
    </div>
  );
}
