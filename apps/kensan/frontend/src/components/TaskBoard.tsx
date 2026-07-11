import { useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  DndContext,
  closestCenter,
  useDraggable,
  useDroppable,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy, arrayMove } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Inbox, ArchiveRestore, GripVertical, ChevronDown, ChevronRight, Calendar, Trash2, Plus } from "lucide-react";
import clsx from "clsx";
import { api, ApiError, todayISO, type Task, type TaskSaveInput } from "../lib/api";
import { TaskDialog, taskToDraft, type TaskDraft } from "./TaskDialog";
import { Card, CardHead, CardBody } from "./ui/card";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Empty, ErrorState, SkeletonRows } from "./ui/states";
import { useToast } from "./ui/toast";

// 今日やる（@today / @due≤今日 + todo.md ## Now）⇄ ストック（project の未完了タスク）。
// レーン間移動は @today タグの付け外し。ストックは @p(N) 優先度で縦に並び、ドラッグで並べ替え。
// 本文クリックで編集ダイアログ、「+ 追加」で作成（共通フォーム）。

const key = (t: Task) => `${t.file}:${t.line}`;
const STEP = 1000;

// lanes: "split" = 今日やる｜ストックを横並び（タスクページ・広い画面向け）。
// "stack" = 縦積み（ダッシュボードのメイン列。右レールに思考の道具を置くため）。
export function TaskBoard({ lanes = "split" }: { lanes?: "split" | "stack" } = {}) {
  const qc = useQueryClient();
  const board = useQuery({ queryKey: ["board"], queryFn: api.board });
  const laneGrid = lanes === "stack" ? "ds-section" : "grid grid-cols-1 lg:grid-cols-2 gap-6 items-start";
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.projects });
  const [lastError, setLastError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<{ mode: "create" | "edit"; initial: TaskDraft } | null>(null);

  const onErr = (e: unknown) =>
    setLastError(
      e instanceof ApiError && e.status === 409
        ? "ファイルが他のクライアントに編集されています。再読込しました。"
        : String(e instanceof Error ? e.message : e),
    );
  const settled = () => qc.invalidateQueries({ queryKey: ["board"] });

  const setToday = useMutation({
    mutationFn: ({ task, on }: { task: Task; on: boolean }) => api.setToday(task, on),
    onSuccess: () => setLastError(null),
    onError: onErr,
    onSettled: settled,
  });
  const toggle = useMutation({
    mutationFn: ({ task, state }: { task: Task; state: Task["state"] }) => api.setTaskState(task, state),
    onSettled: settled,
  });
  const archive = useMutation({ mutationFn: (t: Task) => api.archiveToDaily(t), onSettled: settled });
  const save = useMutation({
    mutationFn: (input: TaskSaveInput) => api.saveTask(input),
    onError: onErr,
    onSuccess: () => setDialog(null),
    onSettled: settled,
  });
  // 削除は確認ダイアログでなく Toast + Undo（patterns.md 04/05: 取り消し可能な操作）。
  // 元に戻すは saveTask による再作成なので、行位置と @p(N) は復元されない（内容は完全復元）。
  const toast = useToast();
  const del = useMutation({
    mutationFn: (t: Task) => api.deleteTask(t),
    onError: onErr,
    onSuccess: (_data, t) => {
      setDialog(null);
      toast({
        title: "タスクを削除しました",
        desc: t.display,
        durationMs: 8000,
        action: {
          label: "元に戻す",
          onClick: () =>
            save.mutate({
              project: t.project ?? "",
              display: t.display,
              today: t.today,
              due: t.due ?? "",
              milestone: t.milestone ?? "",
            }),
        },
      });
    },
    onSettled: settled,
  });
  const setPriority = useMutation({
    mutationFn: ({ task, priority }: { task: Task; priority: number }) => api.setPriority(task, priority),
    onError: onErr,
    onSettled: settled,
  });
  const reorderAll = useMutation({
    mutationFn: (items: { file: string; line: number; text: string; priority: number }[]) => api.reorderTasks(items),
    onError: onErr,
    onSettled: settled,
  });

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));
  const stock = board.data?.stock ?? [];
  const projectList = (projects.data?.projects ?? []).map((p) => p.name);

  function reorder(active: Task, over: Task) {
    const oldI = stock.findIndex((t) => key(t) === key(active));
    const newI = stock.findIndex((t) => key(t) === key(over));
    if (oldI < 0 || newI < 0 || oldI === newI) return;
    const moved = arrayMove(stock, oldI, newI);
    if (!stock.every((t) => (t.priority ?? 0) > 0)) {
      reorderAll.mutate(moved.map((t, i) => ({ file: t.file, line: t.line, text: t.text, priority: (i + 1) * STEP })));
      return;
    }
    const pos = moved.findIndex((t) => key(t) === key(active));
    const prevP = pos > 0 ? (moved[pos - 1].priority ?? 0) : 0;
    const nextP = pos < moved.length - 1 ? (moved[pos + 1].priority ?? 0) : 0;
    let p: number;
    if (prevP && nextP) p = Math.floor((prevP + nextP) / 2);
    else if (prevP) p = prevP + STEP;
    else if (nextP) p = Math.floor(nextP / 2);
    else p = STEP;
    if (p <= prevP || (nextP && p >= nextP) || p <= 0) {
      reorderAll.mutate(moved.map((t, i) => ({ file: t.file, line: t.line, text: t.text, priority: (i + 1) * STEP })));
    } else {
      setPriority.mutate({ task: active, priority: p });
    }
  }

  function onDragEnd(e: DragEndEvent) {
    const a = e.active.data.current as { task: Task; lane: "today" | "stock" } | undefined;
    if (!a || !e.over) return;
    const overData = e.over.data.current as { task?: Task; lane?: "today" | "stock" } | undefined;
    const toLane = e.over.id === "today" ? "today" : e.over.id === "stock" ? "stock" : overData?.lane;
    if (!toLane) return;
    if (a.lane === "stock" && toLane === "stock") {
      if (overData?.task) reorder(a.task, overData.task);
    } else if (toLane === "today" && a.lane !== "today") {
      setToday.mutate({ task: a.task, on: true });
    } else if (toLane === "stock" && a.lane === "today" && a.task.project) {
      setToday.mutate({ task: a.task, on: false });
    }
  }

  if (board.isPending) {
    return (
      <div className={laneGrid}>
        <Card><CardBody><SkeletonRows rows={5} /></CardBody></Card>
        <Card><CardBody><SkeletonRows rows={5} /></CardBody></Card>
      </div>
    );
  }
  if (board.isError) {
    return <Card><CardBody><ErrorState error={board.error} onRetry={() => board.refetch()} /></CardBody></Card>;
  }

  const today = board.data.today ?? [];
  // project ごとの定義済みマイルストーン（## マイルストーン）。ダイアログのドロップダウン用。
  const milestonesByProject: Record<string, string[]> = {};
  for (const ms of board.data.milestones ?? []) {
    if (ms.project) (milestonesByProject[ms.project] ??= []).push(ms.display);
  }
  const busy =
    setToday.isPending || toggle.isPending || archive.isPending || setPriority.isPending || reorderAll.isPending || del.isPending || save.isPending;
  const done = today.filter((t) => t.state === "done");

  const openEdit = (t: Task) => setDialog({ mode: "edit", initial: taskToDraft(t) });
  const onDelete = (t: Task) => del.mutate(t);
  const deleteFromDialog = () => {
    const i = dialog?.initial;
    if (!i?.file || !i.line) return;
    // Undo（saveTask 再作成）に使うため、locator 以外のフィールドも引き継ぐ
    del.mutate({
      file: i.file,
      line: i.line,
      text: i.text ?? "",
      display: i.display,
      project: i.project || undefined,
      today: i.today,
      due: i.due || undefined,
      milestone: i.milestone || undefined,
    } as Task);
  };

  return (
    <div className="ds-section">
      {lastError && (
        <div className="text-sm text-destructive border border-destructive/30 bg-destructive/5 rounded-md px-3 py-2">
          {lastError}
        </div>
      )}
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <div className={laneGrid}>
          <TodayLane
            tasks={today}
            busy={busy}
            onToggle={(t) => toggle.mutate({ task: t, state: "done" })}
            onSendToStock={(t) => setToday.mutate({ task: t, on: false })}
            onDelete={onDelete}
            onOpen={openEdit}
            onCreate={() => setDialog({ mode: "create", initial: { project: "", display: "", today: true, due: "", milestone: "" } })}
          />
          <StockLane
            tasks={stock}
            busy={busy}
            onSendToToday={(t) => setToday.mutate({ task: t, on: true })}
            onDelete={onDelete}
            onOpen={openEdit}
            onCreate={() =>
              setDialog({ mode: "create", initial: { project: projectList[0] ?? "", display: "", today: false, due: "", milestone: "" } })
            }
          />
        </div>
      </DndContext>
      <DoneSection
        done={done}
        busy={busy}
        onUncheck={(t) => toggle.mutate({ task: t, state: "todo" })}
        onArchive={(t) => archive.mutate(t)}
      />
      {dialog && (
        <TaskDialog
          mode={dialog.mode}
          initial={dialog.initial}
          projects={projectList}
          milestonesByProject={milestonesByProject}
          busy={save.isPending || del.isPending}
          onSave={(input) => save.mutate(input)}
          onDelete={dialog.mode === "edit" ? deleteFromDialog : undefined}
          onClose={() => setDialog(null)}
        />
      )}
    </div>
  );
}

