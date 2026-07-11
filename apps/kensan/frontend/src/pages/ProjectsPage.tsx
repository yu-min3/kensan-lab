import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams, Link } from "react-router-dom";
import {
  FolderKanban,
  CheckSquare,
  Square,
  MinusSquare,
  ExternalLink,
  Calendar,
  Target,
  Plus,
  Pencil,
  Trash2,
  Check,
  X,
} from "lucide-react";
import clsx from "clsx";
import { api, ApiError, todayISO, type Doc, type ProjectSummary, type ProjectDetail, type Task } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { MilkdownEditor } from "../components/editors/MilkdownEditor";
import { useAutosaveFile } from "../hooks/useAutosaveFile";
import { Card, CardBody } from "../components/ui/card";
import { SaveStatus } from "../components/ui/save-status";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Empty, ErrorState, SkeletonRows, Skeleton } from "../components/ui/states";

const STATUSES = ["active", "paused", "completed", "cancelled"];

// プロジェクト一覧 + 詳細（List·Detail）。マイルストーン操作・メタ編集・新規作成に対応。
export function ProjectsPage() {
  const [params, setParams] = useSearchParams();
  const selected = params.get("name");
  const qc = useQueryClient();
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.projects });
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");

  const create = useMutation({
    mutationFn: (name: string) => api.createProject(name),
    onSuccess: (res) => {
      setCreating(false);
      setNewName("");
      qc.invalidateQueries({ queryKey: ["projects"] });
      setParams({ name: res.name });
    },
  });

  return (
    <>
      <PageHeader
        eyebrow="プロジェクト · projects/"
        title="プロジェクト"
        sub="目標・マイルストーン・タスク・ログを構造化して見る・編集する。中身は各 README が正本。"
      />
      <div className="grid grid-cols-1 lg:grid-cols-[300px_1fr] gap-6 items-start">
        <Card>
          <div className="ds-card flex items-center justify-between border-b border-border !pb-2">
            <span className="text-xs text-muted-foreground">{(projects.data?.projects ?? []).length} 件</span>
            <Button variant="ghost" size="sm" onClick={() => setCreating((v) => !v)}>
              <Plus size={14} />
              新規
            </Button>
          </div>
          <CardBody className="!p-2">
            {creating && (
              <div className="flex gap-1 p-1">
                <input
                  autoFocus
                  className="flex-1 min-w-0 rounded-md border border-border bg-card px-2 h-8 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  placeholder="project-name（英小文字・-）"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && newName.trim()) create.mutate(newName.trim());
                    if (e.key === "Escape") setCreating(false);
                  }}
                />
                <Button variant="primary" size="sm" loading={create.isPending} disabled={!newName.trim()} onClick={() => create.mutate(newName.trim())}>
                  作成
                </Button>
              </div>
            )}
            {create.isError && (
              <p className="text-xs text-destructive px-2 pb-1">{String((create.error as Error).message)}</p>
            )}
            {projects.isPending ? (
              <SkeletonRows rows={6} />
            ) : projects.isError ? (
              <ErrorState error={projects.error} onRetry={() => projects.refetch()} />
            ) : (projects.data.projects ?? []).length === 0 ? (
              <Empty icon={<FolderKanban />} title="プロジェクトがありません" desc="「+ 新規」で作成できます。" />
            ) : (
              <ul className="ds-stack !gap-1">
                {(projects.data.projects ?? []).map((p) => (
                  <li key={p.name}>
                    <ProjectRow project={p} active={selected === p.name} onClick={() => setParams({ name: p.name })} />
                  </li>
                ))}
              </ul>
            )}
          </CardBody>
        </Card>

        {selected ? (
          <ProjectDetailView key={selected} name={selected} />
        ) : (
          <Card className="min-h-[40vh]">
            <CardBody>
              <Empty icon={<FolderKanban />} title="プロジェクトを選択してください" desc="左の一覧から開くと、構造化表示＋編集ができます。" />
            </CardBody>
          </Card>
        )}
      </div>
    </>
  );
}

