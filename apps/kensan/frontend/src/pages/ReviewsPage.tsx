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
      <div className="grid grid-cols-[280px_1fr] gap-6 items-start">
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
                      <span className="flex-1 truncate font-mono tnum text-xs">{r.name}</span>
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
