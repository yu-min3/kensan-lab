import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, dailyPath } from "../../lib/api";
import { MarkdownView } from "../MarkdownView";
import { Button } from "../ui/button";
import { Card, CardBody, CardHead } from "../ui/card";
import { ErrorState, SkeletonRows } from "../ui/states";
import { useToast } from "../ui/toast";

const jstDate = () => new Intl.DateTimeFormat("sv-SE", {
  timeZone: "Asia/Tokyo", year: "numeric", month: "2-digit", day: "2-digit",
}).format(new Date());

// Only the diary section; headings inside fenced code are ordinary content.
export function diaryBody(content: string): string {
  const lines: string[] = [];
  let inside = false;
  let fence = "";
  for (const line of content.replace(/\r\n/g, "\n").split("\n")) {
    const marker = line.match(/^ {0,3}(`{3,}|~{3,})/);
    if (marker && !fence) fence = marker[1];
    else if (fence && new RegExp(`^ {0,3}${fence[0]}{${fence.length},}\\s*$`).test(line)) fence = "";
    if (!fence && /^ {0,3}#{1,2}\s/.test(line)) {
      if (inside) break;
      if (/^ {0,3}##\s+日記\s*(?:#+\s*)?$/.test(line)) inside = true;
      continue;
    }
    if (inside) lines.push(line);
  }
  return lines.join("\n").trim();
}

export function TodayDiary() {
  const [today, setToday] = useState(jstDate);
  const [draft, setDraft] = useState({ date: today, text: "" });
  const date = draft.text ? draft.date : today;
  const path = dailyPath(date);
  const client = useQueryClient();
  const toast = useToast();
  useEffect(() => {
    const update = () => setToday(jstDate());
    const timer = window.setInterval(update, 30_000);
    window.addEventListener("focus", update);
    return () => { window.clearInterval(timer); window.removeEventListener("focus", update); };
  }, []);
  const file = useQuery({
    queryKey: ["file", path], queryFn: () => api.file(path), refetchInterval: 30_000,
    retry: (count, error) => !(error instanceof ApiError && error.status === 404) && count < 2,
  });
  const missing = file.error instanceof ApiError && file.error.status === 404;
  const content = diaryBody(file.data?.content ?? "");
  const save = useMutation({
    mutationFn: (entry: { date: string; text: string }) => api.dailyLine(entry.date, entry.text),
    onSuccess: async (_, entry) => {
      setDraft({ date: jstDate(), text: "" });
      setToday(jstDate());
      toast({ title: "日記に追記しました" });
      await Promise.all([
        client.invalidateQueries({ queryKey: ["file", dailyPath(entry.date)] }),
        ...["today", "dailyList", "portfolio"].map(key => client.invalidateQueries({ queryKey: [key] })),
      ]);
    },
  });
  function submit(event: FormEvent) {
    event.preventDefault();
    if (draft.text.trim() && !save.isPending) save.mutate({ date, text: draft.text.trim() });
  }
  return (
    <Card>
      <CardHead title="今日の日記" sub={date} actions={
        <Link to={`/daily?date=${date}`} className="ds-control inline-flex items-center text-sm text-brand">全文を編集</Link>
      } />
      <CardBody className="ds-stack">
        {file.isPending ? <SkeletonRows rows={2} /> : file.isError && !missing ? (
          <ErrorState error={file.error} onRetry={() => file.refetch()} />
        ) : content ? (
          <div aria-label="今日の日記の本文" className="max-h-64 overflow-y-auto" tabIndex={0}>
            <MarkdownView content={content} />
          </div>
        ) : <p className="text-sm text-muted-foreground">今日はまだ書いていません。やりたいことを一つ、ここから。</p>}
        <form onSubmit={submit} className="ds-stack">
          <label htmlFor="dashboard-diary" className="text-sm">今日やりたいこと・いま思っていること</label>
          <div className="flex flex-col sm:flex-row gap-2">
            <input id="dashboard-diary" aria-label="今日の日記（1 行）" value={draft.text}
              onChange={event => setDraft({ date, text: event.target.value })} disabled={save.isPending}
              maxLength={500} enterKeyHint="send" autoComplete="off"
              placeholder="今日は何をしたい？ 一行から残せます"
              className="ds-control min-w-0 sm:flex-1 rounded-md border border-border bg-card px-3 text-base focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
            <Button type="submit" variant="primary" loading={save.isPending} disabled={!draft.text.trim()}>残す</Button>
          </div>
          {date !== today && <p role="status" className="text-sm text-muted-foreground">日付が変わりました。この入力は {date} の日記に残します。</p>}
          {save.isError && <p role="alert" className="text-sm text-destructive">{save.error.message}</p>}
        </form>
      </CardBody>
    </Card>
  );
}
