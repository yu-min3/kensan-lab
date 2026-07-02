import type { ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "./button";
import { ApiError } from "../../lib/api";

// patterns.md 03. Empty / Loading / Error — 同一画面の3状態。枠は維持し中身だけ入れ替える

// components.md 11. Skeleton — 実レイアウトを模す
export function Skeleton({ className }: { className?: string }) {
  return <div className={`animate-pulse rounded-md bg-muted ${className ?? ""}`} />;
}

export function SkeletonRows({ rows = 4 }: { rows?: number }) {
  return (
    <div className="ds-stack">
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className="ds-row w-full" />
      ))}
    </div>
  );
}

// components.md 10. Empty State — 説明 + 次の一手 + 主アクション の3点セット必須
export function Empty({
  icon,
  title,
  desc,
  actions,
}: {
  icon: ReactNode;
  title: string;
  desc: string;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center text-center py-8 gap-2">
      <div className="text-muted-foreground [&>svg]:size-8">{icon}</div>
      <p className="font-semibold">{title}</p>
      <p className="text-sm text-muted-foreground max-w-md">{desc}</p>
      {actions && <div className="ds-inline mt-2">{actions}</div>}
    </div>
  );
}

// Error — 人間語の説明 + 技術的詳細（エラーコード） + 復旧アクション
export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const status = error instanceof ApiError ? error.status : undefined;
  const message = error instanceof Error ? error.message : String(error);
  return (
    <div className="flex flex-col items-center text-center py-8 gap-2">
      <AlertTriangle className="size-8 text-warning" />
      <p className="font-semibold">読み込みに失敗しました</p>
      <p className="text-sm text-muted-foreground max-w-md">
        kensan backend に接続できないか、エラーが返されました。backend（
        <code className="font-mono text-xs">kensan serve</code>）が起動しているか確認してください。
      </p>
      <p className="font-mono text-xs text-muted-foreground tnum">
        {status ? `HTTP ${status}: ` : ""}
        {message}
      </p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw size={14} />
          再試行
        </Button>
      )}
    </div>
  );
}
