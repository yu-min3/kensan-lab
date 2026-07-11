import { Link, useLocation } from "react-router-dom";
import { Compass } from "lucide-react";
import { Card, CardBody } from "../components/ui/card";
import { Empty } from "../components/ui/states";

// 未定義ルートの受け皿。SPA fallback で index.html は返るため、ここが無いと
// 「白画面 + console warning だけ」になる（手打ち URL・古いブックマークで踏む）。
export function NotFoundPage() {
  const { pathname } = useLocation();
  return (
    <Card>
      <CardBody>
        <Empty
          icon={<Compass />}
          title="ページが見つかりません"
          desc={`「${pathname}」に対応する画面はありません。URL が変わったか、打ち間違いの可能性があります。`}
          actions={
            <Link
              to="/"
              className="ds-control inline-flex items-center rounded-lg bg-brand text-brand-foreground text-sm px-3.5 font-medium hover:opacity-90"
            >
              ダッシュボードへ戻る
            </Link>
          }
        />
      </CardBody>
    </Card>
  );
}
