import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "../lib/api";
import type { SaveState } from "../components/ui/save-status";

export function splitFrontmatter(content: string): { fm: string; body: string } {
  const m = content.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/);
  if (m) return { fm: m[0].replace(/\n*$/, "\n"), body: content.slice(m[0].length).replace(/^\n+/, "") };
  return { fm: "", body: content };
}
const defaultRead = (content: string) => splitFrontmatter(content).body;
const defaultWrite = (content: string, body: string) => splitFrontmatter(content).fm + body;
const draftKey = (path: string) => `kensan:draft:${path}`;
function storeDraft(path: string, body: string, mtime: string) {
  try { sessionStorage.setItem(draftKey(path), JSON.stringify({ body, mtime })); } catch { /* Storage may be unavailable. */ }
}
function clearDraft(path: string) {
  try { sessionStorage.removeItem(draftKey(path)); } catch { /* Storage may be unavailable. */ }
}
export interface UseAutosaveFileOptions {
  path: string;
  read?: (content: string) => string;
  write?: (content: string, body: string) => string;
  onSaved?: () => void;
  debounceMs?: number;
}

interface SaveSession {
  path: string; full: string; mtime: string; saved: string; latest: string; initial: string | null;
  recovery: string; key: number; error: Error | null; running: Promise<boolean> | null;
  timer: ReturnType<typeof setTimeout> | null;
  listeners: Set<() => void>;
  read: (content: string) => string;
  write: (content: string, body: string) => string;
  onSaved?: () => void;
}
const sessions = new WeakMap<object, Map<string, SaveSession>>();

// Each path owns its draft, mtime and queue. Old responses never update a new file.
// A tab-local recovery copy survives a collapsed editor or a failed unmount save.
export function useAutosaveFile({ path, read = defaultRead, write = defaultWrite, onSaved, debounceMs = 800 }: UseAutosaveFileOptions) {
  const client = useQueryClient();
  const [, redraw] = useState(0);
  const session = useMemo(() => {
    let files = sessions.get(client);
    if (!files) { files = new Map(); sessions.set(client, files); }
    let entry = files.get(path);
    if (!entry) {
      entry = { path, full: "", mtime: "", saved: "", latest: "", initial: null, recovery: "", key: 0,
        error: null, running: null, timer: null, listeners: new Set(), read, write, onSaved };
      files.set(path, entry);
    }
    return entry;
  }, [client, path]);
  session.read = read;
  session.write = write;
  session.onSaved = onSaved;
  const notify = (s: SaveSession) => s.listeners.forEach(listener => listener());
  const query = useQuery({
    queryKey: ["file", path], queryFn: () => api.file(path),
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  });
  async function drain(s: typeof session, retry = false): Promise<boolean> {
    if (s.timer) { clearTimeout(s.timer); s.timer = null; }
    if (s.running) return s.running;
    if (retry) s.error = null;
    if (s.error) return false;
    if (s.initial === null) return true;
    if (s.latest === s.saved) return true;
    s.running = Promise.resolve().then(async () => {
      while (s.latest !== s.saved) {
        const body = s.latest;
        const full = s.write(s.full, body);
        const baseMtime = s.mtime;
        const savedCallback = s.onSaved;
        try {
          const res = await api.putFile(s.path, full, baseMtime);
          s.full = full;
          s.mtime = res.doc.mtime;
          s.saved = body;
          s.initial = body;
          if (s.latest === body) clearDraft(s.path);
          else storeDraft(s.path, s.latest, s.mtime);
          client.setQueryData(["file", s.path], { doc: res.doc, content: full });
          savedCallback?.();
          for (const key of ["today", "portfolio", "dailyList", "project", "projects"]) {
            void client.invalidateQueries({ queryKey: [key] });
          }
        } catch (error) {
          s.error = error instanceof Error ? error : new Error(String(error));
          storeDraft(s.path, s.latest, s.mtime);
          return false;
        }
      }
      return true;
    }).finally(() => { s.running = null; notify(s); });
    notify(s);
    return s.running;
  }
  useEffect(() => {
    const listener = () => redraw(n => n + 1);
    session.listeners.add(listener);
    return () => { session.listeners.delete(listener); void drain(session); };
  }, [session]);
  useEffect(() => {
    const data = query.data;
    if (!data || session.running || (session.initial !== null && (session.latest !== session.saved || session.error))) return;
    if (session.initial !== null && session.mtime === data.doc.mtime) return;
    session.full = data.content;
    session.mtime = data.doc.mtime;
    session.saved = session.read(data.content);
    session.latest = session.saved;
    try {
      const raw = sessionStorage.getItem(draftKey(path));
      if (raw) {
        const draft = JSON.parse(raw);
        if (typeof draft.body === "string" && draft.body !== session.saved) {
          session.latest = draft.body;
          if (draft.mtime !== session.mtime) session.error = new ApiError(409, "保存前の入力を復元しました。ファイルにも変更があるため、内容を確認してください。");
        }
      }
    } catch { /* A broken recovery copy must not prevent reading the real file. */ }
    try { session.recovery = sessionStorage.getItem(`kensan:recovery:${path}`) ?? session.recovery; } catch { /* Storage may be unavailable. */ }
    session.initial = session.latest;
    session.key++;
    notify(session);
    if (session.latest !== session.saved && !session.error) void drain(session);
  }, [query.data, session, session.initial]);
  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => {
      if (session.latest !== session.saved) { event.preventDefault(); event.returnValue = ""; }
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [session]);
  function onChange(body: string) {
    session.latest = body;
    if (body !== session.saved) storeDraft(path, body, session.mtime);
    else clearDraft(path);
    if (session.timer) clearTimeout(session.timer);
    session.timer = setTimeout(() => { void drain(session); }, debounceMs);
    notify(session);
  }
  function retry() {
    if (session.latest !== session.saved) {
      session.recovery = session.latest;
      try { sessionStorage.setItem(`kensan:recovery:${path}`, session.latest); } catch { /* Storage may be unavailable. */ }
    }
    clearDraft(path);
    session.error = null;
    session.initial = null;
    session.latest = session.saved;
    notify(session);
    void query.refetch();
  }
  function replace(body: string) {
    onChange(body);
    session.initial = body;
    session.key++;
    return drain(session, true);
  }
  const conflict = session.error instanceof ApiError && session.error.status === 409;
  const dirty = session.latest !== session.saved;
  const notFound = query.error instanceof ApiError && query.error.status === 404;
  const saveState: SaveState = conflict ? "conflict" : session.error ? "error" : session.running ? "saving" : dirty ? "dirty" : query.data ? "saved" : "idle";
  return { query, initialBody: session.initial, editorKey: session.key, saveState, conflict, notFound, dirty,
    draft: session.latest, recovery: session.recovery, saveError: session.error, onChange, flush: () => drain(session, true), replace, retry };
}
