import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { History, RotateCcw, X } from "lucide-react";
import clsx from "clsx";
import { api, ApiError, type Commit } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { MilkdownEditor } from "../components/editors/MilkdownEditor";
import { ErrorState, Skeleton, SkeletonRows, Empty } from "../components/ui/states";
import { SaveStatus, type SaveState } from "../components/ui/save-status";

// 人生でやりたいこと — 常に見える場所に置く「北極星」ドキュメント。
// 普段は Milkdown(WYSIWYG)で見たまま編集（自動保存）、「履歴」で過去の版
// （= git コミット時点）を振り返る。バージョニングは workspace の git に乗る
// （アプリは git に書かない。版が増えるのは通常の commit のたび）。実体はこの 1 ファイル。
const LIFE_PATH = "notes/2026/03/人生でやりたいこと.md";

// frontmatter（--- ... ---）と本文を分ける。本文だけ編集し、保存時に前置して戻す。
function splitFm(content: string): { fm: string; body: string } {
  const m = content.match(/^---\n[\s\S]*?\n---\n?/);
  if (m) return { fm: m[0].replace(/\n*$/, "\n"), body: content.slice(m[0].length).replace(/^\n+/, "") };
  return { fm: "", body: content };
}

function fmtDate(iso: string): string {
  return iso.slice(0, 10); // RFC3339 → YYYY-MM-DD（日付の粒度で十分）
}

export function LifeGoalsPage() {
  const doc = useQuery({ queryKey: ["file", LIFE_PATH], queryFn: () => api.file(LIFE_PATH) });

  // Milkdown は非制御（defaultValue で初期化）。初期本文と remount 用の key を state で持つ。
  const [initialBody, setInitialBody] = useState<string | null>(null);
  const [editorKey, setEditorKey] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [showHistory, setShowHistory] = useState(false);

  const fm = useRef<string>(""); // frontmatter（保存時に本文へ前置）
  const baseMtime = useRef<string>(""); // 楽観ロックの基準（保存成功ごとに進める）
  const savedBody = useRef<string>(""); // 最後に保存した本文（dirty 判定）
  const latest = useRef<string>(""); // 最新の編集内容（離脱時 flush 用）
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const save = useMutation({
    mutationFn: (next: string) => api.putFile(LIFE_PATH, fm.current + next, baseMtime.current),
    onSuccess: (res, next) => {
      baseMtime.current = res.doc.mtime;
      savedBody.current = next;
      setDirty(false);
    },
  });

  // サーバ値が来たら（まだ取り込んでいなければ）frontmatter を分離して本文を初期化
  useEffect(() => {
    if (doc.data && initialBody === null) {
      const p = splitFm(doc.data.content);
      fm.current = p.fm;
      savedBody.current = p.body;
      latest.current = p.body;
      baseMtime.current = doc.data.doc.mtime;
      setInitialBody(p.body);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doc.data]);

  function scheduleSave(md: string) {
    latest.current = md;
    setDirty(md !== savedBody.current);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      if (latest.current !== savedBody.current) save.mutate(latest.current);
    }, 800);
  }

  function flush() {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    if (latest.current !== savedBody.current) save.mutate(latest.current);
  }
  const flushRef = useRef(flush);
  flushRef.current = flush;
  useEffect(() => () => flushRef.current(), []);

  // 過去の版を編集欄へ呼び戻して保存する（remount で WYSIWYG を作り直す）
  function restore(content: string) {
    const b = splitFm(content).body;
    latest.current = b;
    setInitialBody(b);
    setEditorKey((k) => k + 1);
    setShowHistory(false);
    save.mutate(b);
  }

  const conflict = save.error instanceof ApiError && save.error.status === 409;
  const saveState: SaveState = conflict
    ? "conflict"
    : save.isError
      ? "error"
      : save.isPending
        ? "saving"
        : dirty
          ? "dirty"
          : doc.data
            ? "saved"
            : "idle";

  return (
    <>
      {/* Milkdown(Crepe) のホスト高さ。crepe テーマ CSS は MilkdownEditor 側で読み込み済み */}
      <style>{`.life-editor .milkdown{min-height:60vh}`}</style>

      <PageHeader
        eyebrow={`北極星 · ${LIFE_PATH}`}
        title="人生でやりたいこと"
        sub="いつでも立ち返る場所。見たまま編集・自動保存（離脱時も保存）。版は git に積まれ、「履歴」で当時の全文を振り返れる。"
        actions={
          <>
            {doc.data && <SaveStatus state={saveState} />}
            <Button
              variant={showHistory ? "primary" : "outline"}
              size="sm"
              onClick={() => setShowHistory((v) => !v)}
            >
              <History size={14} />
              履歴
            </Button>
          </>
        }
      />

      <div className={clsx("grid gap-6 items-start", showHistory ? "grid-cols-[1fr_340px]" : "grid-cols-1")}>
        <Card className="min-h-[60vh]">
          {doc.isPending || initialBody === null ? (
            <CardBody>
              <Skeleton className="h-96 w-full" />
            </CardBody>
          ) : conflict ? (
            <CardBody>
              <ErrorState
                error={
                  new ApiError(
                    409,
                    "他のクライアント（Claude Code / VSCode）が先に保存しました。再読込してから編集し直してください。",
                  )
                }
                onRetry={() => {
                  save.reset();
                  setInitialBody(null);
                  setDirty(false);
                  setEditorKey((k) => k + 1);
                  doc.refetch();
                }}
              />
            </CardBody>
          ) : doc.isError ? (
            <CardBody>
              <ErrorState error={doc.error} onRetry={() => doc.refetch()} />
            </CardBody>
          ) : (
            <CardBody className="life-editor">
              <MilkdownEditor key={editorKey} defaultValue={initialBody} onChange={scheduleSave} />
            </CardBody>
          )}
        </Card>

        {showHistory && <HistoryPanel onClose={() => setShowHistory(false)} onRestore={restore} />}
      </div>
    </>
  );
}

