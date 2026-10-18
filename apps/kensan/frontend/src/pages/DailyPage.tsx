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
  const lineKey = `kensan:diary-line:${date}`;
  const [line, setLineState] = useState(() => {
    try { return sessionStorage.getItem(lineKey) ?? ""; } catch { return ""; }
  });
  function setLine(text: string) {
    setLineState(text);
    try {
      if (text) sessionStorage.setItem(lineKey, text);
      else sessionStorage.removeItem(lineKey);
    } catch { /* Storage may be unavailable. */ }
  }
  function clearLine(submitted: string) {
    setLineState("");
    try {
      if (sessionStorage.getItem(lineKey)?.trim() === submitted) sessionStorage.removeItem(lineKey);
    } catch { /* Storage may be unavailable. */ }
  }
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
  const append = useMutation({
    mutationFn: async (text: string) => {
      if (file.notFound) {
        await api.createFile(dailyPath(date), `${dailySkeleton(date).trimEnd()}\n\n${text}\n`);
        await Promise.all([
          client.invalidateQueries({ queryKey: ["file", dailyPath(date)] }),
          client.invalidateQueries({ queryKey: ["dailyList"] }),
        ]);
        clearLine(text);
      } else {
        // Share the editor's save queue and conflict recovery. No second writer.
        const saving = file.replace(`${file.draft.trimEnd()}\n\n${text}\n`);
        clearLine(text); // The line now belongs to the recoverable document draft.
        await saving;
      }
    },
  });
  const busy = closing || append.isPending;
  async function goDate(next: string) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(next) || next > today || busy) return;
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
              : <Button size="sm" variant="primary" disabled={busy} onClick={() => setEditing(true)}>編集・追記</Button>
          )}
        </div>
      } />
      <CardBody className="ds-stack">
        <div className="ds-inline flex-wrap" aria-label="日記の日付を選ぶ">
          <Button size="sm" variant="ghost" iconOnly aria-label="前日" disabled={busy} onClick={() => goDate(shiftDate(date, -1))}><ChevronLeft size={16} /></Button>
          <input aria-label="日記の日付" type="date" max={today} value={date} disabled={busy} onChange={e => goDate(e.target.value)}
            className="ds-control min-w-0 rounded-md border border-border bg-card px-2 text-sm" />
          <Button size="sm" variant="ghost" iconOnly aria-label="翌日" disabled={date >= today || busy} onClick={() => goDate(shiftDate(date, 1))}><ChevronRight size={16} /></Button>
          {date !== today && <Button size="sm" variant="ghost" disabled={busy} onClick={() => goDate(today)}>今日へ</Button>}
        </div>
        {file.notFound ? (
          <div className="ds-stack items-start">
            <p className="text-sm text-muted-foreground">この日の日記はまだありません。自由に書き始められます。</p>
            <Button variant="primary" loading={create.isPending} disabled={busy} onClick={() => create.mutate()}>日記を書く</Button>
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
        {!editing && (file.notFound || file.initialBody !== null) && <form onSubmit={event => {
          event.preventDefault();
          if (line.trim() && !busy && !file.saveError) append.mutate(line.trim());
        }} className="ds-stack">
          <label htmlFor="diary-line" className="text-sm">今日やりたいこと・いま思っていること</label>
          <div className="flex flex-col sm:flex-row gap-2">
            <input id="diary-line" aria-label="日記（1 行）" value={line}
              onChange={event => setLine(event.target.value)} disabled={busy}
              onKeyDown={event => {
                if (event.key === "Enter" && (event.nativeEvent.isComposing || event.keyCode === 229)) event.preventDefault();
              }}
              maxLength={500} enterKeyHint="send" autoComplete="off"
              placeholder="一行から気軽に残せます"
              className="ds-control min-w-0 sm:flex-1 rounded-md border border-border bg-card px-3 text-base focus:outline-none focus-visible:ring-2 focus-visible:ring-ring" />
            <Button type="submit" variant="outline" loading={append.isPending}
              disabled={busy || !line.trim() || !!file.saveError}>残す</Button>
          </div>
          {append.isError && <p role="alert" className="text-sm text-destructive">{append.error.message}</p>}
        </form>}
        <FileSaveRecovery file={file} />
        <details open={calendar} onToggle={e => setCalendar(e.currentTarget.open)}>
          <summary className="cursor-pointer text-sm text-muted-foreground">カレンダー・過去の日記</summary>
          {calendar && <div className="mt-3 max-w-lg"><DailyCalendar selected={date} onSelect={goDate} /></div>}
        </details>
      </CardBody>
    </Card>
  );
}
