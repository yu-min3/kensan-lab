import { useState, useEffect, useRef } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { BookOpen, ChevronLeft, ChevronRight } from "lucide-react";
import { api, ApiError, todayISO, dailyPath, dailySkeleton } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody, CardFoot } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { MarkdownEditor } from "../components/MarkdownEditor";
import { DailyCalendar } from "../components/DailyCalendar";
import { Empty, ErrorState, Skeleton } from "../components/ui/states";
import { SaveStatus, type SaveState } from "../components/ui/save-status";

const WEEKDAYS = ["日", "月", "火", "水", "木", "金", "土"];

// "YYYY-MM-DD" を delta 日ずらす（カレンダー外の前後日ナビ用）
function shiftDate(date: string, delta: number): string {
  const [y, m, d] = date.split("-").map(Number);
  const t = new Date(y, m - 1, d + delta);
  return `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, "0")}-${String(t.getDate()).padStart(2, "0")}`;
}

function weekdayLabel(date: string): string {
  const [y, m, d] = date.split("-").map(Number);
  return WEEKDAYS[new Date(y, m - 1, d).getDay()];
}

// frontmatter（--- ... ---）と本文を分ける。本文だけ編集し、保存時に前置して戻す。
function splitFm(content: string): { fm: string; body: string } {
  const m = content.match(/^---\n[\s\S]*?\n---\n?/);
  if (m) return { fm: m[0].replace(/\n*$/, "\n"), body: content.slice(m[0].length).replace(/^\n+/, "") };
  return { fm: "", body: content };
}

// 日記ページ。書いてる途中で保存を忘れて飛ぶのが悲しい、を解消するため自動保存。
// 800ms デバウンス + フォーカスアウトで保存。保存は編集開始時点の mtime で楽観ロックし、
// Claude / VSCode の外部編集は 409 で素直に伝える。
export function DailyPage() {
  const [params, setParams] = useSearchParams();
  const date = params.get("date") ?? todayISO();
  const isToday = date === todayISO();
  const qc = useQueryClient();

  // 日付を切り替える。今日なら ?date を落として素の /daily に戻す。
  function goDate(next: string) {
    flush(); // 離脱前に未保存分を保存
    if (next === todayISO()) setParams({}, { replace: false });
    else setParams({ date: next }, { replace: false });
  }

  const daily = useQuery({
    queryKey: ["daily", date],
    queryFn: () => api.daily(date),
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  });

  // frontmatter は隠して本文だけ編集（created/updated/tags/type は自動・外部管理）。
  const [body, setBody] = useState<string | null>(null);
  const baseMtime = useRef<string>("");
  const fm = useRef<string>(""); // frontmatter（保存時に本文へ前置）
  const savedBody = useRef<string>(""); // 最後に保存した本文（dirty 判定）
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // 日付が変わったら編集状態をリセット
  useEffect(() => {
    setBody(null);
    baseMtime.current = "";
  }, [date]);

  // サーバ値が来たら（未編集なら）frontmatter を分離して本文だけ取り込む
  useEffect(() => {
    if (daily.data && body === null) {
      const p = splitFm(daily.data.content);
      fm.current = p.fm;
      savedBody.current = p.body;
      baseMtime.current = daily.data.doc.mtime;
      setBody(p.body);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [daily.data]);

  const save = useMutation({
    mutationFn: (next: string) => api.putFile(dailyPath(date), fm.current + next, baseMtime.current),
    onSuccess: (res, next) => {
      baseMtime.current = res.doc.mtime;
      savedBody.current = next;
    },
  });

  const create = useMutation({
    mutationFn: () => api.createFile(dailyPath(date), dailySkeleton(date)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["daily", date] });
      qc.invalidateQueries({ queryKey: ["dailyList"] });
    },
  });

  function flush() {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    if (body !== null && body !== savedBody.current) save.mutate(body);
  }

  function onChange(next: string) {
    setBody(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => save.mutate(next), 800);
  }

  // 最新の flush をアンマウント時に呼ぶ（ref 経由で stale closure を避ける）
  const flushRef = useRef(flush);
  flushRef.current = flush;
  useEffect(() => () => flushRef.current(), []);

  const notFound = daily.error instanceof ApiError && daily.error.status === 404;
  const conflict = save.error instanceof ApiError && save.error.status === 409;
  const dirty = body !== null && body !== savedBody.current;
  const saveState: SaveState = conflict
    ? "conflict"
    : save.isError
      ? "error"
      : save.isPending
        ? "saving"
        : dirty
          ? "dirty"
          : daily.data
            ? "saved"
            : "idle";

  return (
    <>
      <PageHeader
        eyebrow={`記録 · ${dailyPath(date)}`}
        title={`${date}（${weekdayLabel(date)}）の日記`}
        sub="その日の出来事・感想・学び。自動保存（離脱時も保存）。完了タスクは /reflection がここに移してくる。"
        actions={
          <div className="ds-inline">
            {daily.data && <SaveStatus state={saveState} />}
            <Button variant="outline" size="sm" iconOnly aria-label="前日" onClick={() => goDate(shiftDate(date, -1))}>
              <ChevronLeft size={16} />
            </Button>
            <Button
              variant="outline"
              size="sm"
              iconOnly
              aria-label="翌日"
              disabled={isToday}
              onClick={() => goDate(shiftDate(date, 1))}
            >
              <ChevronRight size={16} />
            </Button>
            {!isToday && (
              <Button variant="ghost" size="sm" onClick={() => goDate(todayISO())}>
                今日
              </Button>
            )}
          </div>
        }
      />
      <div className="grid gap-5 lg:grid-cols-[1fr_300px] items-start">
      <Card>
        {daily.isPending ? (
          <CardBody>
            <Skeleton className="h-64 w-full" />
          </CardBody>
        ) : notFound ? (
          <CardBody>
            <Empty
              icon={<BookOpen />}
              title={`${date} の日記はまだありません`}
              desc="骨組み（日記セクション付き）を作成して書き始められます。"
              actions={
                <Button variant="primary" loading={create.isPending} onClick={() => create.mutate()}>
                  日記を作成
                </Button>
              }
            />
          </CardBody>
        ) : daily.isError ? (
          <CardBody>
            <ErrorState error={daily.error} onRetry={() => daily.refetch()} />
          </CardBody>
        ) : conflict ? (
          <CardBody>
            <ErrorState
              error={
                new ApiError(409, "他のクライアント（Claude Code / VSCode）が先に保存しました。再読込してから編集し直してください。")
              }
              onRetry={() => {
                save.reset();
                setBody(null);
                daily.refetch();
              }}
            />
          </CardBody>
        ) : (
          <CardBody>
            <MarkdownEditor
              value={body ?? ""}
              onChange={onChange}
              minHeight="28rem"
              placeholder="今日のこと・学び・みのりちゃん/なぎちゃんへ…"
            />
          </CardBody>
        )}
        {save.isError && !conflict && (
          <CardFoot>
            <ErrorState error={save.error} onRetry={() => flush()} />
          </CardFoot>
        )}
      </Card>
        <DailyCalendar selected={date} onSelect={goDate} />
      </div>
    </>
  );
}
