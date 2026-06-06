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
  text: string;
  state: "todo" | "done" | "skipped";
  file: string;
  line: number;
  project?: string;
  section?: string;
}

export interface Board {
  today: Task[] | null;
  stock: Task[] | null;
  someday: Task[] | null;
  milestones: Task[] | null;
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

  setTaskState: (t: Task, state: Task["state"]) =>
    request<{ task: Task }>("/tasks", {
      method: "PATCH",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text, state }),
    }),

  moveTask: (t: Task, to: "today" | "stock" | "daily", project?: string) =>
    request<{ task: Task }>("/tasks/move", {
      method: "POST",
      body: JSON.stringify({ file: t.file, line: t.line, text: t.text, to, project }),
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
};

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
