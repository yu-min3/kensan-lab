import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api, ApiError, dailyPath } from "../../lib/api";
import { MarkdownView } from "../MarkdownView";
import { Card, CardBody, CardHead } from "../ui/card";
import { ErrorState, SkeletonRows } from "../ui/states";
import { splitFrontmatter } from "../../hooks/useAutosaveFile";

const jstDate = () => new Intl.DateTimeFormat("sv-SE", {
  timeZone: "Asia/Tokyo", year: "numeric", month: "2-digit", day: "2-digit",
}).format(new Date());

export function TodayDiary() {
  const [date, setDate] = useState(jstDate);
  const path = dailyPath(date);
  useEffect(() => {
    const update = () => setDate(jstDate());
    const timer = window.setInterval(update, 30_000);
    window.addEventListener("focus", update);
    return () => { window.clearInterval(timer); window.removeEventListener("focus", update); };
  }, []);
  const file = useQuery({
    queryKey: ["file", path], queryFn: () => api.file(path), refetchInterval: 30_000,
    retry: (count, error) => !(error instanceof ApiError && error.status === 404) && count < 2,
  });
  const missing = file.error instanceof ApiError && file.error.status === 404;
  const content = splitFrontmatter(file.data?.content ?? "").body
    .replace(/^# \d{4}-\d{2}-\d{2}[^\n]*(?:\n|$)/, "").trim();
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
        ) : <p className="text-sm text-muted-foreground">今日の日記はまだありません。「全文を編集」から書けます。</p>}
      </CardBody>
    </Card>
  );
}
