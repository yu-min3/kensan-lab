import { useState } from "react";
import type { TodayView } from "../../lib/api";

// 色の俯瞰と、キーボード／タッチで読める日別値を同じデータから描く。
export function ActivityHistory({ view }: { view: TodayView }) {
  const [selected, setSelected] = useState("");
  const first = view.activity[0]?.date ?? view.date;
  const date =
    selected && selected >= first && selected <= view.date
      ? selected
      : view.date;
  const value = view.activity.find((day) => day.date === date)?.count ?? 0;
  const observed = view.recordedSince && date >= view.recordedSince;
  const padding = new Date(`${first}T00:00:00Z`).getUTCDay();
  return (
    <div className="ds-stack">
      <div className="flex justify-between text-xs text-muted-foreground font-mono tnum">
        <span>{first.slice(5)}</span>
        <span>{view.date.slice(5)}</span>
      </div>
      <div className="grid grid-rows-7 grid-flow-col gap-1" aria-hidden="true">
        {Array.from({ length: padding }, (_, i) => (
          <span key={`pad-${i}`} />
        ))}
        {view.activity.map((day) => (
          <span
            key={day.date}
            title={`${day.date} · ${day.count}件`}
            className={`aspect-square rounded-sm ${day.count >= 4 ? "bg-brand" : day.count >= 2 ? "bg-brand/60" : day.count ? "bg-brand/30" : "bg-muted border border-border"}`}
          />
        ))}
      </div>
      <p className="text-xs text-muted-foreground">
        各列は上から日〜土。色が濃いほど記録が多い日です。
      </p>
      <div
        className="flex flex-wrap gap-3 text-xs text-muted-foreground"
        aria-label="記録数の凡例"
      >
        {[
          { label: "0件", style: "bg-muted border border-border" },
          { label: "1件", style: "bg-brand/30" },
          { label: "2–3件", style: "bg-brand/60" },
          { label: "4件以上", style: "bg-brand" },
        ].map((item) => (
          <span key={item.label} className="inline-flex items-center gap-1">
            <span className={`h-3 w-3 rounded-sm ${item.style}`} />
            {item.label}
          </span>
        ))}
      </div>
      <label className="ds-stack text-xs text-muted-foreground">
        日別の記録を確認
        <input
          type="date"
          value={date}
          min={first}
          max={view.date}
          onChange={(e) => setSelected(e.target.value)}
          className="ds-control w-full min-w-0 rounded-md border border-border bg-card px-2 font-mono tnum focus-visible:ring-2 focus-visible:ring-ring"
        />
      </label>
      <p role="status" className="text-sm">
        {date} · {observed ? `${value}件の記録` : "記録開始前"}
      </p>
    </div>
  );
}
