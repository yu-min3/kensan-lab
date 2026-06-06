import { useMemo, useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  DndContext,
  useDraggable,
  useDroppable,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import { Inbox, ArchiveRestore, GripVertical, ChevronDown, ChevronRight } from "lucide-react";
import clsx from "clsx";
import { api, ApiError, type Task } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardHead, CardBody } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Empty, ErrorState, SkeletonRows } from "../components/ui/states";

// タスクかんばん — ストック（projects/<p>/README.md ## タスク）と
// 今日やる（todo.md ## Now）。ドラッグ / ボタン = チェックボックス行のファイル間移動。
// /morning が Claude 側で同じ操作をする対称性（unification-plan.md）。

export function TasksPage() {
  const qc = useQueryClient();
  const board = useQuery({ queryKey: ["board"], queryFn: api.board });
  const [lastError, setLastError] = useState<string | null>(null);

  const move = useMutation({
    mutationFn: ({ task, to, project }: { task: Task; to: "today" | "stock" | "daily"; project?: string }) =>
      api.moveTask(task, to, project),
    onSuccess: () => setLastError(null),
    onError: (e) =>
      setLastError(
        e instanceof ApiError && e.status === 409
          ? "ファイルが他のクライアントに編集されています。ボードを再読込しました。"
          : String(e instanceof Error ? e.message : e),
      ),
    onSettled: () => qc.invalidateQueries({ queryKey: ["board"] }),
  });

  const toggle = useMutation({
    mutationFn: ({ task, state }: { task: Task; state: Task["state"] }) => api.setTaskState(task, state),
    onSettled: () => qc.invalidateQueries({ queryKey: ["board"] }),
  });

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));

  // project → Task[] のグルーピングは一度だけ計算する（ドラッグ中の isOver 再レンダーで
  // filter を毎回回さない）。project の無い stock タスクは backend 契約違反なので弾く。
  const stockByProject = useMemo(() => {
    const m = new Map<string, Task[]>();
    for (const t of board.data?.stock ?? []) {
      if (!t.project) continue;
      const arr = m.get(t.project);
      if (arr) arr.push(t);
      else m.set(t.project, [t]);
    }
    return m;
  }, [board.data]);

  function onDragEnd(e: DragEndEvent) {
    const task = e.active.data.current?.task as Task | undefined;
    const target = e.over?.id as string | undefined;
    if (!task || !target) return;
    const fromToday = task.file === "todo.md";
    if (target === "today" && !fromToday) {
      move.mutate({ task, to: "today" });
    } else if (target.startsWith("stock:") && fromToday) {
      move.mutate({ task, to: "stock", project: target.slice("stock:".length) });
    }
  }

  let content: ReactNode;
  if (board.isPending) {
    content = (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card><CardBody><SkeletonRows rows={6} /></CardBody></Card>
        <Card><CardBody><SkeletonRows rows={6} /></CardBody></Card>
      </div>
    );
  } else if (board.isError) {
    content = (
      <Card><CardBody><ErrorState error={board.error} onRetry={() => board.refetch()} /></CardBody></Card>
    );
  } else {
    content = (
      <DndContext sensors={sensors} onDragEnd={onDragEnd}>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
          <TodayColumn
            tasks={board.data.today ?? []}
            onToggle={(t) => toggle.mutate({ task: t, state: t.state === "done" ? "todo" : "done" })}
            onSendToDaily={(t) => move.mutate({ task: t, to: "daily" })}
            busy={move.isPending || toggle.isPending}
          />
          <StockColumn
            stockByProject={stockByProject}
            someday={board.data.someday ?? []}
            milestones={board.data.milestones ?? []}
            onSendToToday={(t) => move.mutate({ task: t, to: "today" })}
            busy={move.isPending}
          />
        </div>
      </DndContext>
    );
  }

  return (
    <>
      <PageHeader
        eyebrow="タスク"
        title="かんばん"
        sub="左右のドラッグ（またはボタン）でストック ⇄ 今日を移動。実体は markdown のチェックボックス行。"
      />
      {lastError && (
        <div className="mb-4 text-sm text-destructive border border-destructive/30 bg-destructive/5 rounded-md px-3 py-2">
          {lastError}
        </div>
      )}
      {content}
    </>
  );
}