function DueBadge({ due }: { due: string }) {
  const today = todayISO();
  const variant = due < today ? "destructive" : due === today ? "warning" : "muted";
  const [, m, d] = due.split("-");
  return (
    <Badge variant={variant}>
      <Calendar size={11} className="mr-0.5" />
      <span className="font-mono tnum">{`${Number(m)}/${Number(d)}`}</span>
    </Badge>
  );
}

function Badges({ task }: { task: Task }) {
  return (
    <>
      {task.due && <DueBadge due={task.due} />}
      {task.milestone && <Badge variant="brand">▸ {task.milestone}</Badge>}
      {task.project && <Badge variant="outline">{task.project}</Badge>}
    </>
  );
}

// 本文。クリックで編集ダイアログを開く。clamp 時は 2 行で省略し、全文は title で出す。
function TaskName({
  task,
  onOpen,
  className,
  clamp,
}: {
  task: Task;
  onOpen: (t: Task) => void;
  className?: string;
  clamp?: boolean;
}) {
  return (
    <span
      className={clsx("flex-1 text-sm cursor-pointer hover:text-brand", clamp && "line-clamp-2", className)}
      onClick={() => onOpen(task)}
      title={clamp ? task.display : "クリックで編集"}
    >
      {task.display}
    </span>
  );
}

