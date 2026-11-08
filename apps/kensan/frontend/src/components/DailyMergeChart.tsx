import { todayISO } from "../lib/api";

const jstDate = (value: string) => {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : new Intl.DateTimeFormat("sv-SE", { timeZone: "Asia/Tokyo", year: "numeric", month: "2-digit", day: "2-digit" }).format(date);
};

export function DailyMergeChart({ days, week, updatedAt }: { days?: number[]; week?: string; updatedAt?: string }) {
  const today = todayISO();
  const monday = new Date(`${today}T00:00:00Z`);
  monday.setUTCDate(monday.getUTCDate() - (monday.getUTCDay() + 6) % 7);
  const start = new Date(`${week || monday.toISOString().slice(0,10)}T00:00:00Z`);
  const observedThrough = updatedAt ? jstDate(updatedAt) : "";
  const max = Math.max(1, ...(days ?? []));
  return <div className="grid grid-cols-7 gap-2" role="group" aria-label="曜日別のマージPR数">
    {Array.from({ length: 7 }, (_, i) => {
      const date = new Date(start); date.setUTCDate(date.getUTCDate() + i);
      const key = date.toISOString().slice(0,10);
      const future = key > today;
      const count = !future && observedThrough && key <= observedThrough ? days?.[i] : undefined;
      const label = `${key.slice(5).replace("-", "/")}（${"月火水木金土日"[i]}）: ${count == null ? future ? "未到来" : "未取得" : `${count}件`}`;
      return <div key={key} className="flex flex-col items-center gap-1 min-w-0" role="img" aria-label={label} title={label}>
        <span aria-hidden="true" className="font-mono tnum text-xs">{count ?? "—"}</span>
        <div aria-hidden="true" className="h-8 w-full rounded-sm bg-muted overflow-hidden flex items-end">
          <div className="w-full bg-brand" style={{ height: `${count == null ? 0 : count / max * 100}%` }} />
        </div>
        <span aria-hidden="true" className={`text-xs ${key === today ? "text-brand font-semibold" : "text-muted-foreground"}`}>{"月火水木金土日"[i]}</span>
      </div>;
    })}
  </div>;
}
