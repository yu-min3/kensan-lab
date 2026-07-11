import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { ChartSpline } from "lucide-react";
import clsx from "clsx";
import { api, reviewContentURL, type ReviewEntry } from "../lib/api";
import { PageHeader } from "../components/PageHeader";
import { Card, CardBody } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Empty, ErrorState, SkeletonRows } from "../components/ui/states";

// レビュー閲覧 — /weekly-review と /reflection が生成した HTML をそのまま表示する
// （可視化の進化は Claude 側の HTML 生成が担う、という unification-plan.md の契約）。
// patterns.md 06. List · Detail。

const KIND_LABEL: Record<ReviewEntry["kind"], string> = {
  weekly: "週次",
  daily: "日次",
  monthly: "月次",
  other: "他",
};

// ファイル名（W24.html / 14.html / 04-monthly.md）を人が読める表示にする。
// 年・月日は path から拾う（日次の "14.html" だけでは何月か分からないため）。
function reviewLabel(r: ReviewEntry): string {
  const bare = r.name.replace(/\.[^.]+$/, "");
  const year = r.path.match(/(?:^|\/)(\d{4})(?:\/|$)/)?.[1];
  if (r.kind === "weekly") {
    const w = bare.match(/W0*(\d+)/i)?.[1];
    return w ? `${year ? `${year} ` : ""}第${w}週` : bare;
  }
  if (r.kind === "monthly") {
    const mo = bare.match(/0*(\d{1,2})-monthly/)?.[1];
    return mo ? `${year ? `${year}年` : ""}${mo}月` : bare;
  }
  if (r.kind === "daily") {
    const md = r.path.match(/(\d{4})\/(\d{2})\/(\d{2})/);
    if (md) return `${Number(md[2])}/${Number(md[3])}`;
    const d = bare.match(/0*(\d{1,2})/)?.[1];
    return d ? `${d}日` : bare;
  }
  return bare;
}

export function ReviewsPage() {
  const [params, setParams] = useSearchParams();
  const selected = params.get("path");
  const reviews = useQuery({ queryKey: ["reviews"], queryFn: api.reviews });

  return (
    <>
      <PageHeader
        eyebrow="レビュー · reviews/"
        title="レビュー"
        sub="週次・日次の振り返り。生成は Claude Code の /weekly-review と /reflection が行う。"
      />
      <div className="grid grid-cols-1 md:grid-cols-[280px_1fr] gap-6 items-start">
        <Card>
          <CardBody className="!p-2">
            {reviews.isPending ? (
              <SkeletonRows rows={6} />
            ) : reviews.isError ? (
              <ErrorState error={reviews.error} onRetry={() => reviews.refetch()} />
            ) : reviews.data.reviews.length === 0 ? (
              <Empty
                icon={<ChartSpline />}
                title="レビューがまだありません"
                desc="Claude Code で /weekly-review を実行すると、ここに HTML レポートが並びます。"
              />
            ) : (
              <ul>
                {reviews.data.reviews.map((r) => (
                  <li key={r.path}>
                    <button
                      onClick={() => setParams({ path: r.path })}
                      className={clsx(
                        "ds-row w-full flex items-center gap-2 px-2 rounded-md text-left text-sm",
                        selected === r.path ? "bg-accent text-accent-foreground" : "hover:bg-accent/60",
                      )}
                    >
                      <Badge variant={r.kind === "weekly" ? "brand" : "muted"}>{KIND_LABEL[r.kind]}</Badge>
                      <span className="flex-1 truncate text-sm" title={r.name}>{reviewLabel(r)}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </CardBody>
        </Card>
        <Card className="min-h-[70vh] overflow-hidden">
          {selected ? (
            <iframe
              title={`レビュー: ${selected}`}
              src={reviewContentURL(selected)}
              sandbox="allow-scripts"
              className="w-full h-[80vh] border-0"
            />
          ) : (
            <CardBody>
              <Empty
                icon={<ChartSpline />}
                title="レビューを選択してください"
                desc="左の一覧から開きたいレビューを選ぶと、ここに表示されます。"
              />
            </CardBody>
          )}
        </Card>
      </div>
    </>
  );
}
