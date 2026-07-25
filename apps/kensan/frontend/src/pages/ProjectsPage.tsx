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
  RefreshCw,
  TrendingUp,
  AlertTriangle,
  Minus,
} from "lucide-react";
import clsx from "clsx";
import { api, ApiError, todayISO, type Doc, type ProjectSummary, type ProjectDetail, type ProjectMetric, type Task } from "../lib/api";
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
  const metrics = useQuery({ queryKey: ["project-metrics", name], queryFn: () => api.projectMetrics(name) });
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
  const refreshMetrics = useMutation({
    mutationFn: () => api.refreshProjectMetrics(name),
    onSuccess: (data) => qc.setQueryData(["project-metrics", name], data),
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
    // Journey の連番と対応させる（一覧は未完を上に並べ替えるため、番号が無いと
    // どのステップの話か分からなくなる）。番号は README の並び順。
    const step = String(milestones.indexOf(m) + 1).padStart(2, "0");
    return (
      <li key={k} className="flex items-center gap-2 text-sm group">
        <span className="shrink-0 font-mono tnum text-[11px] text-muted-foreground w-5">{step}</span>
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
            className={clsx("flex-1 cursor-text hover:text-brand", m.state !== "todo" && "text-muted-foreground")}
            onClick={() => {
              setEditingMs(k);
              setMsText(m.display);
            }}
          >
            {cleanLabel(m.display)}
          </span>
        )}
        {/* 完了行は完了日だけを静かに出す。期限チップと 完了 バッジを重ねると
            1 行に日付が 2 つ並んで読みにくいため（未完了行は従来どおり期限を出す）。 */}
        {done && (
          <span className="shrink-0 font-mono tnum text-[11px] text-success">
            {doneDate(m) ? `完了 ${doneDate(m)!.slice(5)}` : "完了"}
          </span>
        )}
        <span className="relative shrink-0">
          {m.due ? (
            <button onClick={() => setEditingDue(k)} disabled={busy} title="期限を変更" className={clsx(done && "opacity-0 group-hover:opacity-100 transition-opacity")}>
              <MilestoneDue due={m.due} done={done} />
            </button>
          ) : (
            <button
              onClick={() => setEditingDue(k)}
              disabled={busy}
              title="期限を設定"
              className="inline-flex items-center gap-1 rounded-md border border-dashed border-border px-1.5 h-6 text-xs text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity hover:text-foreground hover:border-border-strong"
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
    // min-w-0: Journey の min-width が親グリッドの 1fr トラックを押し広げて
    // ページ全体が横スクロールするのを防ぐ（横に溢れるのは Journey の中だけにする）
    <Card className="min-w-0">
      <CardBody className="ds-section min-w-0">
        {/* Hero — 目標・現在地・状態と、いま見るべき 1 つの数字を先頭に固定する */}
        {editingMeta ? (
          <>
            <div className="flex items-center gap-2 min-w-0">
              <StatusBadge status={d.status} />
              <h2 className="h-serif text-xl font-bold truncate">{d.name}</h2>
            </div>
            <MetaEditor d={d} busy={saveMeta.isPending} onCancel={() => setEditingMeta(false)} onSave={(v) => saveMeta.mutate(v)} />
          </>
        ) : (
          <ProjectHero
            d={d}
            milestones={milestones}
            doneMs={doneMs}
            openMs={openMs}
            openTasks={openTasks}
            metrics={metrics.data?.metrics ?? []}
            onEditMeta={() => setEditingMeta(true)}
          />
        )}

        {/* Pulse — Hero の要約に対する根拠。直近の前進と、次に手をつけるものを並べる */}
        <ProjectPulse
          d={d}
          milestones={milestones}
          doneMs={doneMs}
          openMs={openMs}
          openTasks={openTasks}
          metrics={metrics.data?.metrics ?? []}
          metricsError={metrics.error ?? refreshMetrics.error}
          refreshing={refreshMetrics.isPending}
          onRefresh={() => refreshMetrics.mutate()}
        />

        {/* Journey は checkpoint を持つ指標のときだけ独立セクションにする。
            マイルストーン版の Journey は「マイルストーン」節の先頭に置く（同じ情報を 2 箇所に出さない）。
            どちらも無い project は Horizons。 */}
        {(metrics.data?.metrics ?? [])[0]?.checkpoints?.length ? (
          <ProjectJourney
            metric={(metrics.data?.metrics ?? [])[0]}
            hint={d.deadline ? `deadline ${d.deadline}` : undefined}
          />
        ) : milestones.length === 0 ? (
          <ProjectHorizons openTasks={openTasks} />
        ) : null}

        {/* 主指標以外のメトリクス（1 つだけなら Hero に出ているのでここは空） */}
        <ProjectMetrics
          metrics={(metrics.data?.metrics ?? []).slice(1)}
          loading={false}
          error={null}
          onRetry={() => metrics.refetch()}
        />

        {/* マイルストーン（未完了を上・完了を区切り線の下にまとめる） */}
        <Section title="Milestones" sub="到達点と進み具合" hint={d.deadline ? `deadline ${d.deadline}` : undefined}>
          {milestones.length === 0 && <p className="text-sm text-muted-foreground">まだありません。下から追加できます。</p>}
          {milestones.length > 0 && (
            <div className="mb-4">
              <JourneyLine steps={milestoneSteps(milestones)} />
            </div>
          )}
          {openMs.length > 0 && <ul className="ds-stack !gap-1">{openMs.map(renderMs)}</ul>}
          {doneMs.length > 0 && (
            <>
              <div className="flex items-center gap-2 mt-3 mb-1">
                <span className="text-[11px] font-semibold tracking-wide text-success">完了 {doneMs.length}</span>
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
                  <span className="flex-1">{inlineMd(t.display)}</span>
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

        {/* 関連ドキュメント（README ## 関連ノート・リソース。roadmap 俯瞰などへの導線） */}
        {(d.notes ?? []).length > 0 && (
          <Section title="関連ドキュメント">
            <ul className="ds-stack !gap-1">
              {(d.notes ?? []).map((n, i) => (
                <li key={i} className="text-sm">
                  {n.target ? (
                    <Link to={`/notes?path=${encodeURIComponent(n.target)}`} className="text-brand hover:underline">
                      {n.label}
                    </Link>
                  ) : (
                    <span>{n.label}</span>
                  )}
                  {n.desc && <span className="text-muted-foreground"> — {n.desc}</span>}
                </li>
              ))}
            </ul>
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

function ProjectMetrics({
  metrics,
  loading,
  error,
  onRetry,
}: {
  metrics: ProjectMetric[];
  loading: boolean;
  error: Error | null;
  onRetry: () => void;
}) {
  if (loading) return <Section title="Metrics"><Skeleton className="h-28 w-full" /></Section>;
  if (error && metrics.length === 0) return <Section title="Metrics"><ErrorState error={error} onRetry={onRetry} /></Section>;
  if (metrics.length === 0) return null;
  return (
    <Section title="Metrics" sub="そのほかの指標">
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
        {metrics.map((metric) => <MetricCard key={metric.id} metric={metric} />)}
      </div>
    </Section>
  );
}

// ---- Hero -------------------------------------------------------------
// 「どこへ向かうか（目標）→ 今どこか（現在地・状態）→ いま見るべき数字」を
// 1 ブロックに固定する。状態は色だけでなく必ずラベルと根拠を併記する。

type HeroTone = "success" | "warn" | "muted";
// why = Hero バッジ横に出す一行の根拠。sentence = Pulse に出す説明文。
type HeroState = { label: string; tone: HeroTone; why: string; sentence: string };

const DAY_MS = 86_400_000;

function daysFromToday(iso: string): number {
  const target = new Date(`${iso}T00:00:00`);
  const today = new Date(todayISO() + "T00:00:00");
  return Math.round((target.getTime() - today.getTime()) / DAY_MS);
}

// README ログの日付・完了マイルストーンの ✅ 日付・メトリクス更新日のうち最も新しいもの。
function lastActivity(d: ProjectDetail, metrics: ProjectMetric[]): string | undefined {
  const dates: string[] = [];
  for (const entry of d.log ?? []) if (entry.date) dates.push(entry.date);
  for (const m of d.milestones ?? []) {
    const hit = /✅\s*(\d{4}-\d{2}-\d{2})/.exec(m.text);
    if (hit) dates.push(hit[1]);
  }
  for (const m of metrics) if (m.updatedAt) dates.push(m.updatedAt.slice(0, 10));
  return dates.sort().at(-1);
}

// 状態は決定的なルールだけで決める（表示経路に LLM を入れない）。
function heroState(d: ProjectDetail, openMs: Task[], openTasks: Task[], metrics: ProjectMetric[]): HeroState {
  const overdue = openTasks.filter((t) => t.due && daysFromToday(t.due) < 0);
  const toDeadline = d.deadline ? daysFromToday(d.deadline) : undefined;
  const activity = lastActivity(d, metrics);
  const sinceActivity = activity ? -daysFromToday(activity) : undefined;

  if (toDeadline !== undefined && toDeadline >= 0 && toDeadline <= 14 && openMs.length > 0) {
    return {
      label: "仕上げの時期", tone: "warn",
      why: `締切まで ${toDeadline} 日・未完マイルストーン ${openMs.length}`,
      sentence: `締切まで ${toDeadline} 日。未完のマイルストーンが ${openMs.length} 件残っています。`,
    };
  }
  if (overdue.length > 0) {
    return {
      label: "要整理", tone: "warn",
      why: `期限切れタスク ${overdue.length} 件`,
      sentence: `期限切れのタスクが ${overdue.length} 件。まず滞留を片付けると動きが戻ります。`,
    };
  }
  if (sinceActivity !== undefined && sinceActivity <= 30) {
    return {
      label: "前進中", tone: "success",
      why: `${activity} に前進`,
      sentence: sinceActivity === 0 ? "今日前進しました。" : `${sinceActivity} 日前に前進しています。`,
    };
  }
  if (sinceActivity !== undefined) {
    return (d.milestones ?? []).length === 0
      ? { label: "安定運用", tone: "muted", why: `直近の記録は ${activity}`, sentence: `終点を置かない project です。直近の記録は ${activity}。` }
      : { label: "再開待ち", tone: "muted", why: `${sinceActivity} 日動きなし`, sentence: `${sinceActivity} 日動きがありません。小さく再開できる一手を選びます。` };
  }
  return { label: "観測を開始", tone: "muted", why: "ログもメトリクスもまだありません", sentence: "ログもメトリクスもまだありません。まず記録を 1 つ残すところから。" };
}

const TONE_CLASS: Record<HeroTone, string> = {
  success: "border-success/40 bg-success/10 text-success",
  warn: "border-warning/40 bg-warning/10 text-warning",
  muted: "border-border bg-muted text-muted-foreground",
};

function ProjectHero({
  d, milestones, doneMs, openMs, openTasks, metrics, onEditMeta,
}: {
  d: ProjectDetail;
  milestones: Task[];
  doneMs: Task[];
  openMs: Task[];
  openTasks: Task[];
  metrics: ProjectMetric[];
  onEditMeta: () => void;
}) {
  const state = heroState(d, openMs, openTasks, metrics);
  const staleDays = d.current.date ? -daysFromToday(d.current.date) : undefined;
  return (
    // 1 カラム。かつて右側に置いていた数値カードは、現在値 / 次の到達点を Pulse が、
    // 進み具合を Journey・Milestones が持っており、3 箇所で同じことを言っていたので廃止した。
    <div className="ds-stack !gap-3 min-w-0">
        <div className="flex items-center gap-2">
          <StatusBadge status={d.status} />
          <span className={clsx("shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold", TONE_CLASS[state.tone])}>
            {state.label}
          </span>
          {/* 根拠は幅が足りなければ縮める。メタ編集ボタンを追い出して折り返さない */}
          <span className="min-w-0 flex-1 truncate text-[11px] text-muted-foreground">{state.why}</span>
          <Button variant="ghost" size="sm" className="shrink-0" onClick={onEditMeta}>
            <Pencil size={13} />
            メタ編集
          </Button>
        </div>

        <h2 className="h-serif text-xl font-bold truncate">{d.name}</h2>

        {d.goal && (
          <p className="h-serif text-base font-semibold leading-snug flex items-start gap-2">
            <Target size={18} className="text-brand mt-0.5 shrink-0" />
            {d.goal}
          </p>
        )}

        {d.current.text && (
          <div className="rounded-md border border-border bg-accent/30 px-3 py-2">
            <div className="mb-1 flex items-center gap-2 text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
              <span>現在地</span>
              {d.current.date && (
                <span className={clsx("font-mono tnum normal-case tracking-normal", staleDays !== undefined && staleDays > 30 && "text-warning font-semibold")}>
                  {d.current.date}
                  {staleDays !== undefined && staleDays > 30 && `（${staleDays} 日前・要棚卸し）`}
                </span>
              )}
            </div>
            <p className="text-sm leading-relaxed">{inlineMd(d.current.text)}</p>
          </div>
        )}

        <div className="flex items-center gap-3 flex-wrap text-[11px] text-muted-foreground">
          {d.deadline && <DeadlineText deadline={d.deadline} withLabel />}
          {milestones.length > 0 && <span className="font-mono tnum">MS {doneMs.length}/{milestones.length}</span>}
          <span className="font-mono tnum">未完タスク {openTasks.length}</span>
          {d.repo && (
            <a href={d.repo} target="_blank" rel="noreferrer" className="text-brand hover:underline inline-flex items-center gap-1">
              repo <ExternalLink size={11} />
            </a>
          )}
        </div>
    </div>
  );
}

// ---- Pulse ------------------------------------------------------------
// Hero が「今どうなっているか」の結論なら、Pulse はその根拠。
// 左に直近の前進（何が終わったか）、右に次の一手（何から手をつけるか）。

// README の `✅ YYYY-MM-DD` は完了メタデータ。ラベルとしては読みづらいので
// 表示から外し、完了バッジ側で日付として見せる（生テキストは編集用にそのまま）。
function cleanLabel(text: string): string {
  return text.replace(/\s*✅\s*\d{4}-\d{2}-\d{2}\s*/g, " ").trim();
}

function doneDate(t: Task): string | undefined {
  return /✅\s*(\d{4}-\d{2}-\d{2})/.exec(t.text)?.[1];
}

type PulseWin = { date: string; text: string; kind: "milestone" | "task" | "log" };

// 完了マイルストーン・完了タスク・ログを日付順に統合する。
// 近接して並べるだけで、因果は主張しない（Impact Timeline と同じ方針）。
function recentWins(d: ProjectDetail, limit: number): PulseWin[] {
  const wins: PulseWin[] = [];
  for (const m of d.milestones ?? []) {
    const date = m.state === "done" ? doneDate(m) : undefined;
    if (date) wins.push({ date, text: m.display, kind: "milestone" });
  }
  for (const t of d.tasks ?? []) {
    const date = t.state === "done" ? doneDate(t) : undefined;
    if (date) wins.push({ date, text: t.display, kind: "task" });
  }
  for (const entry of d.log ?? []) {
    if (entry.date) wins.push({ date: entry.date, text: entry.text, kind: "log" });
  }
  return wins.sort((a, b) => b.date.localeCompare(a.date)).slice(0, limit);
}

// 次の一手はバンド（今日 → 今週 → 今月 → いつか）を第一キーにする。
// バンドは「いつやるつもりか」の意図なので、締切より先に効かせる。
function nextMoves(openTasks: Task[], limit: number): Task[] {
  const band = (t: Task) => (t.today ? 0 : t.week ? 1 : t.month ? 2 : 3);
  return [...openTasks]
    .sort((a, b) =>
      band(a) - band(b) ||
      (a.due ?? "9999-99-99").localeCompare(b.due ?? "9999-99-99") ||
      (a.priority || Number.MAX_SAFE_INTEGER) - (b.priority || Number.MAX_SAFE_INTEGER))
    .slice(0, limit);
}

// ログ本文は段落なので、fact に出すときは最初の 1 文だけにする。
function firstSentence(text: string): string {
  const head = text.split("。")[0].trim();
  return head.length > 0 && head.length < text.length ? `${head}。` : text;
}

const BAND_LABEL: [keyof Pick<Task, "today" | "week" | "month">, string][] = [
  ["today", "今日"], ["week", "今週"], ["month", "今月"],
];

const STATE_ICON: Record<HeroTone, typeof TrendingUp> = {
  success: TrendingUp,
  warn: AlertTriangle,
  muted: Minus,
};
const STATE_TEXT: Record<HeroTone, string> = {
  success: "text-success",
  warn: "text-warning",
  muted: "text-muted-foreground",
};

// 2×2 の fact グリッド。左の状態文が「結論」、こちらが「今の数字」。
function Fact({ label, value, index }: { label: string; value: React.ReactNode; index: number }) {
  return (
    <div className={clsx("px-4 py-3.5 border-border", index < 2 && "border-b", index % 2 === 1 && "border-l")}>
      <span className="block text-[10px] text-muted-foreground mb-1">{label}</span>
      <b className="text-xs font-semibold leading-snug line-clamp-2">{value}</b>
    </div>
  );
}

function ProjectPulse({
  d, milestones, doneMs, openMs, openTasks, metrics, metricsError, refreshing, onRefresh,
}: {
  d: ProjectDetail;
  milestones: Task[];
  doneMs: Task[];
  openMs: Task[];
  openTasks: Task[];
  metrics: ProjectMetric[];
  metricsError: Error | null;
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const state = heroState(d, openMs, openTasks, metrics);
  const metric = metrics[0];
  const allWins = recentWins(d, 8);
  const win = allWins.find((w) => w.kind !== "log") ?? allWins[0];
  const moves = nextMoves(openTasks, 1);
  const activity = lastActivity(d, metrics);
  const since = activity ? -daysFromToday(activity) : undefined;
  if (!win && moves.length === 0 && !metric) return null;

  const Icon = STATE_ICON[state.tone];
  const format = (v: number) => metric?.display === "integer" ? Math.round(v).toLocaleString() : v.toFixed(1);

  // 現在値と次の到達点は、主指標 → マイルストーン → タスク の順で決める（Hero と同じ判断）。
  let current: React.ReactNode = `${openTasks.length} 件の未完タスク`;
  let nextPoint: React.ReactNode = openMs[0] ? inlineMd(openMs[0].display) : "—";
  if (metric && metric.current !== undefined) {
    current = `${format(metric.current)}${metric.target !== undefined ? ` / ${format(metric.target)}` : ""} ${metric.unit}`;
    const next = nextCheckpoint(metric);
    if (next) {
      nextPoint = `${format(next.value)} ${metric.unit} · ${next.label}（残り ${format(next.value - metric.current)}）`;
    } else if (metric.target !== undefined) {
      nextPoint = `${format(metric.target)} ${metric.unit} · 残り ${format(Math.max(0, metric.target - metric.current))}`;
    } else {
      nextPoint = "目標未設定";
    }
  } else if (milestones.length > 0) {
    current = `${doneMs.length} / ${milestones.length} milestones`;
    nextPoint = openMs[0] ? inlineMd(cleanLabel(openMs[0].display)) : "すべて完了";
  }

  const hint = metric?.delta.days30 !== undefined
    ? `30日 ${signed(metric.delta.days30, format)}`
    : since !== undefined ? `最終前進 ${since === 0 ? "今日" : `${since}日前`}` : undefined;

  return (
    <Section title="Pulse" sub="最近の活動" hint={hint}>
      <div className="rounded-lg border border-border bg-card grid grid-cols-1 lg:grid-cols-[0.8fr_1.2fr] overflow-hidden">
        <div className="p-5 ds-stack !gap-2 border-b lg:border-b-0 lg:border-r border-border">
          <p className="h-serif text-lg font-bold flex items-center gap-2">
            <Icon size={18} className={clsx("shrink-0", STATE_TEXT[state.tone])} />
            {state.label}
          </p>
          <p className="text-sm leading-relaxed text-muted-foreground">{state.sentence}</p>
          {metric && <Sparkline series={metric.series} />}
          {metric?.stale && (
            <Button variant="ghost" size="sm" className="self-start !px-1" loading={refreshing} onClick={onRefresh}>
              <RefreshCw size={13} />
              外部値を更新
            </Button>
          )}
          {metricsError && <p className="text-[10px] text-destructive">更新に失敗しました。最後の値を表示しています。</p>}
        </div>
        <div className="grid grid-cols-2">
          <Fact index={0} label="現在値" value={current} />
          <Fact index={1} label="次の到達点" value={nextPoint} />
          <Fact index={2} label="最近の成果" value={win ? inlineMd(cleanLabel(firstSentence(win.text))) : "—"} />
          <Fact
            index={3}
            label="次の一手"
            value={moves[0] ? (
              <span className="flex items-start gap-1.5">
                <span className="shrink-0 rounded border border-border px-1 text-[10px] font-normal text-muted-foreground">
                  {BAND_LABEL.find(([key]) => moves[0][key])?.[1] ?? "いつか"}
                </span>
                <span className="min-w-0">{inlineMd(moves[0].display)}</span>
              </span>
            ) : "—"}
          />
        </div>
      </div>
    </Section>
  );
}

// ---- Journey / Horizons -----------------------------------------------
// 到達点の並び。growth は metric checkpoint、delivery は README のマイルストーン。
// 終点を持たない project（maintenance）は Journey を出さず、時間軸の Horizons を出す。

type Step = { value: string; label: string; state: "done" | "now" | "todo" };

// 現在値を超える最初の checkpoint。設計上「target との割合」より
// 「次に届く一点」を主役にする。checkpoint 未定義なら undefined。
function nextCheckpoint(metric?: ProjectMetric): { value: number; label: string } | undefined {
  if (!metric || metric.current === undefined) return undefined;
  return (metric.checkpoints ?? []).find((cp) => cp.value > metric.current!);
}

function checkpointSteps(metric: ProjectMetric, format: (v: number) => string): Step[] {
  const points = metric.checkpoints ?? [];
  if (points.length === 0) return [];
  const current = metric.current;
  const steps: Step[] = [];
  let nowPlaced = false;
  for (const cp of points) {
    const reached = current !== undefined && current >= cp.value;
    // 現在値は「まだ届いていない最初の checkpoint」の手前に差し込む
    if (!reached && !nowPlaced && current !== undefined) {
      steps.push({ value: format(current), label: "NOW", state: "now" });
      nowPlaced = true;
    }
    steps.push({
      value: format(cp.value),
      label: metric.target !== undefined && cp.value === metric.target ? "GOAL" : cp.label,
      state: reached ? "done" : "todo",
    });
  }
  if (!nowPlaced && current !== undefined) steps.push({ value: format(current), label: "NOW", state: "now" });
  return steps;
}

function milestoneSteps(milestones: Task[]): Step[] {
  let nowPlaced = false;
  return milestones.map((m, i) => {
    const done = m.state === "done";
    const state: Step["state"] = done ? "done" : !nowPlaced ? ((nowPlaced = true), "now") : "todo";
    return { value: String(i + 1).padStart(2, "0"), label: cleanLabel(m.display), state };
  });
}

const DOT_CLASS: Record<Step["state"], string> = {
  done: "bg-success border-success",
  now: "bg-brand border-brand ring-2 ring-brand ring-offset-2 ring-offset-card",
  todo: "bg-card border-muted-foreground/50",
};

// 進行の可視化そのもの。checkpoint 版は独立した Journey セクション、
// マイルストーン版は「マイルストーン」セクションの先頭に置いて重複を避ける。
function JourneyLine({ steps }: { steps: Step[] }) {
  if (steps.length === 0) return null;
  return (
      <div className="rounded-lg border border-border bg-card p-6 overflow-x-auto">
        {/* ステップが増えたら潰さずに横スクロールさせる（1 ステップあたり最低 120px 確保） */}
        <div
          className="relative grid"
          style={{ gridTemplateColumns: `repeat(${steps.length}, 1fr)`, minWidth: `${Math.max(560, steps.length * 120)}px` }}
        >
          {/* 進行線。両端は最初/最後の dot の中心で止める */}
          <div className="absolute top-[10px] h-[3px] bg-muted" style={{ left: `${50 / steps.length}%`, right: `${50 / steps.length}%` }} />
          {steps.map((s, i) => (
            <div key={i} className="relative text-center px-1">
              <div className={clsx("mx-auto mb-2.5 size-[22px] rounded-full border-2", DOT_CLASS[s.state])} />
              <div className="font-mono tnum text-sm font-bold">{s.value}</div>
              <div
                title={s.label}
                className={clsx("mt-1 text-[11px] leading-snug line-clamp-2", s.state === "now" ? "text-brand font-bold" : "text-foreground/70")}
              >
                {s.label}
              </div>
            </div>
          ))}
        </div>
      </div>
  );
}

function ProjectJourney({ metric, hint }: { metric: ProjectMetric; hint?: string }) {
  const format = (v: number) => metric.display === "integer" ? Math.round(v).toLocaleString() : v.toFixed(1);
  const steps = checkpointSteps(metric, format);
  if (steps.length === 0) return null;
  return (
    <Section title="Journey" sub="目標までの到達点" hint={hint}>
      <JourneyLine steps={steps} />
    </Section>
  );
}

// 終点のない project 向け。達成率を出さず、バンドをそのまま Now / Soon / Someday に写す。
function ProjectHorizons({ openTasks }: { openTasks: Task[] }) {
  const groups: { title: string; note: string; items: Task[] }[] = [
    { title: "Now", note: "今日・今週", items: openTasks.filter((t) => t.today || t.week) },
    { title: "Soon", note: "今月", items: openTasks.filter((t) => t.month && !t.today && !t.week) },
    { title: "Someday", note: "いつか", items: openTasks.filter((t) => !t.today && !t.week && !t.month) },
  ];
  if (groups.every((g) => g.items.length === 0)) return null;
  return (
    <Section title="Horizons" sub="いつやるかで見る" hint="終点のない project">
      <div className="rounded-lg border border-border bg-card grid grid-cols-1 sm:grid-cols-3 overflow-hidden">
        {groups.map((g, i) => (
          <div key={g.title} className={clsx("p-5", i > 0 && "border-t sm:border-t-0 sm:border-l border-border")}>
            <h4 className="h-serif text-base font-bold">{g.title}</h4>
            <p className="mb-2 text-[10px] text-muted-foreground">{g.note}</p>
            {g.items.length === 0 ? (
              <p className="text-xs text-muted-foreground">—</p>
            ) : (
              <ul className="ds-stack !gap-1.5">
                {g.items.slice(0, 5).map((t) => (
                  <li key={`${t.file}:${t.line}`} className="text-xs leading-snug line-clamp-2">{inlineMd(t.display)}</li>
                ))}
                {g.items.length > 5 && <li className="text-[10px] text-muted-foreground">ほか {g.items.length - 5} 件</li>}
              </ul>
            )}
          </div>
        ))}
      </div>
    </Section>
  );
}

function MetricCard({ metric }: { metric: ProjectMetric }) {
  const current = metric.current;
  const pct = current !== undefined && metric.target !== undefined && metric.target !== 0
    ? Math.max(0, Math.min(100, (current / metric.target) * 100))
    : undefined;
  const format = (value: number) => metric.display === "integer" ? Math.round(value).toLocaleString() : value.toFixed(1);
  return (
    <div className="rounded-lg border border-border bg-card p-3 ds-stack !gap-3">
      <div className="flex items-start gap-2">
        <TrendingUp size={16} className="text-brand mt-0.5" />
        <div className="min-w-0 flex-1">
          <p className="text-xs font-medium truncate">{metric.label}</p>
          <p className="text-[10px] text-muted-foreground">
            {metric.updatedAt ? `${metric.updatedAt.slice(0, 10)} 更新` : "未取得"}
            {metric.stale && <span className="ml-1">· 要更新</span>}
          </p>
        </div>
      </div>
      <div className="flex items-end gap-2">
        <span className="font-mono tnum text-2xl font-semibold">{current === undefined ? "—" : format(current)}</span>
        <span className="pb-1 text-xs text-muted-foreground">{metric.unit}</span>
        {metric.target !== undefined && (
          <span className="ml-auto pb-1 font-mono tnum text-xs text-muted-foreground">/ {format(metric.target)}</span>
        )}
      </div>
      {pct !== undefined && (
        <div>
          <div className="h-1.5 rounded-full bg-muted overflow-hidden">
            <div className="h-full rounded-full bg-brand" style={{ width: `${pct}%` }} />
          </div>
          <p className="mt-1 text-right font-mono tnum text-[10px] text-muted-foreground">{Math.round(pct)}%</p>
        </div>
      )}
      <div className="grid grid-cols-[minmax(80px,1fr)_auto] items-end gap-3">
        <Sparkline series={metric.series} />
        <div className="text-right text-[10px] text-muted-foreground space-y-0.5">
          {metric.delta.days30 !== undefined && <p>30日 {signed(metric.delta.days30, format)}</p>}
          {metric.best && <p>最高 {format(metric.best.value)}</p>}
        </div>
      </div>
    </div>
  );
}

function signed(value: number, format: (value: number) => string): string {
  if (value === 0) return "±0";
  return `${value > 0 ? "+" : "−"}${format(Math.abs(value))}`;
}

function Sparkline({ series }: { series: ProjectMetric["series"] }) {
  if (series.length < 2) return <div className="h-10 flex items-end text-[10px] text-muted-foreground">履歴を蓄積中</div>;
  const width = 180;
  const height = 40;
  const values = series.map((point) => point.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const points = series.map((point, index) => {
    const x = (index / (series.length - 1)) * width;
    const y = height - 3 - ((point.value - min) / span) * (height - 6);
    return `${x},${y}`;
  }).join(" ");
  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="h-10 w-full" role="img" aria-label="メトリクスの推移">
      <polyline points={points} fill="none" stroke="currentColor" strokeWidth="2" vectorEffect="non-scaling-stroke" className="text-brand" />
    </svg>
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

// title は block 名（Pulse / Journey ...）、sub はそれが何かを一言で示す副題。
// 直訳の日本語を主見出しにすると冗長なので、説明は副題に落とす。
function Section({ title, sub, hint, children }: { title: string; sub?: string; hint?: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="flex items-baseline justify-between gap-3 pb-2 mb-3 border-b border-border">
        <div className="min-w-0 flex items-baseline gap-2">
          <h3 className="h-serif text-lg font-bold leading-tight shrink-0">{title}</h3>
          {sub && <span className="text-[11px] text-muted-foreground truncate">{sub}</span>}
        </div>
        {hint && <span className="shrink-0 font-mono tnum text-[11px] text-muted-foreground">{hint}</span>}
      </div>
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

// 最小限のインライン markdown（**太字** / `コード` / [ラベル](URL)）を React ノードへ。
// 外部 URL のみアンカー化。相対パス（docs/*.md 等）はラベルのみ表示（プロジェクト画面からは辿らせない。
// 内部ドキュメントへの導線は「関連ドキュメント」節で target 付きリンクとして出す）。
function inlineMd(text: string): React.ReactNode[] {
  const out: React.ReactNode[] = [];
  const re = /\*\*([^*]+)\*\*|`([^`]+)`|\[([^\]]+)\]\(([^)]+)\)/g;
  let last = 0;
  let key = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index));
    if (m[1]) out.push(<strong key={key++}>{m[1]}</strong>);
    else if (m[2]) out.push(<code key={key++} className="px-1 rounded bg-muted text-xs font-mono">{m[2]}</code>);
    else {
      const label = m[3].replace(/[`*]/g, "");
      const url = m[4];
      if (/^https?:\/\//.test(url)) {
        out.push(
          <a key={key++} href={url} target="_blank" rel="noreferrer" className="text-brand hover:underline">
            {label}
          </a>,
        );
      } else {
        out.push(<span key={key++}>{label}</span>);
      }
    }
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