function CreateButton({ onClick, busy }: { onClick: () => void; busy: boolean }) {
  return (
    <Button variant="ghost" size="sm" disabled={busy} onClick={onClick}>
      <Plus size={14} />
      追加
    </Button>
  );
}

function TodayLane({
  tasks,
  busy,
  onToggle,
  onSendToStock,
  onDelete,
  onOpen,
  onCreate,
}: {
  tasks: Task[];
  busy: boolean;
  onToggle: (t: Task) => void;
  onSendToStock: (t: Task) => void;
  onDelete: (t: Task) => void;
  onOpen: (t: Task) => void;
  onCreate: () => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: "today" });
  const visible = tasks.filter((t) => t.state !== "done");
  return (
    <Card className={clsx(isOver && "ring-2 ring-ring")}>
      <CardHead
        title="今日やる"
        sub="@today / @due≤今日"
        badge={
          <Badge variant={visible.length === 0 ? "success" : "brand"}>
            <span className="font-mono tnum">残 {visible.length}</span>
          </Badge>
        }
        actions={<CreateButton busy={busy} onClick={onCreate} />}
      />
      <CardBody>
        <div ref={setNodeRef} className="ds-stack !gap-1 min-h-24">
          {visible.length === 0 ? (
            <Empty
              icon={<Inbox />}
              title={tasks.length === 0 ? "今日やるタスクがありません" : "未完了のタスクなし"}
              desc="「+ 追加」で作るか、ストックからドラッグしてください。"
            />
          ) : (
            visible.map((t) => (
              <DraggableRow key={key(t)} task={t} lane="today" busy={busy}>
                <input
                  type="checkbox"
                  className="size-4 accent-[hsl(var(--brand))]"
                  checked={false}
                  onChange={() => onToggle(t)}
                  aria-label={`${t.display} を完了にする`}
                />
                <TaskName task={t} onOpen={onOpen} className={t.state === "skipped" ? "line-through text-muted-foreground" : undefined} />
                <Badges task={t} />
                {/* ストックへ戻す = @today タグ外し。@due≤今日 由来のタスクは @today を
                    外しても today 判定のままで「戻したのに復活する」ように見えるので出さない
                    （kensan 今日やる復活問題 調査レポートの方針 A）。期限を動かしたい場合は
                    本文クリック → 編集ダイアログで期限を変更・削除する */}
                {t.project && t.today && !(t.due && t.due <= todayISO()) && (
                  <button
                    className="text-xs text-muted-foreground hover:text-foreground opacity-0 group-hover:opacity-100 transition-opacity"
                    disabled={busy}
                    onClick={() => onSendToStock(t)}
                    title="ストックへ戻す"
                  >
                    ↩
                  </button>
                )}
                <DeleteButton busy={busy} onClick={() => onDelete(t)} />
              </DraggableRow>
            ))
          )}
        </div>
      </CardBody>
    </Card>
  );
}