function TodayColumn({
  tasks,
  onToggle,
  onSendToDaily,
  busy,
}: {
  tasks: Task[];
  onToggle: (t: Task) => void;
  onSendToDaily: (t: Task) => void;
  busy: boolean;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: "today" });
  const done = tasks.filter((t) => t.state === "done").length;
  return (
    <Card className={clsx(isOver && "ring-2 ring-ring")}>
      <CardHead
        title="今日やる"
        sub="todo.md · Now"
        badge={
          tasks.length > 0 ? (
            <Badge variant={done === tasks.length ? "success" : "brand"}>
              <span className="font-mono tnum">{done}/{tasks.length}</span>
            </Badge>
          ) : undefined
        }
      />
      <CardBody>
        <div ref={setNodeRef} className="ds-stack min-h-24">
          {tasks.length === 0 ? (
            <Empty
              icon={<Inbox />}
              title="今日のタスクがありません"
              desc="ストックからドラッグするか、Claude Code で /morning を実行してください。"
            />
          ) : (
            tasks.map((t) => (
              <TaskCard key={`${t.file}:${t.line}`} task={t} busy={busy}>
                <input
                  type="checkbox"
                  className="size-4 accent-[hsl(var(--brand))]"
                  checked={t.state === "done"}
                  onChange={() => onToggle(t)}
                  aria-label={`${t.text} の完了切替`}
                />
                <span className={clsx("flex-1 text-sm", t.state === "done" && "line-through text-muted-foreground")}>
                  {t.text}
                </span>
                {t.state === "done" && (
                  <Button
                    variant="ghost"
                    size="sm"
                    iconOnly
                    aria-label="daily に片付ける"
                    title="daily に片付ける（0〜6時は前日の daily へ）"
                    onClick={() => onSendToDaily(t)}
                  >
                    <ArchiveRestore size={14} />
                  </Button>
                )}
              </TaskCard>
            ))
          )}
        </div>
      </CardBody>
    </Card>
  );
}

function StockColumn({
  stockByProject,
  someday,
  milestones,
  onSendToToday,
  busy,
}: {
  stockByProject: Map<string, Task[]>;
  someday: Task[];
  milestones: Task[];
  onSendToToday: (t: Task) => void;
  busy: boolean;
}) {
  return (
    <div className="ds-section">
      <Card>
        <CardHead title="ストック" sub="projects/*/README.md · タスク" />
        <CardBody className="ds-stack !gap-5">
          {stockByProject.size === 0 ? (
            <Empty
              icon={<Inbox />}
              title="ストックが空です"
              desc="各プロジェクトの README の ## タスク セクションに追加すると、ここに並びます。"
            />
          ) : (
            [...stockByProject.entries()].map(([project, tasks]) => (
              <ProjectGroup
                key={project}
                project={project}
                tasks={tasks}
                onSendToToday={onSendToToday}
                busy={busy}
              />
            ))
          )}
        </CardBody>
      </Card>
      <Collapsible title="いつかやる" tasks={someday} />
      <Collapsible title="マイルストーン" tasks={milestones} />
    </div>
  );
}

function ProjectGroup({
  project,
  tasks,
  onSendToToday,
  busy,
}: {
  project: string;
  tasks: Task[];
  onSendToToday: (t: Task) => void;
  busy: boolean;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `stock:${project}` });
  const open = tasks.filter((t) => t.state === "todo");
  return (
    <div ref={setNodeRef} className={clsx("rounded-md", isOver && "ring-2 ring-ring")}>
      <div className="flex items-center justify-between pb-1">
        <span className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">{project}</span>
        <span className="font-mono tnum text-xs text-muted-foreground">{open.length}</span>
      </div>
      <div className="ds-stack">
        {open.map((t) => (
          <TaskCard key={`${t.file}:${t.line}`} task={t} busy={busy}>
            <span className="flex-1 text-sm">{t.text}</span>
            <Button variant="ghost" size="sm" onClick={() => onSendToToday(t)}>
              今日へ
            </Button>
          </TaskCard>
        ))}
        {open.length === 0 && (
          <p className="text-xs text-muted-foreground py-1">未着手のタスクなし</p>
        )}
      </div>
    </div>
  );
}

function TaskCard({ task, busy, children }: { task: Task; busy: boolean; children: ReactNode }) {
  const { setNodeRef, attributes, listeners, transform, isDragging } = useDraggable({
    id: `${task.file}:${task.line}`,
    data: { task },
    disabled: busy,
  });
  return (
    <div
      ref={setNodeRef}
      style={transform ? { transform: `translate(${transform.x}px, ${transform.y}px)` } : undefined}
      className={clsx(
        "flex items-center gap-2 rounded-md border border-border bg-card px-2 py-1.5",
        "hover:shadow-sm transition-shadow duration-fast",
        isDragging && "opacity-70 shadow-md z-10 relative",
      )}
    >
      <button
        className="cursor-grab text-muted-foreground touch-none p-1 -m-1"
        aria-label={`${task.text} をドラッグ`}
        {...attributes}
        {...listeners}
      >
        <GripVertical size={14} />
      </button>
      {children}
    </div>
  );
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
                {t.text}
              </span>
              {t.project && <Badge variant="outline">{t.project}</Badge>}
            </div>
          ))}
        </CardBody>
      )}
    </Card>
  );
}