function HistoryPanel({
  onClose,
  onRestore,
}: {
  onClose: () => void;
  onRestore: (content: string) => void;
}) {
  const log = useQuery({ queryKey: ["history", LIFE_PATH], queryFn: () => api.history(LIFE_PATH) });
  const [rev, setRev] = useState<string | null>(null);

  const version = useQuery({
    queryKey: ["history", LIFE_PATH, rev],
    queryFn: () => api.historyAt(LIFE_PATH, rev!),
    enabled: !!rev,
  });

  return (
    <Card className="sticky top-6">
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <div className="flex items-center gap-2 text-sm font-medium">
          <History size={15} />
          版の履歴
        </div>
        <button onClick={onClose} aria-label="履歴を閉じる" className="text-muted-foreground hover:text-foreground">
          <X size={15} />
        </button>
      </div>
      <CardBody className="!p-2">
        {log.isPending ? (
          <SkeletonRows rows={5} />
        ) : log.isError ? (
          <ErrorState error={log.error} onRetry={() => log.refetch()} />
        ) : log.data.commits.length === 0 ? (
          <Empty
            icon={<History />}
            title="まだ版がありません"
            desc="このファイルを編集して commit すると、当時の全文がここに積まれていきます。"
          />
        ) : (
          <ul className="ds-stack !gap-0.5">
            {log.data.commits.map((c) => (
              <li key={c.hash}>
                <button
                  onClick={() => setRev(rev === c.hash ? null : c.hash)}
                  className={clsx(
                    "w-full px-2 py-1.5 rounded-md text-left",
                    rev === c.hash ? "bg-accent text-accent-foreground" : "hover:bg-accent/60",
                  )}
                >
                  <div className="flex items-center gap-2">
                    <span className="font-mono tnum text-xs text-brand">{fmtDate(c.date)}</span>
                    <span className="font-mono tnum text-[11px] text-muted-foreground">{c.short}</span>
                  </div>
                  <div className="text-sm truncate">{c.subject}</div>
                </button>
                {rev === c.hash && <VersionView commit={c} version={version} onRestore={onRestore} />}
              </li>
            ))}
          </ul>
        )}
      </CardBody>
    </Card>
  );
}

function VersionView({
  commit,
  version,
  onRestore,
}: {
  commit: Commit;
  version: ReturnType<typeof useQuery<{ rev: string; content: string }, Error>>;
  onRestore: (content: string) => void;
}) {
  return (
    <div className="mx-1 mb-2 mt-1 rounded-md border border-border bg-card">
      {version.isPending ? (
        <div className="p-3">
          <Skeleton className="h-40 w-full" />
        </div>
      ) : version.isError ? (
        <div className="p-3">
          <ErrorState error={version.error} onRetry={() => version.refetch()} />
        </div>
      ) : (
        <>
          <pre className="max-h-[40vh] overflow-y-auto whitespace-pre-wrap font-sans text-[13px] leading-relaxed p-3">
            {splitFm(version.data.content).body}
          </pre>
          <div className="flex justify-end border-t border-border p-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => onRestore(version.data.content)}
              title={`${fmtDate(commit.date)} 時点の内容を編集欄に戻して保存する`}
            >
              <RotateCcw size={13} />
              この版に戻す
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
