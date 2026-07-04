import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api, ApiError } from "../lib/api";
import type { SaveState } from "../components/ui/save-status";

// workspace ファイルの「非制御エディタ + 自動保存」を 1 箇所に集約する hook。
// 日記 / プロジェクトのフリースペース / 人生でやりたいこと が共有する:
//   - 800ms デバウンス + 離脱（unmount / path 切替）時 flush
//   - mtime 楽観ロック（Claude Code / VSCode の外部編集は 409 で顕在化）
//   - 非制御エディタ（Milkdown）向けの initialBody + editorKey remount
//
// read/write でファイル全文 ⇄ 編集対象本文の対応を差し替えられる。
// 既定は frontmatter を隠して本文だけ編集（保存時に前置して戻す）。

// frontmatter（--- ... ---）と本文を分ける
export function splitFrontmatter(content: string): { fm: string; body: string } {
  const m = content.match(/^---\n[\s\S]*?\n---\n?/);
  if (m) return { fm: m[0].replace(/\n*$/, "\n"), body: content.slice(m[0].length).replace(/^\n+/, "") };
  return { fm: "", body: content };
}

const defaultRead = (content: string) => splitFrontmatter(content).body;
const defaultWrite = (content: string, body: string) => splitFrontmatter(content).fm + body;

export interface UseAutosaveFileOptions {
  path: string;
  /** ファイル全文 → 編集対象本文（既定: frontmatter を除いた本文） */
  read?: (content: string) => string;
  /** (最新の全文, 新しい本文) → 保存する全文（既定: frontmatter を前置） */
  write?: (content: string, body: string) => string;
  onSaved?: () => void;
  debounceMs?: number;
}

export function useAutosaveFile({ path, read = defaultRead, write = defaultWrite, onSaved, debounceMs = 800 }: UseAutosaveFileOptions) {
  const query = useQuery({
    queryKey: ["file", path],
    queryFn: () => api.file(path),
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  });

  // initialBody は非制御エディタの defaultValue。editorKey の変更で作り直す。
  const [initialBody, setInitialBody] = useState<string | null>(null);
  const [editorKey, setEditorKey] = useState(0);
  const [dirty, setDirty] = useState(false);

  const pathRef = useRef(path); // 現在の本文が属するファイル（flush が path 切替後でも正しい先へ保存するため）
  const full = useRef(""); // 最新の既知の全文
  const baseMtime = useRef("");
  const savedBody = useRef(""); // 最後に保存した本文（dirty 判定）
  const latest = useRef(""); // 最新の編集内容（flush 用）
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const readRef = useRef(read);
  const writeRef = useRef(write);
  readRef.current = read;
  writeRef.current = write;

  const save = useMutation({
    mutationFn: (next: string) => {
      const newFull = writeRef.current(full.current, next);
      return api.putFile(pathRef.current, newFull, baseMtime.current).then((res) => ({ res, newFull, next }));
    },
    onSuccess: ({ res, newFull, next }) => {
      baseMtime.current = res.doc.mtime;
      full.current = newFull;
      savedBody.current = next;
      setDirty(latest.current !== next);
      onSaved?.();
    },
  });
  const saveRef = useRef(save);
  saveRef.current = save;

  // サーバ値が来たら（未取り込みなら）本文を切り出してエディタを作る
  useEffect(() => {
    if (query.data && initialBody === null) {
      pathRef.current = path;
      full.current = query.data.content;
      const b = readRef.current(query.data.content);
      savedBody.current = b;
      latest.current = b;
      baseMtime.current = query.data.doc.mtime;
      setInitialBody(b);
      setEditorKey((k) => k + 1);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, initialBody]);

  function flushNow() {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    if (initialBodyRef.current !== null && latest.current !== savedBody.current) {
      saveRef.current.mutate(latest.current);
    }
  }
  const initialBodyRef = useRef(initialBody);
  initialBodyRef.current = initialBody;
  const flushRef = useRef(flushNow);
  flushRef.current = flushNow;

  // path が変わる・unmount する時: 未保存分を旧ファイルへ flush してから編集状態をリセット
  useEffect(() => {
    return () => {
      flushRef.current();
    };
  }, [path]);
  useEffect(() => {
    if (pathRef.current !== path) {
      setInitialBody(null);
      setDirty(false);
      baseMtime.current = "";
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path]);

  function onChange(next: string) {
    latest.current = next;
    setDirty(next !== savedBody.current);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      if (latest.current !== savedBody.current) saveRef.current.mutate(latest.current);
    }, debounceMs);
  }

  /** エディタの中身をプログラムから差し替えて保存する（履歴からの復元など） */
  function replace(body: string) {
    latest.current = body;
    setInitialBody(body);
    setEditorKey((k) => k + 1);
    save.mutate(body);
  }

  /** 409 / エラーからの復帰: 編集状態を捨ててサーバ値を取り直す */
  function retry() {
    save.reset();
    setInitialBody(null);
    setDirty(false);
    query.refetch();
  }

  const conflict = save.error instanceof ApiError && save.error.status === 409;
  const notFound = query.error instanceof ApiError && query.error.status === 404;
  const saveState: SaveState = conflict
    ? "conflict"
    : save.isError
      ? "error"
      : save.isPending
        ? "saving"
        : dirty
          ? "dirty"
          : query.data
            ? "saved"
            : "idle";

  return { query, initialBody, editorKey, saveState, conflict, notFound, dirty, saveError: save.error, onChange, flush: flushNow, replace, retry };
}
