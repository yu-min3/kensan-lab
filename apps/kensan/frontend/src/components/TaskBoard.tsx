import { useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  DndContext,
  closestCenter,
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
import { api, ApiError, todayISO, type Band, type Task, type TaskSaveInput } from "../lib/api";
import { TaskDialog, taskToDraft, bandOfTask, type TaskDraft } from "./TaskDialog";
import { Card, CardHead, CardBody } from "./ui/card";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Empty, ErrorState, SkeletonRows } from "./ui/states";
import { useToast } from "./ui/toast";

// 時間軸バンドのかんばん: 今日 / 今週 / 今月 / 中期以降。
// タスクは project の ## タスク に住み、行内タグ（@today/@week/@month）でバンドが決まる。
// レーン間ドラッグ = バンドタグの張り替え（setBand）。各バンド内は @p(N) でドラッグ並べ替え。
// 今日・今週は常時表示、今月・中期は折りたたみ（review で 1 つ上のバンドから繰り上げる運用）。

const key = (t: Task) => `${t.file}:${t.line}`;
const STEP = 1000;

type LaneDef = { band: Band; title: string; sub: string; collapsible: boolean };
const LANES: LaneDef[] = [
  { band: "today", title: "今日", sub: "@today / @due≤今日", collapsible: false },
  { band: "week", title: "今週", sub: "@week / @due≤今週末", collapsible: false },
  { band: "month", title: "今月", sub: "@month / @due≤月末", collapsible: true },
  { band: "later", title: "中期以降", sub: "着手時期は未定（backlog）", collapsible: true },
];

