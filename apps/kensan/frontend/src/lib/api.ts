// kensan backend API クライアント。
// 契約はファイル形式（conventions.md）と apps/kensan/README.md の API 表。

export interface Meta {
  type?: string;
  title?: string;
  status?: string;
  tags?: string[];
  created?: string;
  updated?: string;
  parseError?: string;
}

export interface Doc {
  path: string;
  size: number;
  mtime: string;
  meta: Meta;
}

export interface Task {
  text: string; // 行内タグ込みの生テキスト（state/today/archive の照合に使う）
  display: string; // タグを除いた表示用テキスト
  state: "todo" | "done" | "skipped";
  file: string;
  line: number;
  project?: string;
  section?: string;
  today: boolean; // @today
  due?: string; // @due(YYYY-MM-DD)
  milestone?: string; // @ms(slug)
  priority?: number; // @p(N)（小さいほど上。0/未指定は末尾）
}

export interface Board {
  today: Task[] | null;
  stock: Task[] | null;
  someday: Task[] | null;
  milestones: Task[] | null;
}

export interface Focus {
  title: string;
  detail: string;
}

export interface Goals {
  northStar: string;
  focus: Focus[] | null;
}

export interface TaskSaveInput {
  file?: string; // 編集時の locator（作成時は省略）
  line?: number;
  text?: string;
  project: string; // "" = todo.md ## Now（今日やる・project 外）
  display: string;
  today: boolean;
  due: string; // "" = なし
  milestone: string; // "" = なし
}

export interface ProjectSummary {
  name: string;
  status: string;
  deadline?: string;
  goal: string;
  milestonesDone: number;
  milestonesTotal: number;
  openTasks: number;
}

export interface LogEntry {
  date?: string;
  text: string;
}

export interface NoteRef {
  group?: string;
  target?: string; // app で開ける .md パス。無ければ外部テキスト
  label: string;
  desc?: string;
}

export interface ProjectDetail {
  name: string;
  status: string;
  deadline?: string;
  repo?: string;
  overview: string;
  goal: string;
  milestones: Task[] | null;
  tasks: Task[] | null;
  log: LogEntry[] | null;
  notes: NoteRef[] | null;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

// W3C traceparent を生成して伝搬する（backend の otelhttp が拾う）
function traceparent(): string {
  const hex = (n: number) =>
    Array.from(crypto.getRandomValues(new Uint8Array(n)))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
  return `00-${hex(16)}-${hex(8)}-01`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      traceparent: traceparent(),
      ...init?.headers,
    },
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const body = await res.json();
      if (body.error) msg = body.error;
    } catch {
      /* JSON でないエラーは statusText のまま */
    }
    throw new ApiError(res.status, msg);
  }
  return res.json() as Promise<T>;
}

