import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { BookOpen } from "lucide-react";
import { api, dailyPath, type DiarySummary } from "../../lib/api";
import { useWorkspaceRefresh } from "../../hooks/useWorkspaceRefresh";
import { Button } from "../ui/button";
import { Card, CardBody } from "../ui/card";
import { useToast } from "../ui/toast";
import { daysUntil } from "./model";

// 今日画面の最上段。1 行で日記を残す（daily の ## 日記 へ時刻付きで追記）。
// 長く書く日は日記ページへ。記入の有無は事実だけ出し、評価語は付けない。
export function DiaryLine({ date, diary }: { date: string; diary: DiarySummary }) {
  const [text, setText] = useState("");
  const refresh = useWorkspaceRefresh();
  const toast = useToast();
  const save = useMutation({
    mutationFn: (line: string) => api.dailyLine(date, line),
    onSuccess: () => {
      setText("");
      toast({ title: "日記に追記しました", desc: dailyPath(date) });
    },
    onSettled: refresh,
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    const line = text.trim();
    if (line) save.mutate(line);
  };
  const writtenToday = diary.days.some((d) => d.date === date && d.written);
  const since = diary.last ? daysUntil(diary.last, date) : null;
  const written30 = diary.days.filter((d) => d.written).length;
  return (
    <Card>
      <CardBody className="ds-stack">
        <form onSubmit={submit} className="flex flex-col sm:flex-row gap-2">
          <label htmlFor="diary-line" className="sr-only">
            今日の日記（1 行）
          </label>
          <input
            id="diary-line"
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={500}
            enterKeyHint="send"
            autoComplete="off"
            placeholder={writtenToday ? "もう 1 行足す" : "今日を 1 行で"}
            className="min-w-0 flex-1 rounded-md border border-border bg-card px-3 h-10 text-base focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
          <Button type="submit" variant="primary" loading={save.isPending} disabled={!text.trim()}>
            残す
          </Button>
        </form>
        {save.isError && (
          <p role="alert" className="text-xs text-destructive">
            {save.error.message}
          </p>
        )}
        <div className="ds-stack !gap-2 text-xs text-muted-foreground" data-testid="diary-days">
          <div
            className="grid gap-[3px] w-full max-w-[420px]"
            style={{ gridTemplateColumns: `repeat(${diary.days.length || 1}, minmax(0, 1fr))` }}
            aria-hidden="true"
          >
            {diary.days.map((d) => (
              <span
                key={d.date}
                title={d.date}
                className={`aspect-square max-w-3 rounded-[2px] ${d.written ? "bg-brand" : "bg-muted"}`}
              />
            ))}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
            <span>
              直近30日で<span className="font-mono tnum"> {written30} </span>日
              {diary.last
                ? since !== null && since < 0
                  ? ` · 最後は ${diary.last}（${-since}日前）`
                  : " · 今日書いた"
                : " · まだ記録なし"}
            </span>
            <Link to={`/daily?date=${date}`} className="inline-flex items-center gap-1 text-brand">
              <BookOpen size={14} />
              日記ページで書く
            </Link>
          </div>
        </div>
      </CardBody>
    </Card>
  );
}
