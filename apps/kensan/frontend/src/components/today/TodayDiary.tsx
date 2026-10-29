import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, ApiError, dailyPath, dailySkeleton, todayISO } from "../../lib/api";
import { readIntentions, writeIntentions } from "../../lib/dailyIntentions";
import { Card, CardBody, CardHead } from "../ui/card";
import { Button } from "../ui/button";
import { ErrorState, Skeleton } from "../ui/states";
import { MarkdownView } from "../MarkdownView";

export function TodayDiary() {
  const [date, setDate] = useState(todayISO);
  useEffect(() => {
    const update = () => setDate(todayISO());
    const timer = window.setInterval(update, 30_000);
    window.addEventListener("focus", update);
    return () => { window.clearInterval(timer); window.removeEventListener("focus", update); };
  }, []);
  return <TodayIntentions key={date} date={date} />;
}

function TodayIntentions({ date }: { date: string }) {
  const path = dailyPath(date);
  const client = useQueryClient();
  const file = useQuery({ queryKey: ["file", path], queryFn: () => api.file(path),
    retry: (count, error) => !(error instanceof ApiError && error.status === 404) && count < 2,
    refetchInterval: 60_000 });
  const missing = file.error instanceof ApiError && file.error.status === 404;
  const storageKey = `kensan:intentions:${date}`;
  const [recoverable, setRecoverable] = useState(() => {
    try { return !!sessionStorage.getItem(storageKey); } catch { return false; }
  });
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [base, setBase] = useState<{ content: string; mtime: string } | null>(null);
  const save = useMutation({
    mutationFn: async () => {
      if (!base) throw new Error("読み込み後に編集してください。");
      const content = writeIntentions(base.content, draft);
      if (base.mtime) await api.putFile(path, content, base.mtime);
      else await api.createFile(path, content);
    },
    onSuccess: async () => {
      await Promise.all([client.invalidateQueries({ queryKey: ["file", path] }), client.invalidateQueries({ queryKey: ["dailyList"] })]);
      try { sessionStorage.removeItem(storageKey); } catch { /* Optional recovery cache. */ }
      setRecoverable(false);
      setEditing(false);
    },
  });
  function edit() {
    const content = file.data?.content ?? dailySkeleton(date);
    let text = readIntentions(content);
    let mtime = file.data?.doc.mtime ?? "";
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) ?? "null");
      if (saved && typeof saved.text === "string" && typeof saved.mtime === "string") {
        text = saved.text;
        mtime = saved.mtime; // Do not overwrite a changed file after recovering old input.
      }
    } catch { /* Invalid recovery data must not block editing. */ }
    setBase({ content, mtime });
    setDraft(text);
    save.reset();
    setEditing(true);
  }
  const intentions = readIntentions(file.data?.content ?? "");
  return <Card>
    <CardHead title="今日やりたいこと" sub={date} actions={
      <Link to="/daily" className="text-sm text-brand underline underline-offset-2">日記を開く</Link>
    } />
    <CardBody className="ds-stack">
      {editing ? <form className="ds-stack" onSubmit={event => { event.preventDefault(); if (!save.isPending) save.mutate(); }}>
        <label htmlFor="today-intentions" className="text-sm">今日やりたいこと（ダッシュボードに表示）</label>
        <textarea id="today-intentions" value={draft} onChange={event => {
            const text = event.target.value;
            setDraft(text);
            try { sessionStorage.setItem(storageKey, JSON.stringify({ text, mtime: base?.mtime ?? "" })); setRecoverable(true); } catch { /* Optional recovery cache. */ }
          }} disabled={save.isPending}
          rows={4} className="w-full rounded-md border border-border bg-card p-3 text-base focus-visible:ring-2 focus-visible:ring-ring" />
        <p className="text-xs text-muted-foreground">日記の「今日やりたいこと」欄へ保存します。</p>
        <div className="ds-inline">
          <Button type="submit" variant="primary" loading={save.isPending}>保存する</Button>
          <Button type="button" variant="ghost" disabled={save.isPending} onClick={() => {
            try { sessionStorage.removeItem(storageKey); } catch { /* Optional recovery cache. */ }
            setRecoverable(false); setEditing(false);
          }}>キャンセル</Button>
        </div>
        {save.isError && <div className="ds-stack">
          <p role="alert" className="text-sm text-destructive">保存できませんでした。入力は残っています。最新の内容を取り込んでから、保存をやり直せます。</p>
          <Button type="button" variant="outline" onClick={async () => {
            const latest = await file.refetch();
            if (latest.isError || !latest.data) return;
            const mtime = latest.data.doc.mtime;
            setBase({ content: latest.data.content, mtime });
            try { sessionStorage.setItem(storageKey, JSON.stringify({ text: draft, mtime })); } catch { /* Optional recovery cache. */ }
            save.reset();
          }}>入力を残して最新の内容を取り込む</Button>
        </div>}
      </form> : file.isPending ? <Skeleton className="h-20 w-full" />
        : file.isError && !missing ? <ErrorState error={file.error} onRetry={() => file.refetch()} />
        : <>
          {intentions ? <div aria-label="今日やりたいことの本文"><MarkdownView content={intentions} /></div>
            : <p className="text-sm text-muted-foreground">今日はまだ書いていません。ここに書いたことだけをダッシュボードへ表示します。</p>}
          {recoverable && <p className="text-sm text-muted-foreground">未保存の入力があります。編集を開くと続きから書けます。</p>}
          <div><Button variant="outline" onClick={edit}>{intentions ? "今日やりたいことを編集" : "今日やりたいことを書く"}</Button></div>
        </>}
    </CardBody>
  </Card>;
}
