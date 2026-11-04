import type { WatchView } from "./api";

// Matches project-watch-cronjob.yaml: 08–22 JST, every two hours.
// These are expected checks, not a claim that Kubernetes has run a job.
export function watchSchedule(now: number) {
 const day = new Date(now + 9 * 3_600_000).toISOString().slice(0, 10);
 const midnight = Date.parse(`${day}T00:00:00+09:00`);
 const times = [-1, 0, 1].flatMap(offset => Array.from({ length: 8 }, (_, i) => midnight + offset * 86_400_000 + (8 + 2 * i) * 3_600_000));
 return { next: times.find(time => time > now)!, expected: times.filter(time => time <= now - 15 * 60_000).at(-1)! };
}

export function watchStatus(data: WatchView, now: number, readError = false) {
 const schedule = watchSchedule(now);
 const checked = Date.parse(data.state.checkedAt ?? "");
 const overdue = data.state.status !== "not_configured" && (!Number.isFinite(checked) || checked < schedule.expected);
 let label = data.stale ? "新しい記録あり" : "確認済み";
 let warning = false;
 let help = "変更がある時だけAIが要約します。画面は60秒ごとに保存済みの結果を読み込みます。";
 const state = data.state.status;
 if (readError || data.stateError) {
  label = "取得を確認できません"; warning = true;
  help = "最新の状態を取得できません。保存済みの表示を残しています。接続が戻ると自動で再取得します。";
 } else if (state === "not_configured") {
  label = "AI更新は設定前";
  help = "AIの定期実行を設定すると、記録の変化を要約できます。現在は元の記録を表示しています。";
 } else if (state === "waiting") {
  warning = true;
  label = data.state.reasonCode === "quota_wait" ? "利用枠の回復待ち" : "AI更新は要再開";
  help = data.state.reasonCode === "auth_required" ? "AIの再認証後、待機中の依頼を再開してください。"
   : data.state.reasonCode === "quota_wait" ? "利用枠の回復後、待機中の依頼を再開してください。"
   : "AIの認証・利用条件を確認し、待機中の依頼を再開してください。";
  help += "画面を再読み込みするだけでは再開しません。";
 } else if (["failed", "cancelled", "invalid_state", "reset"].includes(state)) {
  label = state === "reset" ? "再生成を準備中" : "AI更新の確認が必要"; warning = state !== "reset";
  help = "前回の結果を保持しています。実行状態を確認し、明示的に再生成してください。";
 } else if (state === "daily_limit") {
  label = "本日の更新上限"; warning = true;
  help = "本日の新規AI依頼の上限に達しました。次の生成機会は明日8時です。";
 } else if (state === "connection_error") {
  label = "AIに接続できません"; warning = true;
  help = "同じ依頼への接続を再確認します。前回の結果は保持しています。";
 } else if (overdue) {
  label = "定期確認が遅れています"; warning = true;
  help = "予定時刻から15分以上、確認の記録が更新されていません。定期実行と接続を確認してください。";
 } else if (state === "queued" || state === "running") {
  label = state === "queued" ? "AI更新を受付中" : "AIで要約中";
 } else if (state === "unchanged" && !data.stale) {
  label = "新しい記録なし";
 } else if (state !== "succeeded" && state !== "unchanged") {
  label = "AI状態を確認してください"; warning = true;
 }
 return { label, warning, help, next: schedule.next, overdue };
}

export function watchTime(value?: string | number) {
 if (value == null || value === "" || (typeof value === "string" && value.startsWith("0001-"))) return "未取得";
 const date = new Date(value);
 return Number.isNaN(date.getTime()) ? "日時不明" : new Intl.DateTimeFormat("ja-JP", { timeZone: "Asia/Tokyo", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(date);
}