// lanes は後方互換のため受け取るが、4 バンドは縦積みで描画する（今日を最上部に）。
export function TaskBoard(_props: { lanes?: "split" | "stack" } = {}) {
  const qc = useQueryClient();
  const board = useQuery({ queryKey: ["board"], queryFn: api.board });
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

  const setBand = useMutation({
    mutationFn: ({ task, band }: { task: Task; band: Band }) => api.setBand(task, band),
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
              band: bandOfTask(t),
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
  const projectList = (projects.data?.projects ?? []).map((p) => p.name);

  // バンド内の並べ替え（@p を中間値に書換。隙間が尽きたら全体を再採番）。
  function reorder(active: Task, over: Task, list: Task[]) {
    const oldI = list.findIndex((t) => key(t) === key(active));
    const newI = list.findIndex((t) => key(t) === key(over));
    if (oldI < 0 || newI < 0 || oldI === newI) return;
    const moved = arrayMove(list, oldI, newI);
    if (!list.every((t) => (t.priority ?? 0) > 0)) {
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

  if (board.isPending) {
    return (
      <div className="ds-section">
        <Card><CardBody><SkeletonRows rows={4} /></CardBody></Card>
        <Card><CardBody><SkeletonRows rows={4} /></CardBody></Card>
      </div>
    );
  }
  if (board.isError) {
    return <Card><CardBody><ErrorState error={board.error} onRetry={() => board.refetch()} /></CardBody></Card>;
  }

  const lists: Record<Band, Task[]> = {
    today: board.data.today ?? [],
    week: board.data.week ?? [],
    month: board.data.month ?? [],
    later: board.data.later ?? [],
  };

  function onDragEnd(e: DragEndEvent) {
    const a = e.active.data.current as { task: Task; band: Band } | undefined;
    if (!a || !e.over) return;
    const overData = e.over.data.current as { task?: Task; band?: Band } | undefined;
    const toBand = (LANES.some((l) => l.band === e.over!.id) ? e.over.id : overData?.band) as Band | undefined;
    if (!toBand) return;
    if (toBand === a.band) {
      // 今日は todo.md ## Now の並びを尊重して @p ソートしない（バックエンド仕様）ため、
      // バンド内の並べ替えは今週/今月/中期のみ。
      if (a.band !== "today" && overData?.task) reorder(a.task, overData.task, lists[a.band]);
    } else {
      setBand.mutate({ task: a.task, band: toBand });
    }
  }

  // project ごとの定義済みマイルストーン（## マイルストーン）。ダイアログのドロップダウン用。
  const milestonesByProject: Record<string, string[]> = {};
  for (const ms of board.data.milestones ?? []) {
    if (ms.project) (milestonesByProject[ms.project] ??= []).push(ms.display);
  }
  const busy =
    setBand.isPending || toggle.isPending || archive.isPending || setPriority.isPending || reorderAll.isPending || del.isPending || save.isPending;
  const done = lists.today.filter((t) => t.state === "done");

  const openEdit = (t: Task) => setDialog({ mode: "edit", initial: taskToDraft(t) });
  const onDelete = (t: Task) => del.mutate(t);
  const openCreate = (band: Band) =>
    setDialog({
      mode: "create",
      initial: { project: band === "today" ? "" : (projectList[0] ?? ""), display: "", band, due: "", milestone: "" },
    });
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
      today: i.band === "today",
      week: i.band === "week",
      month: i.band === "month",
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
        <div className="ds-section">
          {LANES.map((lane) => (
            <BandLane
              key={lane.band}
              def={lane}
              tasks={lists[lane.band]}
              busy={busy}
              onToggle={(t) => toggle.mutate({ task: t, state: "done" })}
              onPromoteToday={(t) => setBand.mutate({ task: t, band: "today" })}
              onDelete={onDelete}
              onOpen={openEdit}
              onCreate={() => openCreate(lane.band)}
            />
          ))}
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

// 1 バンドのレーン。今日は完了チェックボックスを持ち、完了は下の DoneSection へ回す。
// 今週/今月/中期は未完了のみで、優先度順の番号 + 「今日へ」クイック昇格を持つ。
// collapsible なバンド（今月・中期）は既定で畳む。
function BandLane({
  def,
  tasks,
  busy,
  onToggle,
  onPromoteToday,
  onDelete,
  onOpen,
  onCreate,
}: {
  def: LaneDef;
  tasks: Task[];
  busy: boolean;
  onToggle: (t: Task) => void;
  onPromoteToday: (t: Task) => void;
  onDelete: (t: Task) => void;
  onOpen: (t: Task) => void;
  onCreate: () => void;
}) {
  const [open, setOpen] = useState(!def.collapsible);
  const { setNodeRef, isOver } = useDroppable({ id: def.band });
  const isToday = def.band === "today";
  const visible = isToday ? tasks.filter((t) => t.state !== "done") : tasks;
  const count = visible.length;

  return (
    <Card className={clsx(isOver && "ring-2 ring-ring")}>
      {def.collapsible ? (
        <button
          className="ds-card w-full flex items-center justify-between text-left"
          onClick={() => setOpen(!open)}
          aria-expanded={open}
        >
          <span className="h-serif text-base font-semibold flex items-center gap-2">
            {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
            {def.title}
            <span className="text-xs font-normal text-muted-foreground">{def.sub}</span>
          </span>
          <Badge variant="muted"><span className="font-mono tnum">{count}</span></Badge>
        </button>
      ) : (
        <CardHead
          title={def.title}
          sub={def.sub}
          badge={
            isToday ? (
              <Badge variant={count === 0 ? "success" : "brand"}>
                <span className="font-mono tnum">残 {count}</span>
              </Badge>
            ) : count > 0 ? (
              <Badge variant="muted"><span className="font-mono tnum">{count}</span></Badge>
            ) : undefined
          }
          actions={<CreateButton busy={busy} onClick={onCreate} />}
        />
      )}
      {open && (
        <CardBody className={def.collapsible ? "!pt-0" : undefined}>
          <div ref={setNodeRef} className="min-h-16">
            {count === 0 ? (
              <Empty
                icon={<Inbox />}
                title={isToday ? "今日やるタスクがありません" : `${def.title}のタスクはありません`}
                desc={
                  isToday
                    ? "「+ 追加」で作るか、下のバンドからドラッグしてください。"
                    : "「+ 追加」で作るか、他のバンドからドラッグしてください。"
                }
              />
            ) : (
              <SortableContext items={visible.map(key)} strategy={verticalListSortingStrategy}>
                <div className="ds-stack !gap-1">
                  {visible.map((t, i) => (
                    <SortableRow key={key(t)} task={t} band={def.band} busy={busy}>
                      {isToday ? (
                        <input
                          type="checkbox"
                          className="size-4 accent-[hsl(var(--brand))]"
                          checked={false}
                          onChange={() => onToggle(t)}
                          aria-label={`${t.display} を完了にする`}
                        />
                      ) : (
                        <span
                          className="font-mono tnum text-xs text-muted-foreground/60 w-5 shrink-0 text-right"
                          title="優先度順の位置（ドラッグで並べ替え）"
                        >
                          {i + 1}
                        </span>
                      )}
                      <TaskName
                        task={t}
                        onOpen={onOpen}
                        clamp={!isToday}
                        className={t.state === "skipped" ? "line-through text-muted-foreground" : undefined}
                      />
                      <Badges task={t} />
                      {!isToday && (
                        <Button variant="ghost" size="sm" disabled={busy} onClick={() => onPromoteToday(t)}>
                          今日へ
                        </Button>
                      )}
                      <DeleteButton busy={busy} onClick={() => onDelete(t)} />
                    </SortableRow>
                  ))}
                </div>
              </SortableContext>
            )}
          </div>
        </CardBody>
      )}
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

function SortableRow({ task, band, busy, children }: { task: Task; band: Band; busy: boolean; children: ReactNode }) {
  const { setNodeRef, attributes, listeners, transform, transition, isDragging } = useSortable({
    id: key(task),
    data: { task, band },
    disabled: busy,
  });
  return (
    <div ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition }} className={rowClass(isDragging)}>
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
