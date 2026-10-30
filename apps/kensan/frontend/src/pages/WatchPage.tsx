import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { AlertTriangle, ExternalLink } from "lucide-react";
import { api, todayISO, type WatchView } from "../lib/api";
import { TodayDiary } from "../components/today/TodayDiary";
import { Card, CardBody, CardHead } from "../components/ui/card";
import { ErrorState, SkeletonRows } from "../components/ui/states";
import { splitFeedItems, splitFeedSections } from "../components/FeedSections";
import { MarkdownView } from "../components/MarkdownView";

const poll = { refetchInterval: 60_000, refetchIntervalInBackground: true } as const;
function stamp(value?: string) {
 if (!value) return "未確認";
 const date = new Date(value); if (Number.isNaN(date.getTime())) return "日時不明";
 return new Intl.DateTimeFormat("ja-JP", { timeZone: "Asia/Tokyo", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(date);
}
const labels: Record<string, string> = {
 not_configured: "AI更新は設定前", queued: "AI更新を受付中", running: "AIで要約中", waiting: "AI更新は待機中",
 succeeded: "AI要約を更新済み", unchanged: "新しい記録なし", connection_error: "AI接続を再確認中",
 failed: "AI更新に失敗", cancelled: "AI更新を取消", daily_limit: "本日のAI更新上限", invalid_state: "AI更新状態を確認してください", reset: "AI再生成を準備中",
};
export function WatchPage() {
 const watch = useQuery({ queryKey: ["watch"], queryFn: api.watch, ...poll });
 const feed = useQuery({ queryKey: ["feeds", "latest"], queryFn: api.latestFeed, ...poll });
 const ack = useQuery({ queryKey: ["feeds", "acknowledgements"], queryFn: api.feedAcknowledgements, ...poll });
 const sections = splitFeedSections(feed.data?.feed?.content ?? "");
 const acknowledged = new Map(ack.data?.items.map(item => [item.key, item.version]) ?? []);
 const inbox = sections.filter(s => ["要対応", "確認", "要確認", "確認待ち"].includes(s.title)).flatMap(s => splitFeedItems(s.content));
 const unconfirmed = inbox.filter(item => acknowledged.get(item.key) !== item.version);
 const news = sections.filter(s => ["今日のニュース", "今日の5本", "リリース・定点観測"].includes(s.title)).flatMap(s => splitFeedItems(s.content));
 const items = unconfirmed.length ? unconfirmed : news;
 return <div className="ds-section">
   <div className="flex justify-between items-baseline gap-3 flex-wrap">
     <h1 className="h-serif text-xl font-semibold">今日と、最近の変化</h1>
     <span className="font-mono text-xs text-muted-foreground">{todayISO()} · JST</span>
   </div>
   <TodayDiary />
   <div className="grid grid-cols-1 md:grid-cols-2 gap-4 items-start">
     <Card><CardHead title="プロジェクトの変化" sub="元の記録と、根拠付きのAI要約" />
       <CardBody className="ds-stack">
         {watch.isPending ? <SkeletonRows rows={4} /> : watch.isError && !watch.data ? <ErrorState error={watch.error} onRetry={() => watch.refetch()} /> : watch.data ? <ProjectChanges data={watch.data} /> : null}
         {watch.isError && watch.data && <p className="text-sm text-warning" role="status">更新を取得できません。前回の表示を残しています。</p>}
       </CardBody>
     </Card>
     <Card><CardHead title={unconfirmed.length ? "まだ確認していないもの" : "新着フィード"} sub={feed.data?.feed ? `生成 ${stamp(feed.data.feed.generatedAt)} · ${feed.data.feed.date}` : "取り込み済みの情報から表示"} />
       <CardBody className="ds-stack">
         {feed.isPending ? <SkeletonRows rows={4} /> : feed.isError && !feed.data ? <ErrorState error={feed.error} onRetry={() => feed.refetch()} /> : <>
           {(feed.isError || feed.data?.stale || feed.data?.stateError) && <p className="text-sm text-warning flex items-start gap-2" role="status"><AlertTriangle size={16} className="shrink-0" />最新の取得を確認できません。前回成功分を表示しています。</p>}
           {ack.isError && <p className="text-sm text-warning">確認済み状態を取得できません。未確認の件数は確定していません。</p>}
           {items.length ? <ul className="divide-y divide-border">{items.slice(0,3).map(item => <li key={item.key} className="py-3 first:pt-0 ds-stack">
             <h2 className="font-semibold text-sm">{item.title}</h2>
             <div className="text-sm text-muted-foreground line-clamp-3"><MarkdownView content={item.content} /></div>
           </li>)}</ul> : <p className="text-sm text-muted-foreground">{feed.data?.feed ? "表示する新着項目はありません。" : "フィードはまだありません。"}</p>}
           {unconfirmed.length > 0 && !ack.isError && <p className="text-xs text-muted-foreground">未確認 {unconfirmed.length}件 · 表示しただけでは確認済みになりません</p>}
         </>}
         <Link to="/feed" className="text-sm text-brand">フィード全文を開く →</Link>
       </CardBody>
     </Card>
   </div>
 </div>;
}
function ProjectChanges({ data }: { data: WatchView }) {
 const { state, result } = data;
 const unhealthy = ["failed", "connection_error", "invalid_state", "waiting", "daily_limit"].includes(state.status) || !!data.stateError;
 const sourceById = new Map(result?.sources.map(s => [s.id,s]) ?? []);
 return <>
   <div className="text-xs text-muted-foreground ds-stack" role="status">
     <p className={unhealthy ? "text-warning" : undefined}>{unhealthy ? "要確認 · " : ""}{labels[state.status] ?? "AI更新状態を確認してください"}</p>
     <p>最終確認 {stamp(state.checkedAt)} ／ AI生成 {stamp(result?.generatedAt)}</p>
     {(state.reason || data.stateError) && <p>{data.stateError ?? state.reason}</p>}
     {data.stale && <p className="text-warning">AI要約のあとに記録が変わっています。下の元の記録が最新です。</p>}
   </div>
   {result && result.items.length > 0 && <details open={!data.stale}><summary className="text-sm text-brand cursor-pointer mb-2">{data.stale ? "前回成功したAI要約" : "AIによる変化の要約"}</summary><section aria-label="AIによる変化の要約" className="border-l-2 border-brand pl-3 ds-stack">
     {result.items.slice(0,3).map((item, i) => <article key={`${item.project}-${i}`} className="ds-stack">
       <span className="font-mono text-xs text-muted-foreground">{item.project || "フィード"} · AI要約</span>
       <p className="text-sm font-medium">{item.change}</p>
       {item.meaning && <p className="text-sm text-muted-foreground">{item.meaning}</p>}
       {item.decision && <p className="text-sm text-warning">判断待ち：{item.decision}</p>}
       <div className="flex gap-3 flex-wrap">{item.sourceIds.map(id => {
         const source = sourceById.get(id); return source ? <Link key={id} to={source.project ? `/#project-${encodeURIComponent(source.project)}` : "/feed"} className="text-xs text-brand inline-flex items-center gap-1">根拠：{source.project || "フィード"} {source.date}<ExternalLink size={11} /></Link> : null;
       })}</div>
     </article>)}
   </section></details>}
   <details open={data.stale || !result?.items.length}><summary className="text-xs text-muted-foreground cursor-pointer mb-2">元の記録 · 最近の3件</summary>
   {data.snapshot.projects.length ? <ul className="divide-y divide-border">{data.snapshot.projects.slice(0,3).map(p => <li key={p.name} className="py-3 first:pt-0 ds-stack">
     <div className="flex justify-between gap-2 text-xs text-muted-foreground"><Link to={`/#project-${encodeURIComponent(p.name)}`} className="font-mono text-brand">{p.name}</Link><span>{p.date ? `記録 ${p.date}` : "日付付きログなし"}</span></div>
     <div className="text-sm line-clamp-3"><MarkdownView content={p.text || "新しい記録はありません。"} /></div>
     {p.next && <p className="text-xs text-muted-foreground line-clamp-2">次の節目：{p.next}</p>}
   </li>)}</ul> : <p className="text-sm text-muted-foreground">表示するアクティブなプロジェクトがありません。</p>}</details>
 </>;
}