function StockLane({
  tasks,
  busy,
  onSendToToday,
  onDelete,
  onOpen,
  onCreate,
}: {
  tasks: Task[];
  busy: boolean;
  onSendToToday: (t: Task) => void;
  onDelete: (t: Task) => void;
  onOpen: (t: Task) => void;
  onCreate: () => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: "stock" });
  return (
    <Card className={clsx(isOver && "ring-2 ring-ring")}>
      <CardHead
        title="ストック"
        sub="優先度順（ドラッグで並べ替え）"
        badge={tasks.length > 0 ? <Badge variant="muted"><span className="font-mono tnum">{tasks.length}</span></Badge> : undefined}
        actions={<CreateButton busy={busy} onClick={onCreate} />}
      />
      <CardBody>
        <div ref={setNodeRef} className="min-h-24">
          {tasks.length === 0 ? (
            <Empty icon={<Inbox />} title="ストックが空です" desc="「+ 追加」で作るか、各 project の README ## タスク に書くと集まります。" />
          ) : (
            <SortableContext items={tasks.map(key)} strategy={verticalListSortingStrategy}>
              <div className="ds-stack !gap-1">
                {tasks.map((t, i) => (
                  <SortableRow key={key(t)} task={t} busy={busy}>
                    <span
                      className="font-mono tnum text-xs text-muted-foreground/60 w-5 shrink-0 text-right"
                      title="優先度順の位置（ドラッグで並べ替え）"
                    >
                      {i + 1}
                    </span>
                    <TaskName task={t} onOpen={onOpen} clamp />
                    <Badges task={t} />
                    <Button variant="ghost" size="sm" disabled={busy} onClick={() => onSendToToday(t)}>
                      今日へ
                    </Button>
                    <DeleteButton busy={busy} onClick={() => onDelete(t)} />
                  </SortableRow>
                ))}
              </div>
            </SortableContext>
          )}
        </div>
      </CardBody>
    </Card>
  );
}