export const api = {
  board: () => request<Board>("/tasks"),

  goals: () => request<Goals>("/goals"),

  setTaskState: (t: Task, state: Task["state"]) =>
    request<{ task: Task }>("/tasks", {
      method: "PATCH",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text, state }),
    }),

  // 今日やる ⇄ ストックは @today タグの切替（行はその場 = project ファイルのまま）
  setToday: (t: Task, on: boolean) =>
    request<{ task: Task }>("/tasks/today", {
      method: "POST",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text, on }),
    }),

  // 完了タスクを daily へ退避（/reflection 相当）
  archiveToDaily: (t: Task) =>
    request<{ task: Task }>("/tasks/move", {
      method: "POST",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text }),
    }),

  // プロジェクト一覧（サマリ: status/締切/目標/進捗）
  projects: () => request<{ projects: ProjectSummary[] | null }>("/projects"),

  // プロジェクト詳細（目標/マイルストーン/タスク/ログ/関連ノート）
  projectDetail: (name: string) => request<ProjectDetail>(`/projects/${encodeURIComponent(name)}`),

  // 行の @due(YYYY-MM-DD) を設定（空文字で除去）
  setDue: (t: Task, due: string) =>
    request<{ task: Task }>("/tasks/due", { method: "POST", body: JSON.stringify({ file: t.file, line: t.line, text: t.text, due }) }),

  // 指定セクション末尾にチェックボックス行を追加（マイルストーン追加など）
  addLine: (file: string, section: string, display: string) =>
    request<{ task: Task }>("/tasks/add", { method: "POST", body: JSON.stringify({ file, section, display }) }),

  // プロジェクトのメタ更新（status / deadline / goal）
  updateProject: (name: string, input: { status: string; deadline: string; goal: string }) =>
    request<ProjectDetail>(`/projects/${encodeURIComponent(name)}`, { method: "PATCH", body: JSON.stringify(input) }),

  // 新規プロジェクト作成（テンプレート付き）
  createProject: (name: string) =>
    request<{ name: string }>("/projects", { method: "POST", body: JSON.stringify({ name }) }),

  // タスクの作成・編集（共通）。locator（file/line/text）があれば編集、無ければ作成。
  // project を変えると編集はファイル間移動になる。
  saveTask: (input: TaskSaveInput) =>
    request<{ task: Task }>("/tasks/save", { method: "POST", body: JSON.stringify(input) }),

  // タスク本文のインライン編集（行内タグは維持）
  setText: (t: Task, display: string) =>
    request<{ task: Task }>("/tasks/text", {
      method: "POST",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text, display }),
    }),

  // タスク行を削除
  deleteTask: (t: Task) =>
    request<{ status: string }>("/tasks/delete", {
      method: "POST",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text }),
    }),

  // ストックの 1 タスクに @p(N) を設定（ドラッグ並べ替えの 1 手）
  setPriority: (t: Task, priority: number) =>
    request<{ task: Task }>("/tasks/priority", {
      method: "POST",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text, priority }),
    }),

  // ストック全体の @p(N) 一括再採番（中間値の隙間が尽きたときのフォールバック）
  reorderTasks: (items: { file: string; line: number; text: string; priority: number }[]) =>
    request<{ status: string }>("/tasks/reorder", {
      method: "POST",
      body: JSON.stringify({ items }),
    }),

  daily: (date?: string) =>
    request<{ doc: Doc; content: string }>(`/daily${date ? `?date=${date}` : ""}`),

  dailyList: (limit: number) =>
    request<{ files: Doc[]; total: number }>(`/daily?limit=${limit}`),

  file: (path: string) =>
    request<{ doc: Doc; content: string }>(`/files/${encodeURI(path)}`),

  putFile: (path: string, content: string, baseMtime: string) =>
    request<{ doc: Doc }>(`/files/${encodeURI(path)}`, {
      method: "PUT",
      body: JSON.stringify({ content, baseMtime }),
    }),

  createFile: (path: string, content: string) =>
    request<{ doc: Doc }>(`/files`, {
      method: "POST",
      body: JSON.stringify({ path, content }),
    }),

  stats: () => request<Record<string, unknown>>("/stats"),

  files: (params: { type?: string; tag?: string; q?: string }) => {
    const qs = new URLSearchParams(
      Object.fromEntries(Object.entries(params).filter(([, v]) => v)) as Record<string, string>,
    ).toString();
    return request<{ files: Doc[]; total: number }>(`/files${qs ? `?${qs}` : ""}`);
  },

  search: (q: string, type?: string) =>
    request<{ hits: SearchHit[]; total: number; truncated: boolean }>(
      `/search?q=${encodeURIComponent(q)}${type ? `&type=${type}` : ""}`,
    ),

  reviews: () => request<{ reviews: ReviewEntry[]; total: number }>("/reviews"),

  // git 履歴（読み取り専用）。コミット一覧（新しい順）。
  history: (path: string) =>
    request<{ commits: Commit[] }>(`/history/${encodeURI(path)}`),

  // 指定コミット時点のファイル内容。
  historyAt: (path: string, rev: string) =>
    request<{ rev: string; content: string }>(`/history/${encodeURI(path)}?rev=${rev}`),
};

export interface Commit {
  hash: string;
  short: string;
  date: string; // RFC3339（author date）
  subject: string;
}

export interface SearchHit {
  path: string;
  line: number;
  snippet: string;
}

export interface ReviewEntry {
  path: string;
  name: string;
  kind: "weekly" | "daily" | "monthly" | "other";
  mtime: string;
  size: number;
}

// iframe 用のレビュー配信 URL（path は "reviews/..." 形式）
export function reviewContentURL(path: string): string {
  return `/api/v1/reviews/${path.replace(/^reviews\//, "")}`;
}

export function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

export function dailyPath(date: string): string {
  const [y, m, d] = date.split("-");
  return `daily/${y}/${m}/${d}.md`;
}

// conventions.md に従った daily の骨組み
export function dailySkeleton(date: string): string {
  return `---
type: daily
tags: []
created: ${date}
updated: ${date}
---

# ${date}

## 日記
`;
}
