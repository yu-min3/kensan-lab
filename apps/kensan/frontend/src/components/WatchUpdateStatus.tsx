import { useEffect, useState } from "react";
import type { WatchView } from "../lib/api";
import { watchStatus, watchTime } from "../lib/watchStatus";
import { Badge } from "./ui/badge";

export function WatchUpdateStatus({ data, readError = false }: { data: WatchView; readError?: boolean }) {
 const [now, setNow] = useState(Date.now);
 useEffect(() => {
  const update = () => setNow(Date.now());
  const timer = window.setInterval(update, 60_000);
  window.addEventListener("focus", update);
  return () => { window.clearInterval(timer); window.removeEventListener("focus", update); };
 }, []);
 const status = watchStatus(data, now, readError);
 return <div className="ds-stack text-xs text-muted-foreground" aria-label="AI更新の状態">
  <div className="flex flex-wrap items-center gap-2"><Badge variant={status.warning ? "warning" : "muted"} dot>{status.label}</Badge>
   <span>次回確認予定 {watchTime(status.next)} JST</span>
  </div>
  <p>AI生成 {watchTime(data.result?.generatedAt)} · 最終確認 {watchTime(data.state.checkedAt)}</p>
  <details><summary className="cursor-pointer text-brand">{status.warning ? "必要な対応を見る" : "更新のしくみ"}</summary>
   <p className="mt-2">{status.help}</p>
   {status.overdue && status.label !== "定期確認が遅れています" && <p className="mt-2 text-warning">定期確認の記録も予定より遅れています。</p>}
   <p className="mt-2">毎日8〜22時、2時間ごとに確認します。プロジェクトの記録とフィードの差分を要約し、日記は送りません。</p>
  </details>
 </div>;
}