function rowClass(dragging: boolean) {
  return clsx(
    "flex items-center gap-2 rounded-md border border-border bg-card px-2 py-1.5 group",
    "hover:bg-accent/40 transition-colors duration-fast",
    dragging && "opacity-60 shadow-md z-10 relative",
  );
}

function Handle(props: Record<string, unknown>) {
  return (
    <button className="cursor-grab text-muted-foreground touch-none p-1 -m-1" aria-label="ドラッグ" {...props}>
      <GripVertical size={14} />
    </button>
  );
}

function DeleteButton({ onClick, busy }: { onClick: () => void; busy: boolean }) {
  return (
    <button
      className="size-6 grid place-items-center rounded-md text-destructive hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition-opacity duration-fast shrink-0"
      disabled={busy}
      onClick={onClick}
      title="削除"
      aria-label="タスクを削除"
    >
      <Trash2 size={14} />
    </button>
  );
}

function SortableRow({ task, busy, children }: { task: Task; busy: boolean; children: ReactNode }) {
  const { setNodeRef, attributes, listeners, transform, transition, isDragging } = useSortable({
    id: key(task),
    data: { task, lane: "stock" },
    disabled: busy,
  });
  return (
    <div ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition }} className={rowClass(isDragging)}>
      <Handle {...attributes} {...listeners} />
      {children}
    </div>
  );
}

function DraggableRow({ task, lane, busy, children }: { task: Task; lane: "today" | "stock"; busy: boolean; children: ReactNode }) {
  const { setNodeRef, attributes, listeners, transform, isDragging } = useDraggable({
    id: key(task),
    data: { task, lane },
    disabled: busy,
  });
  return (
    <div
      ref={setNodeRef}
      style={transform ? { transform: `translate(${transform.x}px, ${transform.y}px)` } : undefined}
      className={rowClass(isDragging)}
    >
      <Handle {...attributes} {...listeners} />
      {children}
    </div>
  );
}

function DoneSection({
  done,
  busy,
  onUncheck,
  onArchive,
}: {
  done: Task[];
  busy: boolean;
  onUncheck: (t: Task) => void;
  onArchive: (t: Task) => void;
}) {
  const [open, setOpen] = useState(false);
  if (done.length === 0) return null;
  return (
    <Card>
      <button className="ds-card w-full flex items-center justify-between text-left" onClick={() => setOpen(!open)} aria-expanded={open}>
        <span className="h-serif text-base font-semibold flex items-center gap-2">
          {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          完了済み
        </span>
        <Badge variant="success"><span className="font-mono tnum">{done.length}</span></Badge>
      </button>
      {open && (
        <CardBody className="!pt-0 ds-stack !gap-0">
          {done.map((t) => (
            <div key={key(t)} className="ds-row flex items-center gap-3 border-b border-border last:border-b-0">
              <input
                type="checkbox"
                className="size-4 accent-[hsl(var(--brand))]"
                checked
                disabled={busy}
                onChange={() => onUncheck(t)}
                aria-label={`${t.display} を未完了に戻す`}
              />
              <span className="flex-1 text-sm line-through text-muted-foreground">{t.display}</span>
              {t.project && <Badge variant="outline">{t.project}</Badge>}
              <Button
                variant="ghost"
                size="sm"
                iconOnly
                disabled={busy}
                aria-label="daily に片付ける"
                title="daily に片付ける（0〜6時は前日の daily へ）"
                onClick={() => onArchive(t)}
              >
                <ArchiveRestore size={14} />
              </Button>
            </div>
          ))}
        </CardBody>
      )}
    </Card>
  );
}
