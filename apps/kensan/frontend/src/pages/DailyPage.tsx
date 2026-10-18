import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { api, todayISO, dailyPath, dailySkeleton } from "../lib/api";
import { Card, CardBody, CardHead } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { MilkdownEditor } from "../components/editors/MilkdownEditor";
import { MarkdownView } from "../components/MarkdownView";
import { DailyCalendar } from "../components/DailyCalendar";
import { useAutosaveFile } from "../hooks/useAutosaveFile";
import { ErrorState, Skeleton } from "../components/ui/states";
import { FileSaveRecovery } from "../components/FileSaveRecovery";
import { SaveStatus } from "../components/ui/save-status";

function shiftDate(date: string, delta: number): string {
  const next = new Date(`${date}T12:00:00Z`);
  next.setUTCDate(next.getUTCDate() + delta);
  return next.toISOString().slice(0, 10);
}

export function DailyPage() {
  const [params, setParams] = useSearchParams();
  const [defaultDate, setDefaultDate] = useState(todayISO);
  const [, setClock] = useState(0);
  useEffect(() => {
    const update = () => setClock(value => value + 1);
    const timer = window.setInterval(update, 30_000);
    window.addEventListener("focus", update);
    return () => { window.clearInterval(timer); window.removeEventListener("focus", update); };
  }, []);
  const requestedDate = params.get("date");
  const date = requestedDate && /^\d{4}-\d{2}-\d{2}$/.test(requestedDate) ? requestedDate : defaultDate;
  const changeDate = (next: string) => {
    if (next === todayISO()) setDefaultDate(next);
    setParams(previous => {
      const result = new URLSearchParams(previous);
      if (next === todayISO()) result.delete("date");
      else result.set("date", next);
      return result;
    });
  };
  return <DailyJournal key={date} date={date} onDate={changeDate} />;
}

// The same free-form document is read and edited here. No separate intention/task schema.
export function DailyJournal({ date, onDate }: { date: string; onDate: (date: string) => void }) {
  const file = useAutosaveFile({ path: dailyPath(date) });
  const client = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [calendar, setCalendar] = useState(false);
  const [closing, setClosing] = useState(false);
  const today = todayISO();
  const create = useMutation({
    mutationFn: () => api.createFile(dailyPath(date), dailySkeleton(date)),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ["file", dailyPath(date)] }),
        client.invalidateQueries({ queryKey: ["dailyList"] }),
      ]);
      setEditing(true);
    },
  });
  async function goDate(next: string) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(next) || next > today || closing) return;
    setClosing(true);
    if (await file.flush()) onDate(next);
    setClosing(false);
  }
  async function finish() {
    setClosing(true);
    if (await file.flush()) setEditing(false);
    setClosing(false);
  }
  const body = file.draft.replace(/^# \d{4}-\d{2}-\d{2}[^\n]*(?:\n|$)/, "").trim();
  return (
    <Card>
      <CardHead title={date === today ? "今日の日記" : `${date} の日記`} sub={date} actions={
        <div className="ds-inline flex-wrap justify-end">
          {file.query.data && <SaveStatus state={file.saveState} />}
          {!file.notFound && file.initialBody !== null && (
            editing ? <Button size="sm" variant="outline" loading={closing} onClick={finish}>編集を終える</Button>
              : <Button size="sm" variant="primary" onClick={() => setEditing(true)}>編集・追記</Button>
          )}
        </div>
      } />
      <CardBody className="ds-stack">
        <div className="ds-inline flex-wrap" aria-label="日記の日付を選ぶ">
          <Button size="sm" variant="ghost" iconOnly aria-label="前日" disabled={closing} onClick={() => goDate(shiftDate(date, -1))}><ChevronLeft size={16} /></Button>
          <input aria-label="日記の日付" type="date" max={today} value={date} disabled={closing} onChange={e => goDate(e.target.value)}
            className="ds-control min-w-0 rounded-md border border-border bg-card px-2 text-sm" />
          <Button size="sm" variant="ghost" iconOnly aria-label="翌日" disabled={date >= today || closing} onClick={() => goDate(shiftDate(date, 1))}><ChevronRight size={16} /></Button>
          {date !== today && <Button size="sm" variant="ghost" disabled={closing} onClick={() => goDate(today)}>今日へ</Button>}
        </div>
        {file.notFound ? (
          <div className="ds-stack items-start">
            <p className="text-sm text-muted-foreground">この日の日記はまだありません。自由に書き始められます。</p>
            <Button variant="primary" loading={create.isPending} onClick={() => create.mutate()}>日記を書く</Button>
            {create.isError && <p role="alert" className="text-sm text-destructive">{create.error.message}</p>}
          </div>
        ) : file.query.isError ? <ErrorState error={file.query.error} onRetry={() => file.query.refetch()} />
          : file.initialBody === null ? <Skeleton className="h-40 w-full" />
          : editing ? (
            <div aria-label="日記を編集">
              <MilkdownEditor key={file.editorKey} defaultValue={file.draft} onChange={file.onChange} minHeight="18rem" placeholder="今日のことを自由に…" />
            </div>
          ) : body ? <div aria-label="今日の日記の本文"><MarkdownView content={body} /></div>
            : <p className="text-sm text-muted-foreground">「編集・追記」から書き始められます。</p>}
        <FileSaveRecovery file={file} />
        <details open={calendar} onToggle={e => setCalendar(e.currentTarget.open)}>
          <summary className="cursor-pointer text-sm text-muted-foreground">カレンダー・過去の日記</summary>
          {calendar && <div className="mt-3 max-w-lg"><DailyCalendar selected={date} onSelect={goDate} /></div>}
        </details>
      </CardBody>
    </Card>
  );
}