function ProjectRow({ project, active, onClick }: { project: ProjectSummary; active: boolean; onClick: () => void }) {
  const { milestonesDone: md, milestonesTotal: mt } = project;
  return (
    <button
      onClick={onClick}
      className={clsx("w-full text-left rounded-md px-2 py-2 transition-colors", active ? "bg-accent text-accent-foreground" : "hover:bg-accent/60")}
    >
      <div className="flex items-center gap-2">
        <StatusBadge status={project.status} />
        <span className="flex-1 truncate text-sm font-medium">{project.name}</span>
      </div>
      <div className="mt-1.5 flex items-center gap-2">
        <ProgressBar done={md} total={mt} />
        <span className="font-mono tnum text-[10px] text-muted-foreground shrink-0">{md}/{mt}</span>
      </div>
      <div className="mt-1 flex items-center gap-2 text-[10px] text-muted-foreground">
        <span>未完 {project.openTasks}</span>
        {project.deadline && <DeadlineText deadline={project.deadline} />}
      </div>
    </button>
  );
}

function ProjectDetailView({ name }: { name: string }) {
  const qc = useQueryClient();
  const detail = useQuery({ queryKey: ["project", name], queryFn: () => api.projectDetail(name) });
  const taggedNotes = useQuery({ queryKey: ["files", "note", name], queryFn: () => api.files({ type: "note", tag: name }) });
  const file = `projects/${name}/README.md`;
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["project", name] });
    qc.invalidateQueries({ queryKey: ["projects"] });
    qc.invalidateQueries({ queryKey: ["board"] });
  };

  const toggleMs = useMutation({ mutationFn: (m: Task) => api.setTaskState(m, m.state === "done" ? "todo" : "done"), onSettled: invalidate });
  const editMs = useMutation({ mutationFn: ({ m, text }: { m: Task; text: string }) => api.setText(m, text), onSettled: invalidate });
  const delMs = useMutation({ mutationFn: (m: Task) => api.deleteTask(m), onSettled: invalidate });
  const addMs = useMutation({ mutationFn: (display: string) => api.addLine(file, "マイルストーン", display), onSettled: invalidate });
  const dueMs = useMutation({ mutationFn: ({ m, due }: { m: Task; due: string }) => api.setDue(m, due), onSettled: invalidate });
  const saveMeta = useMutation({
    mutationFn: (input: { status: string; deadline: string; goal: string }) => api.updateProject(name, input),
    onSuccess: () => setEditingMeta(false),
    onSettled: invalidate,
  });

  const [editingMeta, setEditingMeta] = useState(false);
  const [msInput, setMsInput] = useState("");
  const [msDueInput, setMsDueInput] = useState("");
  const [editingMs, setEditingMs] = useState<string | null>(null);
  const [msText, setMsText] = useState("");
  const [editingDue, setEditingDue] = useState<string | null>(null);

  if (detail.isPending) {
    return <Card><CardBody><Skeleton className="h-64 w-full" /></CardBody></Card>;
  }
  if (detail.isError) {
    return <Card><CardBody><ErrorState error={detail.error} onRetry={() => detail.refetch()} /></CardBody></Card>;
  }

  const d = detail.data;
  const milestones = d.milestones ?? [];
  // 期限の昇順（期限なしは後ろ）→ 行順
  const byDue = (a: Task, b: Task) => {
    const ad = a.due ?? "";
    const bd = b.due ?? "";
    if ((ad === "") !== (bd === "")) return ad === "" ? 1 : -1;
    if (ad !== bd) return ad < bd ? -1 : 1;
    return a.line - b.line;
  };
  const openMs = milestones.filter((m) => m.state !== "done").sort(byDue);
  const doneMs = milestones.filter((m) => m.state === "done").sort(byDue);
  const openTasks = (d.tasks ?? []).filter((t) => t.state === "todo");
  const busy = toggleMs.isPending || editMs.isPending || delMs.isPending || addMs.isPending || dueMs.isPending;

  const addMilestone = () => {
    if (!msInput.trim()) return;
    addMs.mutate(msDueInput ? `${msInput.trim()} @due(${msDueInput})` : msInput.trim());
    setMsInput("");
    setMsDueInput("");
  };

  const renderMs = (m: Task) => {
    const k = `${m.file}:${m.line}`;
    const done = m.state === "done";
    return (
      <li key={k} className="flex items-center gap-2 text-sm group">
        <button onClick={() => toggleMs.mutate(m)} disabled={busy} aria-label="完了切替">
          <StateIcon state={m.state} />
        </button>
        {editingMs === k ? (
          <input
            autoFocus
            className="flex-1 rounded-md border border-border bg-card px-2 py-0.5 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={msText}
            onChange={(e) => setMsText(e.target.value)}
            onBlur={() => {
              if (msText.trim() && msText.trim() !== m.display) editMs.mutate({ m, text: msText.trim() });
              setEditingMs(null);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.nativeEvent.isComposing) (e.target as HTMLInputElement).blur();
              if (e.key === "Escape") setEditingMs(null);
            }}
          />
        ) : (
          <span
            className={clsx("flex-1 cursor-text hover:text-brand", m.state !== "todo" && "line-through text-muted-foreground")}
            onClick={() => {
              setEditingMs(k);
              setMsText(m.display);
            }}
          >
            {m.display}
          </span>
        )}
        <span className="relative shrink-0">
          {m.due ? (
            <button onClick={() => setEditingDue(k)} disabled={busy} title="期限を変更">
              <MilestoneDue due={m.due} done={done} />
            </button>
          ) : (
            <button
              onClick={() => setEditingDue(k)}
              disabled={busy}
              title="期限を設定"
              className="inline-flex items-center gap-1 rounded-md border border-dashed border-border px-1.5 h-6 text-xs text-muted-foreground hover:text-foreground hover:border-border-strong"
            >
              <Calendar size={12} /> 日付
            </button>
          )}
          {editingDue === k && (
            <DuePicker
              value={m.due ?? ""}
              onSet={(due) => {
                dueMs.mutate({ m, due });
                setEditingDue(null);
              }}
              onClose={() => setEditingDue(null)}
            />
          )}
        </span>
        {done && <Badge variant="success">完了</Badge>}
        <button
          className="size-6 grid place-items-center rounded-md text-destructive hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
          disabled={busy}
          onClick={() => delMs.mutate(m)}
          aria-label="削除"
        >
          <Trash2 size={13} />
        </button>
      </li>
    );
  };

  return (
    <Card>
      <CardBody className="ds-section">
        {/* タイトル */}
        <div className="flex items-center gap-2 min-w-0">
          <StatusBadge status={d.status} />
          <h2 className="h-serif text-xl font-bold truncate">{d.name}</h2>
        </div>

        {/* メタ */}
        {editingMeta ? (
          <MetaEditor d={d} busy={saveMeta.isPending} onCancel={() => setEditingMeta(false)} onSave={(v) => saveMeta.mutate(v)} />
        ) : (
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              {d.deadline && <DeadlineText deadline={d.deadline} withLabel />}
              {d.repo && (
                <a href={d.repo} target="_blank" rel="noreferrer" className="text-xs text-brand hover:underline inline-flex items-center gap-1">
                  repo <ExternalLink size={11} />
                </a>
              )}
              <Button variant="ghost" size="sm" className="ml-auto" onClick={() => setEditingMeta(true)}>
                <Pencil size={13} />
                メタ編集
              </Button>
            </div>
            {d.goal && (
              <p className="mt-3 h-serif text-base font-semibold leading-snug flex items-start gap-2">
                <Target size={18} className="text-brand mt-0.5 shrink-0" />
                {d.goal}
              </p>
            )}
          </div>
        )}

        {/* 進捗 */}
        <Section title="進捗">
          <div className="flex items-center gap-4 text-sm">
            <div className="flex items-center gap-2 flex-1 max-w-xs">
              <ProgressBar done={doneMs.length} total={milestones.length} />
              <span className="font-mono tnum text-xs text-muted-foreground shrink-0">MS {doneMs.length}/{milestones.length}</span>
            </div>
            <span className="text-muted-foreground">未完タスク {openTasks.length}</span>
          </div>
        </Section>

        {/* マイルストーン（未完了を上・完了を区切り線の下にまとめる） */}
        <Section title="マイルストーン">
          {milestones.length === 0 && <p className="text-sm text-muted-foreground">まだありません。下から追加できます。</p>}
          {openMs.length > 0 && <ul className="ds-stack !gap-1">{openMs.map(renderMs)}</ul>}
          {doneMs.length > 0 && (
            <>
              <div className="flex items-center gap-2 mt-3 mb-1">
                <span className="text-[10px] uppercase tracking-[0.12em] text-success">完了 {doneMs.length}</span>
                <div className="flex-1 h-px bg-border" />
              </div>
              <ul className="ds-stack !gap-1 opacity-65">{doneMs.map(renderMs)}</ul>
            </>
          )}
          <div className="flex gap-1 mt-3">
            <input
              className="flex-1 rounded-md border border-border bg-card px-2 h-8 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              placeholder="マイルストーンを追加…"
              value={msInput}
              onChange={(e) => setMsInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.nativeEvent.isComposing && msInput.trim()) addMilestone();
              }}
            />
            <input
              type="date"
              className="rounded-md border border-border bg-card px-1 h-8 text-xs text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring shrink-0"
              title="期限（任意）"
              value={msDueInput}
              onChange={(e) => setMsDueInput(e.target.value)}
            />
            <Button variant="secondary" size="sm" loading={addMs.isPending} disabled={!msInput.trim()} onClick={addMilestone}>
              <Plus size={14} />
            </Button>
          </div>
        </Section>

        {/* 進行中タスク（編集はタスクボード） */}
        {openTasks.length > 0 && (
          <Section title={`進行中タスク (${openTasks.length})`}>
            <ul className="ds-stack !gap-1">
              {openTasks.map((t) => (
                <li key={`${t.file}:${t.line}`} className="flex items-center gap-2 text-sm">
                  <span className="size-1.5 rounded-full bg-warning shrink-0" />
                  <span className="flex-1">{t.display}</span>
                  <TaskTags task={t} />
                </li>
              ))}
            </ul>
          </Section>
        )}

        {/* フリースペース（README の ## フリースペース を md 自由編集） */}
        <Section title="フリースペース">
          <FreeSpace name={name} onSaved={invalidate} />
        </Section>

        {/* ログ（日付ごとのタイムライン） */}
        {(d.log ?? []).length > 0 && (
          <Section title="ログ">
            <Timeline entries={d.log ?? []} />
          </Section>
        )}

        {/* タグ付きノート（notes/ の frontmatter tags に project 名） */}
        <Section title={`タグ付きノート #${name}`}>
          {(taggedNotes.data?.files ?? []).length > 0 ? (
            <NoteLinkList docs={taggedNotes.data?.files ?? []} />
          ) : (
            <p className="text-xs text-muted-foreground">
              ノートの frontmatter <code className="px-1 rounded bg-muted">tags</code> に{" "}
              <code className="px-1 rounded bg-muted">{name}</code> を足すと、ここに自動で並びます。
            </p>
          )}
        </Section>
      </CardBody>
    </Card>
  );
}

