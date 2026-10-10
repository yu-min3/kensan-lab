import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { BookOpen } from "lucide-react";
import { api, dailyPath, type TodayView } from "../../lib/api";
import { useWorkspaceRefresh } from "../../hooks/useWorkspaceRefresh";
import { Button } from "../ui/button";
import { Card, CardBody } from "../ui/card";
import { useToast } from "../ui/toast";
import { daysUntil } from "./model";

type Mode = "diary" | "done";

// 今日画面の最上段。1 行で残す入口を 1 つにまとめる。
// - 日記: daily の ## 日記 へ時刻付きで追記
// - やったこと: 選んだ project の ## タスク へ完了済み（@done 今日）で追加。予定外の作業もここから入る
// 記入の有無は事実だけ出し、評価語は付けない。
export function DiaryLine({ view }: { view: TodayView }) {
  const { date, diary } = view;
  const projects = view.projects.filter((p) => p.status === "active").map((p) => p.name);
  const focus = (view.goals.focus ?? []).map((f) => f.project).find((p) => p && projects.includes(p));
  const [mode, setMode] = useState<Mode>("diary");
  const [text, setText] = useState("");
  const [project, setProject] = useState(focus ?? projects[0] ?? "");
  const refresh = useWorkspaceRefresh();
  const toast = useToast();
  const save = useMutation({
    mutationFn: async (line: string): Promise<void> => {
      if (mode === "diary") await api.dailyLine(date, line);
      else await api.recordDone(project, line);
    },
    onSuccess: () => {
      setText("");
      toast(
        mode === "diary"
          ? { title: "日記に追記しました", desc: dailyPath(date) }
          : { title: "やったことを残しました", desc: project || "todo.md" },
      );
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
  const { planned, unplanned } = view.doneToday;
  const tab = (m: Mode, label: string) => (
    <button
      type="button"
      role="tab"
      aria-selected={mode === m}
      onClick={() => {
        setMode(m);
        save.reset();
      }}
      className={`px-3 h-8 rounded-md text-sm ${mode === m ? "bg-brand text-brand-foreground" : "text-muted-foreground hover:text-foreground"}`}
    >
      {label}
    </button>
  );
  return (
    <Card>
      <CardBody className="ds-stack">
        <div role="tablist" aria-label="残すもの" className="flex gap-1">
          {tab("diary", "日記")}
          {tab("done", "やったこと")}
        </div>
        <form onSubmit={submit} className="flex flex-col sm:flex-row gap-2">
          {mode === "done" && (
            <>
              <label htmlFor="done-project" className="sr-only">
                プロジェクト
              </label>
              <select
                id="done-project"
                value={project}
                onChange={(e) => setProject(e.target.value)}
                className="rounded-md border border-border bg-card px-2 h-10 text-sm sm:max-w-[12rem]"
              >
                {projects.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
                <option value="">プロジェクト外</option>
              </select>
            </>
          )}
          <label htmlFor="diary-line" className="sr-only">
            {mode === "diary" ? "今日の日記（1 行）" : "やったこと（1 行）"}
          </label>
          <input
            id="diary-line"
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={mode === "diary" ? 500 : 200}
            enterKeyHint="send"
            autoComplete="off"
            placeholder={
              mode === "diary"
                ? writtenToday
                  ? "もう 1 行足す"
                  : "今日を 1 行で"
                : "予定になかったことも、そのまま"
            }
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
        {mode === "diary" ? (
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
        ) : (
          <p className="text-xs text-muted-foreground" data-testid="done-split">
            今日の完了 · 予定どおり<span className="font-mono tnum"> {planned} </span>· 予定外
            <span className="font-mono tnum"> {unplanned}</span>
          </p>
        )}
      </CardBody>
    </Card>
  );
}
