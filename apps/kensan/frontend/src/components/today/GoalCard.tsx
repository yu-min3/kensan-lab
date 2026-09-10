import { Link } from "react-router-dom";
import { ArrowUpRight, Plus } from "lucide-react";
import { type ProjectSummary, type Routine, type Task } from "../../lib/api";
import { Card, CardBody } from "../ui/card";
import { Button } from "../ui/button";
import { Badge } from "../ui/badge";
import { daysUntil, ranked, taskKey } from "./model";

export function TaskCheck({ task, date, busy, onState }: { task: Task; date: string; busy: boolean; onState: (task: Task, state: Task["state"]) => void }) {
  const done = task.state === "done";
  return <label className="flex items-start gap-3 py-2 cursor-pointer">
    <input type="checkbox" className="mt-1 h-5 w-5 shrink-0 accent-brand" checked={done} disabled={busy} onChange={() => onState(task, done ? "todo" : "done")} />
    <span className={`min-w-0 flex-1 text-sm break-words ${done ? "line-through text-muted-foreground" : ""}`}>{task.display}</span>
    {task.due && <Badge variant={task.due < date && !done ? "warning" : "muted"}>{task.due.slice(5)}{task.due < date && !done ? " 超過" : ""}</Badge>}
  </label>;
}

export function RoutineCheck({ routine: r, busy, onToggle }: { routine: Routine; busy: boolean; onToggle: (r: Routine) => void }) {
  const current = r.periods[0];
  const short = r.text.split("（")[0];
  return <div className="ds-stack">
    <label className="flex flex-wrap items-center gap-3 cursor-pointer">
      <input type="checkbox" className="h-5 w-5 accent-brand" checked={r.doneToday} disabled={busy} onChange={() => onToggle(r)} />
      <span className="text-sm font-medium flex-1">{short}</span>
      <span className="text-xs text-muted-foreground">{r.schedule}</span>
      <span className="font-mono tnum text-sm">{r.supported ? `${current?.count ?? 0}/${r.target}` : (r.doneToday ? "今日 記録済み" : "今日 未記録")}</span>
    </label>
    {r.supported ? <div className="flex flex-wrap items-center gap-2 pl-8">
      <div className="flex gap-1.5" aria-label={`${short}の直近8期間`}>
        {[...r.periods].reverse().map(p => <span key={p.start} title={`${p.start} · ${p.count}/${p.target} · ${p.status === "unknown" ? "記録開始前" : p.status === "pending" ? "進行中" : p.status === "met" ? "達成" : "未達"}`} className={`h-3 w-3 rounded-full border ${p.status === "met" ? "bg-brand border-brand" : p.status === "missed" ? "border-dashed border-muted-foreground" : p.status === "pending" ? "bg-brand/20 border-brand" : "bg-muted border-border"}`} />)}
      </div>
      <span className="text-xs text-muted-foreground">{r.streak ? `${r.streak}${r.unit} 継続` : "ここから積み上げる"}{!r.expectedToday && " · 今日は予定なし"}</span>
    </div> : <p className="text-xs text-muted-foreground pl-8">補足条件あり：達成判定は保留。{r.text}</p>}
  </div>;
}

export function GoalCard({ project: p, tasks, candidates, routines, date, busy, onState, onChoose, onRoutine, forecast, phase }: {
  project: ProjectSummary; tasks: Task[]; candidates: Task[]; routines: Routine[]; date: string; busy: boolean;
  onState: (t: Task, state: Task["state"]) => void; onChoose: (t: Task) => void; onRoutine: (r: Routine) => void;
  forecast?: string; phase?: string;
}) {
  const open = ranked(tasks.filter(t => t.state === "todo"));
  const completed = tasks.filter(t => t.state === "done");
  const candidate = ranked(candidates.filter(t => t.state === "todo"))[0];
  const metric = p.metric;
  const percent = metric?.direction === "increase" && metric.current !== undefined && metric.target !== undefined && metric.target > 0 ? Math.min(100, Math.max(0, metric.current / metric.target * 100)) : null;
  const remaining = daysUntil(p.deadline ?? "", date);
  return <Card><CardBody className="ds-stack">
    <div className="flex items-start justify-between gap-4">
      <div className="min-w-0 ds-stack">
        <Link to={`/projects/${p.name}`} className="text-xs text-brand font-medium inline-flex items-center gap-1">{p.name}<ArrowUpRight size={14} /></Link>
        <h3 className="h-serif text-lg font-semibold break-words">{p.goal.replace(/\*\*/g, "") || "暮らしを整える、一歩ずつ"}</h3>
      </div>
      {metric?.current !== undefined && <div className="shrink-0 text-right"><div className="font-mono tnum text-2xl">{metric.current}<span className="text-sm text-muted-foreground">{metric.target !== undefined && ` / ${metric.target}`}</span></div><div className="text-xs text-muted-foreground">{metric.label} {metric.unit}</div></div>}
    </div>
    {percent !== null && <div role="progressbar" aria-label={metric?.label} aria-valuenow={Math.round(percent)} aria-valuemin={0} aria-valuemax={100} className="h-1.5 rounded-full bg-muted overflow-hidden"><div className="h-full bg-brand rounded-full" style={{ width: `${percent}%` }} /></div>}
    <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
      {forecast && <span>{forecast}</span>}
      {p.milestonesTotal > 0 && <Link to={`/projects/${p.name}`}>節目 <span className="font-mono tnum">{p.milestonesDone}/{p.milestonesTotal}</span></Link>}
      {remaining !== null && <span>{remaining < 0 ? `期限から${-remaining}日` : `期限まで${remaining}日`} · {p.deadline}</span>}
      {!metric && <span>進捗はプロジェクトで確認</span>}
    </div>
    {phase && <p className="text-xs text-muted-foreground">今の節目 · {phase.replace(/\*\*/g, "")}</p>}
    <div className="border-t border-border pt-3 ds-stack">
      {routines.map(r => <RoutineCheck key={r.id} routine={r} busy={busy} onToggle={onRoutine} />)}
      {open[0] ? <TaskCheck task={open[0]} date={date} busy={busy} onState={onState} /> : candidate ?
        <Button variant="ghost" className="!h-auto py-3 text-left justify-start whitespace-normal" disabled={busy} onClick={() => onChoose(candidate)}><Plus size={16} className="shrink-0" /><span>今日ここに一歩置くなら — {candidate.display}</span></Button> : <Link to="/tasks" className="text-sm text-muted-foreground py-2">今日の一歩はまだありません。タスクを選ぶ →</Link>}
      {open.length > 1 && <details><summary className="text-xs text-muted-foreground cursor-pointer">ほかに今日のタスク {open.length - 1}件{open.some(t => t.due && t.due <= date) ? " · 期限のある項目を含む" : ""}</summary>{open.slice(1).map(t => <TaskCheck key={taskKey(t)} task={t} date={date} busy={busy} onState={onState} />)}</details>}
      {completed.length > 0 && <details><summary className="text-xs text-muted-foreground cursor-pointer">完了済み {completed.length}件 · 確認／取消</summary>{completed.map(t => <TaskCheck key={taskKey(t)} task={t} date={date} busy={busy} onState={onState} />)}</details>}
    </div>
  </CardBody></Card>;
}