// タグ付きノートのリンク一覧。長寿プロジェクトでは 40 件超の「リンクの壁」になるため、
// 既定は先頭 8 件 + 「すべて表示」で折りたたむ（新しい順が上に来る前提の一覧をそのまま使う）。
const NOTE_LIST_COLLAPSED = 8;
function NoteLinkList({ docs }: { docs: Doc[] }) {
  const [expanded, setExpanded] = useState(false);
  const visible = expanded ? docs : docs.slice(0, NOTE_LIST_COLLAPSED);
  return (
    <>
      <ul className="ds-stack !gap-1">
        {visible.map((doc) => (
          <li key={doc.path} className="text-sm">
            <Link to={`/notes?path=${encodeURIComponent(doc.path)}`} className="text-brand hover:underline">
              {doc.meta.title || doc.path.replace(/^notes\//, "").replace(/\.md$/, "")}
            </Link>
          </li>
        ))}
      </ul>
      {docs.length > NOTE_LIST_COLLAPSED && (
        <button
          className="mt-2 text-xs text-muted-foreground hover:text-foreground"
          onClick={() => setExpanded(!expanded)}
          aria-expanded={expanded}
        >
          {expanded ? "折りたたむ" : `すべて表示（${docs.length} 件）`}
        </button>
      )}
    </>
  );
}

// README 内の ## 見出し セクションの境界を返す（同レベル以上の見出しで終端）。
const HEADING_RE = /^(#{1,6})\s+(.+?)\s*$/;
function sectionBounds(lines: string[], heading: string): { start: number; end: number; level: number } | null {
  let start = -1;
  let level = 0;
  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(HEADING_RE);
    if (m && m[2] === heading) {
      start = i;
      level = m[1].length;
      break;
    }
  }
  if (start < 0) return null;
  let end = lines.length;
  for (let i = start + 1; i < lines.length; i++) {
    const m = lines[i].match(HEADING_RE);
    if (m && m[1].length <= level) {
      end = i;
      break;
    }
  }
  return { start, end, level };
}
function extractSection(content: string, heading: string): string {
  const lines = content.split("\n");
  const b = sectionBounds(lines, heading);
  return b ? lines.slice(b.start + 1, b.end).join("\n").trim() : "";
}
function spliceSection(content: string, heading: string, body: string): string {
  const lines = content.split("\n");
  const b = sectionBounds(lines, heading);
  const trimmed = body.trim();
  if (!b) {
    return content.replace(/\n*$/, "") + `\n\n## ${heading}\n\n${trimmed}\n`;
  }
  const mid = trimmed ? ["", trimmed, ""] : [""];
  return [...lines.slice(0, b.start + 1), ...mid, ...lines.slice(b.end)].join("\n");
}

const FREE_HEADING = "フリースペース";

// README の ## フリースペース セクションだけを md 自由編集（autosave）。
// README 全文は files API で読み書きし、該当セクションだけ差し替える（他セクションは保持）。
// デバウンス保存・楽観ロック・離脱時 flush は useAutosaveFile に集約。
function FreeSpace({ name, onSaved }: { name: string; onSaved: () => void }) {
  const path = `projects/${name}/README.md`;
  const file = useAutosaveFile({
    path,
    read: (c) => extractSection(c, FREE_HEADING),
    write: (c, b) => spliceSection(c, FREE_HEADING, b),
    onSaved,
  });

  if (file.query.isError) return <ErrorState error={file.query.error} onRetry={() => file.query.refetch()} />;
  if (file.conflict) {
    return (
      <ErrorState
        error={new ApiError(409, "他のクライアント（Claude Code / VSCode）が先に編集しました。再読込してください。")}
        onRetry={file.retry}
      />
    );
  }
  if (file.query.isPending || file.initialBody === null) return <Skeleton className="h-40 w-full" />;

  return (
    <div className="ds-stack !gap-2">
      <div className="flex justify-end">
        <SaveStatus state={file.saveState} />
      </div>
      <div className="rounded-md border border-border bg-card p-3">
        <MilkdownEditor
          key={file.editorKey}
          defaultValue={file.initialBody}
          onChange={file.onChange}
          minHeight="14rem"
          placeholder="このプロジェクトの自由メモ。設計の走り書き・関連リンク（[[…]] や URL）・なんでも。"
        />
      </div>
    </div>
  );
}

function MetaEditor({
  d,
  busy,
  onCancel,
  onSave,
}: {
  d: ProjectDetail;
  busy: boolean;
  onCancel: () => void;
  onSave: (v: { status: string; deadline: string; goal: string }) => void;
}) {
  const [status, setStatus] = useState(d.status);
  const [deadline, setDeadline] = useState(d.deadline ?? "");
  const [goal, setGoal] = useState(d.goal);
  return (
    <div className="ds-stack !gap-3 border border-border rounded-lg p-3">
      <div className="flex items-center gap-2">
        <h2 className="h-serif text-lg font-bold flex-1">{d.name}</h2>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <label className="block">
          <span className="text-xs text-muted-foreground">status</span>
          <select
            className="mt-1 w-full rounded-md border border-border bg-card px-2 h-9 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
          >
            {STATUSES.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-xs text-muted-foreground">締切</span>
          <input
            type="date"
            className="mt-1 w-full rounded-md border border-border bg-card px-2 h-9 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={deadline}
            onChange={(e) => setDeadline(e.target.value)}
          />
        </label>
      </div>
      <label className="block">
        <span className="text-xs text-muted-foreground">目標</span>
        <textarea
          rows={2}
          className="mt-1 w-full resize-y rounded-md border border-border bg-card px-3 py-1.5 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          value={goal}
          onChange={(e) => setGoal(e.target.value)}
        />
      </label>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onCancel}>
          <X size={14} />
          キャンセル
        </Button>
        <Button variant="primary" size="sm" loading={busy} onClick={() => onSave({ status, deadline, goal })}>
          <Check size={14} />
          保存
        </Button>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground pb-2 border-b border-border mb-3">{title}</h3>
      {children}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const variant = status === "active" ? "success" : status === "paused" ? "warning" : "muted";
  return <Badge variant={variant}>{status || "—"}</Badge>;
}

function ProgressBar({ done, total }: { done: number; total: number }) {
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  return (
    <div className="h-1.5 flex-1 rounded-full bg-muted overflow-hidden">
      <div className="h-full bg-brand rounded-full" style={{ width: `${pct}%` }} />
    </div>
  );
}

function DeadlineText({ deadline, withLabel }: { deadline: string; withLabel?: boolean }) {
  const overdue = deadline < todayISO();
  return (
    <span className={clsx("inline-flex items-center gap-0.5", overdue ? "text-destructive" : "text-muted-foreground")}>
      <Calendar size={11} />
      <span className="font-mono tnum">{withLabel ? `締切 ${deadline}` : deadline.slice(5)}</span>
    </span>
  );
}

// 日付ピッカー（ポップオーバー）。数ヶ月先はクイックチップで一発、正確指定は日付入力で。
const pad2 = (n: number) => String(n).padStart(2, "0");
function isoOffset(days: number, months: number): string {
  const d = new Date();
  if (months) d.setMonth(d.getMonth() + months);
  if (days) d.setDate(d.getDate() + days);
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}

function DuePicker({ value, onSet, onClose }: { value: string; onSet: (d: string) => void; onClose: () => void }) {
  const chips: [string, number, number][] = [
    ["今日", 0, 0],
    ["+1週", 7, 0],
    ["+2週", 14, 0],
    ["+1ヶ月", 0, 1],
    ["+3ヶ月", 0, 3],
    ["+半年", 0, 6],
    ["+1年", 0, 12],
  ];
  return (
    <>
      <div className="fixed inset-0 z-30" onClick={onClose} />
      <div className="absolute right-0 top-7 z-40 w-60 bg-card border border-border rounded-lg shadow-lg p-2 ds-stack !gap-2">
        <div className="flex flex-wrap gap-1">
          {chips.map(([label, days, months]) => (
            <button
              key={label}
              className="rounded-md border border-border px-2 h-7 text-xs hover:bg-accent/60"
              onClick={() => onSet(isoOffset(days, months))}
            >
              {label}
            </button>
          ))}
        </div>
        <input
          type="date"
          autoFocus
          className="w-full rounded-md border border-border bg-card px-2 h-8 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          value={value}
          onChange={(e) => e.target.value && onSet(e.target.value)}
        />
        <div className="flex items-center justify-between">
          {value ? (
            <button className="text-xs text-destructive hover:underline" onClick={() => onSet("")}>
              期限をクリア
            </button>
          ) : (
            <span />
          )}
          <button className="text-xs text-muted-foreground hover:text-foreground" onClick={onClose}>
            閉じる
          </button>
        </div>
      </div>
    </>
  );
}

// マイルストーンの期限バッジ。未完で過ぎていたら赤（遅延）。
function MilestoneDue({ due, done }: { due: string; done: boolean }) {
  const overdue = !done && due < todayISO();
  return (
    <Badge variant={overdue ? "destructive" : "muted"}>
      <Calendar size={11} className="mr-0.5" />
      <span className="font-mono tnum">{due.slice(5)}</span>
    </Badge>
  );
}

function StateIcon({ state }: { state: Task["state"] }) {
  if (state === "done") return <CheckSquare size={15} className="text-success shrink-0" />;
  if (state === "skipped") return <MinusSquare size={15} className="text-muted-foreground shrink-0" />;
  return <Square size={15} className="text-muted-foreground shrink-0" />;
}

function TaskTags({ task }: { task: Task }) {
  return (
    <>
      {task.due && (
        <Badge variant={task.due < todayISO() ? "destructive" : "muted"}>
          <span className="font-mono tnum">{task.due.slice(5)}</span>
        </Badge>
      )}
      {task.today && <Badge variant="brand">今日</Badge>}
      {task.milestone && <Badge variant="outline">▸ {task.milestone}</Badge>}
    </>
  );
}

// 最小限のインライン markdown（**太字** と `コード` のみ）を React ノードへ。
function inlineMd(text: string): React.ReactNode[] {
  const out: React.ReactNode[] = [];
  const re = /\*\*([^*]+)\*\*|`([^`]+)`/g;
  let last = 0;
  let key = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index));
    if (m[1]) out.push(<strong key={key++}>{m[1]}</strong>);
    else out.push(<code key={key++} className="px-1 rounded bg-muted text-xs font-mono">{m[2]}</code>);
    last = re.lastIndex;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

const LOG_LIMIT = 10;

function Timeline({ entries }: { entries: { date?: string; text: string }[] }) {
  const [all, setAll] = useState(false);
  const shown = all ? entries : entries.slice(0, LOG_LIMIT);
  return (
    <>
      <ul className="ds-stack !gap-3">
        {shown.map((e, i) => (
          <li key={i} className="border-l-2 border-border pl-3">
            {e.date && <div className="font-mono tnum text-xs text-brand">{e.date}</div>}
            <p className="text-sm text-muted-foreground leading-relaxed mt-0.5">{inlineMd(e.text)}</p>
          </li>
        ))}
      </ul>
      {entries.length > LOG_LIMIT && (
        <button className="text-xs text-brand hover:underline mt-2" onClick={() => setAll((v) => !v)}>
          {all ? "折りたたむ" : `他 ${entries.length - LOG_LIMIT} 件を表示`}
        </button>
      )}
    </>
  );
}
