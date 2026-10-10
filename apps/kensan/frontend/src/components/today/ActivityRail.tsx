import { Link } from "react-router-dom";
import type { TodayView } from "../../lib/api";
import { Card, CardHead, CardBody } from "../ui/card";
import { ActivityHistory } from "./ActivityHistory";

export function ActivityRail({ view }: { view: TodayView }) {
  const counts = new Map<string, number>();
  for (const t of view.board.later ?? [])
    if (t.state === "todo")
      counts.set(
        t.project || "その他",
        (counts.get(t.project || "その他") || 0) + 1,
      );
  const inventory = [...counts].sort((a, b) => b[1] - a[1]);
  const max = Math.max(1, ...counts.values());
  const total = view.activity.reduce((sum, d) => sum + d.count, 0);
  return (
    <aside className="ds-section min-w-0">
      <Card>
        <CardHead title="積み上げ" sub="直近126日 · 完了と習慣の実績" />
        <CardBody className="ds-stack">
          <div className="font-mono tnum text-3xl">
            {total}
            <span className="text-sm text-muted-foreground ml-2">回の一歩</span>
          </div>
          <ActivityHistory view={view} />
          <p className="text-xs text-muted-foreground">
            {view.recordedSince
              ? `${view.recordedSince} から記録。空白は過去の未実施を意味しません。`
              : "チェックを付けた日から記録が始まります。過去の実績は推測しません。"}
          </p>
        </CardBody>
      </Card>
      <Card>
        <CardHead title="続いていること" />
        <CardBody className="ds-stack">
          {view.routines.length ? (
            view.routines.map((r) => (
              <div
                className="flex items-baseline justify-between gap-2"
                key={r.id}
              >
                <span className="text-sm">{r.text.split("（")[0]}</span>
                <span className="font-mono tnum whitespace-nowrap">
                  {r.supported ? `${r.streak}${r.unit}` : "判定保留"}
                </span>
              </div>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">
              プロジェクトのルーティンを登録すると、ここで継続を確認できます。
            </p>
          )}
        </CardBody>
      </Card>
      <Card>
        <CardHead
          title="溜まり具合"
          sub="いつかのタスク · 件数による警告はしません"
        />
        <CardBody className="ds-stack">
          {inventory.map(([name, n]) => (
            <Link
              to="/tasks"
              className="grid grid-cols-2 items-center gap-2 text-xs"
              key={name}
            >
              <span>{name}</span>
              <span className="flex items-center gap-2">
                <span className="bg-muted h-1.5 flex-1 rounded-full">
                  <span
                    className="block bg-brand/60 h-full rounded-full"
                    style={{ width: `${(n / max) * 100}%` }}
                  />
                </span>
                <span className="font-mono tnum">{n}</span>
              </span>
            </Link>
          ))}
          <Link to="/tasks" className="text-sm text-brand">
            全タスクを開く →
          </Link>
        </CardBody>
      </Card>
    </aside>
  );
}
