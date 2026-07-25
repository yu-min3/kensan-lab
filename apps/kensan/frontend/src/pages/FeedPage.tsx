import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, CalendarDays, Newspaper } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import clsx from "clsx";
import { api, type FeedContent } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { FeedSections } from "../components/FeedSections";
import { Badge } from "../components/ui/badge";
import { Card, CardBody, CardHead } from "../components/ui/card";
import { Empty, ErrorState, Skeleton, SkeletonRows } from "../components/ui/states";

function stripFrontmatter(content: string): string {
  const match = content.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/);
  return match ? content.slice(match[0].length).replace(/^\r?\n+/, "") : content;
}

function formatDateTime(value?: string): string {
  if (!value) return "未記録";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("ja-JP", {
    timeZone: "Asia/Tokyo",
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function FeedPage() {
  const [params, setParams] = useSearchParams();
  const requestedDate = params.get("date");
  const list = useQuery({ queryKey: ["feeds"], queryFn: api.feeds });
  const latest = useQuery({ queryKey: ["feeds", "latest"], queryFn: api.latestFeed });

  const selectedDate = requestedDate ?? latest.data?.feed?.date ?? null;
  const selectedEntry = list.data?.feeds.find((entry) => entry.date === selectedDate);
  const selectedIsLatest = !!selectedDate && selectedDate === latest.data?.feed?.date;
  const past = useQuery({
    queryKey: ["feed", selectedEntry?.path],
    queryFn: () => api.file(selectedEntry!.path),
    enabled: !!selectedEntry && !selectedIsLatest,
  });

  let content: FeedContent | null = null;
  if (selectedIsLatest && latest.data?.feed) {
    content = latest.data.feed;
  } else if (selectedEntry && past.data) {
    content = {
      ...selectedEntry,
      generatedAt:
        typeof past.data.doc.meta.extra?.generated_at === "string"
          ? past.data.doc.meta.extra.generated_at
          : undefined,
      content: stripFrontmatter(past.data.content),
    };
  }

  const loading = list.isPending || latest.isPending || (!!selectedEntry && !selectedIsLatest && past.isPending);
  const error = list.error ?? latest.error ?? past.error;

  return (
    <>
      <PageHeader
        eyebrow="今日 · feeds/"
        title="Daily Briefing"
        sub="要対応の通知と、最近の活動・関心に合うニュースを5〜10分で読む朝刊。"
      />

      {latest.data && (latest.data.stale || latest.data.state?.status === "failed" || latest.data.stateError) && (
        <Card className="mb-5 border-warning">
          <CardBody className="flex items-start gap-3">
            <AlertTriangle className="size-5 shrink-0 text-warning mt-0.5" />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-semibold">本日のFeedはまだ更新されていません</p>
                <Badge variant="warning" dot>
                  stale
                </Badge>
              </div>
              <p className="text-sm text-muted-foreground mt-1">
                最新の正常なレポートを表示しています。最終成功:{" "}
                <span className="font-mono tnum">{formatDateTime(latest.data.state?.lastSuccessAt)}</span>
              </p>
              {(latest.data.state?.errorCode || latest.data.stateError) && (
                <p className="font-mono text-xs text-muted-foreground mt-1 break-all">
                  {latest.data.state?.errorCode || "state_error"}:{" "}
                  {latest.data.state?.errorMessage || latest.data.stateError}
                </p>
              )}
            </div>
          </CardBody>
        </Card>
      )}

      <div className="min-w-0 max-w-full grid grid-cols-1 md:grid-cols-[280px_1fr] gap-6 items-start">
        <Card className="min-w-0">
          <CardHead title="過去のFeed" sub="日付を選んで読み返す" />
          <CardBody className="!p-2 max-h-[70vh] overflow-y-auto">
            {list.isPending ? (
              <SkeletonRows rows={7} />
            ) : list.isError ? (
              <ErrorState error={list.error} onRetry={() => list.refetch()} />
            ) : list.data.feeds.length === 0 ? (
              <Empty
                icon={<CalendarDays />}
                title="Feedはまだありません"
                desc="Claude Scheduled Taskとfeed CronJobが成功すると、ここに日付別のレポートが並びます。"
              />
            ) : (
              <ul className="space-y-1">
                {list.data.feeds.map((entry) => (
                  <li key={entry.path}>
                    <button
                      type="button"
                      onClick={() => setParams({ date: entry.date })}
                      className={clsx(
                        "ds-row w-full rounded-md px-2 text-left",
                        selectedDate === entry.date
                          ? "bg-accent text-accent-foreground"
                          : "hover:bg-accent/60",
                      )}
                    >
                      <span className="block font-mono tnum text-sm">{entry.date}</span>
                      <span className="block truncate text-[11px] text-muted-foreground">
                        {entry.title}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </CardBody>
        </Card>

        <Card className="min-w-0 min-h-[70vh] overflow-hidden">
          {loading ? (
            <CardBody className="space-y-5">
              <Skeleton className="h-8 w-2/3" />
              <Skeleton className="h-64 w-full" />
            </CardBody>
          ) : error ? (
            <CardBody>
              <ErrorState error={error} onRetry={() => void Promise.all([list.refetch(), latest.refetch(), past.refetch()])} />
            </CardBody>
          ) : !content ? (
            <CardBody>
              <Empty
                icon={<Newspaper />}
                title="表示できるFeedがありません"
                desc="左の一覧からレポートを選択するか、次回のimport完了を待ってください。"
              />
            </CardBody>
          ) : (
            <>
              <CardHead
                title={content.title}
                sub={`生成 ${formatDateTime(content.generatedAt)}`}
                badge={
                  <Badge variant={selectedIsLatest && !latest.data?.stale ? "success" : "muted"} dot>
                    {selectedIsLatest && !latest.data?.stale ? "latest" : content.date}
                  </Badge>
                }
              />
              <CardBody>
                <FeedSections content={content.content} />
              </CardBody>
            </>
          )}
        </Card>
      </div>
    </>
  );
}
