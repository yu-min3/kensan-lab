import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Compass } from "lucide-react";
import { api, type Task } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { MemoSummary } from "../components/MemoSummary";
import { Whiteboard } from "../components/Whiteboard";

import { GoalCard, TaskCheck } from "../components/today/GoalCard";
import { ActivityRail } from "../components/today/ActivityRail";
import { projectGroups, taskKey } from "../components/today/model";
import { Card, CardHead, CardBody } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { ErrorState, SkeletonRows } from "../components/ui/states";

export function Dashboard() {
  const qc = useQueryClient();
  const data = useQuery({ queryKey: ["today"], queryFn: api.today, refetchInterval: 60_000 });
  const [error, setError] = useState<Error | null>(null);
  const change = useMutation({
    mutationFn: (operation: () => Promise<unknown>) => operation(),
    onSuccess: () => setError(null),
    onError: (e: Error) => setError(e),
    onSettled: () => Promise.all(["today", "board", "projects", "project"].map(key => qc.invalidateQueries({ queryKey: [key] }))),
  });
  if (data.isPending) return <SkeletonRows rows={6} />;
  if (data.isError) return <ErrorState error={data.error} onRetry={() => data.refetch()} />;
  const view = data.data;
  const groups = projectGroups(view);
  const today = view.board.today ?? [];
  const candidates = [...(view.board.week ?? []), ...(view.board.month ?? []), ...(view.board.later ?? [])];
  const weekday = new Date(`${view.date}T12:00:00+09:00`).toLocaleDateString("ja-JP", { month: "long", day: "numeric", weekday: "short", timeZone: "Asia/Tokyo" });
  const setState = (t: Task, state: Task["state"]) => change.mutate(() => api.setTaskState(t, state));
  const triage = view.triage;
  return <div className="ds-section">
    <PageHeader eyebrow={`今日 · ${view.date}`} title={weekday} sub="目標の下に、今日の一歩を。" actions={<Link to="/tasks" className="text-sm text-brand">全タスクを開く →</Link>} />
    <div className="flex items-start gap-3 text-sm text-muted-foreground"><Compass size={18} className="text-brand shrink-0 mt-0.5" /><p>{view.goals.northStar || "North Star は目標ページで設定できます。"}</p></div>
    {error && <ErrorState error={error} onRetry={() => { setError(null); data.refetch(); }} />}
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
      <div className="lg:col-span-2 ds-section min-w-0">
        {([{ title: "North Star へ", items: groups.focus }, { title: "生活・その他の取り組み", items: groups.other }]).map(group => group.items.length > 0 && <section key={group.title} className="ds-section">
          <h2 className="h-serif text-base font-semibold flex items-center gap-3">{group.title}<span className="h-px flex-1 bg-border" /></h2>
          {group.items.map(p => <GoalCard key={p.name} project={p} date={view.date} forecast={view.forecasts[p.name]} phase={view.phases[p.name]} tasks={today.filter(t => t.project === p.name)} candidates={candidates.filter(t => t.project === p.name)} routines={view.routines.filter(r => r.project === p.name)} busy={change.isPending} onState={setState} onChoose={t => change.mutate(() => api.setBand(t, "today"))} onRoutine={r => change.mutate(() => api.routineState(r, view.date, !r.doneToday))} />)}
        </section>)}
        {today.some(t => !t.project && t.state !== "skipped") && <Card><CardHead title="プロジェクト外の今日の一歩" sub="従来の今日ボードの項目もここに残ります" /><CardBody>{today.filter(t => !t.project && t.state !== "skipped").map(t => <TaskCheck key={taskKey(t)} task={t} date={view.date} busy={change.isPending} onState={setState} />)}</CardBody></Card>}
        <Card><CardHead title="溜まっているものを、ひとつだけ" sub={`${(view.board.later ?? []).length}件 · 未確認を先に、確認したものは後ろへ`} /><CardBody className="ds-stack">
          {triage ? <><p className="h-serif text-lg font-semibold">{triage.display}</p><p className="text-xs text-muted-foreground">{triage.project} · いつか</p><div className="flex flex-wrap gap-2"><Button variant="primary" disabled={change.isPending} onClick={() => change.mutate(() => api.triageTask(triage, "today"))}>今日やる</Button><Button variant="outline" disabled={change.isPending} onClick={() => change.mutate(() => api.triageTask(triage, "later"))}>あとで</Button><Button variant="ghost" disabled={change.isPending} onClick={() => change.mutate(() => api.triageTask(triage, "skip"))}>捨てる</Button></div><p className="text-xs text-muted-foreground">捨てた項目は下の「見送ったタスク」から戻せます。</p></> : <p className="text-sm text-muted-foreground">{view.deferredToday ? "今日の仕分けは済みました。また明日。" : "未整理のタスクはありません。"}</p>}
        </CardBody></Card>
        {view.skipped.length > 0 && <details className="text-sm"><summary className="cursor-pointer text-muted-foreground">見送ったタスク {view.skipped.length}件 · 復元</summary><div className="ds-stack mt-3">{view.skipped.map(t => <div className="flex items-center gap-3" key={taskKey(t)}><span className="flex-1">{t.display}</span><Button variant="ghost" disabled={change.isPending} onClick={() => setState(t, "todo")}>戻す</Button></div>)}</div></details>}
        <details><summary className="text-sm text-muted-foreground cursor-pointer">思考の置き場 · メモ／ホワイトボード</summary><div className="grid grid-cols-1 md:grid-cols-2 gap-6 mt-4"><MemoSummary /><Whiteboard /></div></details>
      </div>
      <ActivityRail view={view} />
    </div>
  </div>;
}
