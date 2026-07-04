import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { BookOpen, ChevronLeft, ChevronRight } from "lucide-react";
import { api, ApiError, todayISO, dailyPath, dailySkeleton } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody, CardFoot } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { MilkdownEditor } from "../components/editors/MilkdownEditor";
import { DailyCalendar } from "../components/DailyCalendar";
import { useAutosaveFile } from "../hooks/useAutosaveFile";
import { Empty, ErrorState, Skeleton } from "../components/ui/states";
import { SaveStatus } from "../components/ui/save-status";

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

// 日記ページ。書いてる途中で保存を忘れて飛ぶのが悲しい、を解消するため自動保存。
// frontmatter は隠して本文だけ Milkdown で見たまま編集（created/updated/tags/type は自動・外部管理）。
// デバウンス保存・楽観ロック・離脱時 flush は useAutosaveFile に集約。
export function DailyPage() {
  const [params, setParams] = useSearchParams();
  const date = params.get("date") ?? todayISO();
  const isToday = date === todayISO();
  const qc = useQueryClient();

  const file = useAutosaveFile({ path: dailyPath(date) });

  // 日付を切り替える。今日なら ?date を落として素の /daily に戻す。
  // 未保存分は path 変更時に hook が旧ファイルへ flush する。
  function goDate(next: string) {
    if (next === todayISO()) setParams({}, { replace: false });
    else setParams({ date: next }, { replace: false });
  }

  const create = useMutation({
    mutationFn: () => api.createFile(dailyPath(date), dailySkeleton(date)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["file", dailyPath(date)] });
      qc.invalidateQueries({ queryKey: ["dailyList"] });
    },
  });

  return (
    <>
      <PageHeader
        eyebrow={`記録 · ${dailyPath(date)}`}
        title={`${date}（${weekdayLabel(date)}）の日記`}
        sub="その日の出来事・感想・学び。自動保存（離脱時も保存）。完了タスクは /reflection がここに移してくる。"
        actions={
          <div className="ds-inline">
            {file.query.data && <SaveStatus state={file.saveState} />}
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
        {file.notFound ? (
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
        ) : file.query.isError ? (
          <CardBody>
            <ErrorState error={file.query.error} onRetry={() => file.query.refetch()} />
          </CardBody>
        ) : file.conflict ? (
          <CardBody>
            <ErrorState
              error={
                new ApiError(409, "他のクライアント（Claude Code / VSCode）が先に保存しました。再読込してから編集し直してください。")
              }
              onRetry={file.retry}
            />
          </CardBody>
        ) : file.query.isPending || file.initialBody === null ? (
          <CardBody>
            <Skeleton className="h-64 w-full" />
          </CardBody>
        ) : (
          <CardBody>
            <MilkdownEditor
              key={file.editorKey}
              defaultValue={file.initialBody}
              onChange={file.onChange}
              minHeight="28rem"
              placeholder="今日のこと・学び・みのりちゃん/なぎちゃんへ…"
            />
          </CardBody>
        )}
        {file.saveState === "error" && (
          <CardFoot>
            <ErrorState error={file.saveError} onRetry={file.flush} />
          </CardFoot>
        )}
      </Card>
        <DailyCalendar selected={date} onSelect={goDate} />
      </div>
    </>
  );
}
