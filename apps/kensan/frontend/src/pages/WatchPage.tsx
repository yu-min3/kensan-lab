import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Activity, AlertTriangle, ArrowUpRight, Clock3, ExternalLink, Inbox, Layers3, Sparkles } from "lucide-react";
import { api, todayISO, type WatchView } from "../lib/api";
import { watchExcerpt } from "../lib/watchExcerpt";
import { TodayDiary } from "../components/today/TodayDiary";
import { Badge } from "../components/ui/badge";
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
 const feedUnknown = !feed.data || !ack.data || feed.isError || ack.isError || !!feed.data.stale || !!feed.data.stateError;
 const count = feedUnknown ? "—" : unconfirmed.length;
 const data = watch.data;
 return <div className="ds-section min-w-0">
   <div className="flex justify-between items-baseline gap-3 flex-wrap">
     <h1 className="h-serif text-xl font-semibold">今日と、最近の変化</h1>
     <span className="font-mono text-xs text-muted-foreground">{todayISO()} · JST</span>
   </div>
   <div className="grid grid-cols-3 gap-3" aria-label="ひと目で確認">
     <Card className="min-w-0"><CardBody className="ds-stack"><Layers3 size={20} className="text-brand" aria-hidden="true" /><p className="text-xs text-muted-foreground">プロジェクト</p><p className="font-mono tnum text-3xl">{watch.isError ? "—" : data?.snapshot.projects.length ?? "—"}</p></CardBody></Card>
     <Card className="min-w-0"><CardBody className="ds-stack"><Inbox size={20} className={unconfirmed.length ? "text-warning" : "text-muted-foreground"} aria-hidden="true" /><p className="text-xs text-muted-foreground">未確認</p><p className={`font-mono tnum text-3xl ${unconfirmed.length && !feedUnknown ? "text-warning" : ""}`}>{count}</p></CardBody></Card>
     <Card className="min-w-0"><CardBody className="ds-stack"><Activity size={20} className="text-muted-foreground" aria-hidden="true" /><p className="text-xs text-muted-foreground">記録とAI要約</p><p className={`font-semibold ${watch.isError || data?.stale || data?.stateError ? "text-warning" : "text-muted-foreground"}`}>{watch.isError ? "取得失敗" : !data ? "読込中" : data.stateError ? "要確認" : data.stale ? "記録更新あり" : data.result ? "最新記録と一致" : "元の記録を表示"}</p></CardBody></Card>
   </div>
   <TodayDiary />
   <div className="grid grid-cols-1 md:grid-cols-2 gap-4 items-start min-w-0">
     <Card className="min-w-0"><CardHead title="プロジェクトの変化" badge={<Layers3 size={16} className="text-brand" />} />
       <CardBody className="ds-stack">
         {watch.isPending ? <SkeletonRows rows={4} /> : watch.isError && !data ? <ErrorState error={watch.error} onRetry={() => watch.refetch()} /> : data ? <ProjectChanges data={data} /> : null}
         {watch.isError && data && <p className="text-sm text-warning" role="status">更新を取得できません。前回の表示を残しています。</p>}
       </CardBody>
     </Card>
     <Card className="min-w-0"><CardHead title={unconfirmed.length ? "まだ確認していないもの" : "新着フィード"} badge={<Badge variant={unconfirmed.length ? "warning" : "muted"}>{feedUnknown ? "件数未確認" : `${unconfirmed.length || news.length}件`}</Badge>} />
       <CardBody className="ds-stack">
         {feed.isPending ? <SkeletonRows rows={4} /> : feed.isError && !feed.data ? <ErrorState error={feed.error} onRetry={() => feed.refetch()} /> : <>
           {(feed.isError || feed.data?.stale || feed.data?.stateError) && <p className="text-sm text-warning flex items-start gap-2" role="status"><AlertTriangle size={16} className="shrink-0" />最新の取得を確認できません。前回成功分を表示しています。</p>}
           {ack.isError && <p className="text-sm text-warning">確認済み状態を取得できません。未確認の件数は確定していません。</p>}
           {items.length ? <ol className="divide-y divide-border">{items.slice(0,3).map((item, i) => <li key={item.key} className="py-3 first:pt-0 flex gap-3 min-w-0">
             <span className="font-mono tnum text-xl text-muted-foreground" aria-hidden="true">{String(i+1).padStart(2,"0")}</span>
             <div className="min-w-0 ds-stack"><h2 className="font-semibold text-base leading-snug line-clamp-2 [overflow-wrap:anywhere]">{item.title}</h2>
               <p className="text-sm text-muted-foreground line-clamp-2 [overflow-wrap:anywhere]">{watchExcerpt(item.content)}</p>
             </div>
           </li>)}</ol> : <p className="text-sm text-muted-foreground">{feed.data?.feed ? "表示する新着項目はありません。" : "フィードはまだありません。"}</p>}
           <p className="text-xs text-muted-foreground inline-flex gap-1 items-center"><Clock3 size={12} />生成 {stamp(feed.data?.feed?.generatedAt)}</p>
         </>}
         <Link to="/feed" className="text-sm text-brand inline-flex gap-1 items-center">フィード全文を開く<ArrowUpRight size={14} /></Link>
       </CardBody>
     </Card>
   </div>
 </div>;
}
function ProjectChanges({ data }: { data: WatchView }) {
 const { state, result } = data;
 const unhealthy = ["failed", "connection_error", "invalid_state", "waiting", "daily_limit"].includes(state.status) || !!data.stateError;
 const sourceById = new Map(result?.sources.map(s => [s.id,s]) ?? []);
 const showSources = data.stale || !result?.items.length;
 const sources = <>{data.snapshot.projects.length ? <ul className="divide-y divide-border">{data.snapshot.projects.slice(0,3).map(p => <li key={p.name} className="py-3 first:pt-0 ds-stack min-w-0">
     <div className="flex justify-between items-center gap-2 flex-wrap"><Link to={`/#project-${encodeURIComponent(p.name)}`} className="font-semibold text-base inline-flex gap-1 items-center">{p.name}<ArrowUpRight size={14} className="text-brand" /></Link><span className="font-mono text-xs text-muted-foreground">{p.date || "日付なし"}</span></div>
     <p className="text-sm line-clamp-2 [overflow-wrap:anywhere]">{watchExcerpt(p.text) || "新しい記録はありません。"}</p>
     {p.next && <p className="text-xs text-muted-foreground truncate" title={p.next}>次の節目：{watchExcerpt(p.next) || p.next}</p>}
    </li>)}</ul> : <p className="text-sm text-muted-foreground">表示するアクティブなプロジェクトがありません。</p>}</>;
 return <>
   <div className="flex items-center gap-2 flex-wrap" role="status"><Badge variant={unhealthy || data.stale ? "warning" : "muted"} dot>{unhealthy ? "要確認 · " : ""}{data.stale && !unhealthy ? "AI要約は前回分" : labels[state.status] ?? "AI更新状態を確認してください"}</Badge></div>
   {data.stale && <p className="text-xs text-warning">AI要約のあとに記録が変わっています。下の元の記録が最新です。</p>}
   {showSources && sources}
   {result && result.items.length > 0 && <details open={!data.stale}><summary className="text-sm text-brand cursor-pointer mb-2 inline-flex gap-1 items-center"><Sparkles size={14} />{data.stale ? "前回成功したAI要約" : "AIによる変化の要約"}</summary><section aria-label="AIによる変化の要約" className="ds-stack">
     {result.items.slice(0,3).map((item, i) => <article key={`${item.project}-${i}`} className="border-l-2 border-brand pl-3 ds-stack min-w-0">
       <span className="text-xs text-muted-foreground">{item.project || "フィード"} · AI要約</span>
       <p className="text-base font-medium line-clamp-2 [overflow-wrap:anywhere]">{item.change}</p>
       {item.decision && <p className="text-sm text-warning line-clamp-2">判断待ち：{item.decision}</p>}
       <details><summary className="text-xs text-muted-foreground cursor-pointer">要約と根拠</summary><div className="ds-stack text-sm"><p>{item.change}</p>{item.meaning && <p>{item.meaning}</p>}{item.decision && <p>判断待ち：{item.decision}</p>}
       <div className="flex gap-3 flex-wrap">{item.sourceIds.map(id => {
         const source = sourceById.get(id); return source ? <Link key={id} to={source.project ? `/#project-${encodeURIComponent(source.project)}` : "/feed"} className="text-xs text-brand inline-flex items-center gap-1">根拠：{source.project || "フィード"} {source.date}<ExternalLink size={11} /></Link> : null;
       })}</div></div></details>
     </article>)}
   </section></details>}
   {showSources && <details><summary className="text-xs text-muted-foreground cursor-pointer">記録の全文</summary>{data.snapshot.projects.slice(0,3).map(p => <article key={p.name}><h4 className="font-semibold">{p.name}</h4><MarkdownView content={p.text || "新しい記録はありません。"} /></article>)}</details>}
   {!showSources && <details><summary className="text-xs text-muted-foreground cursor-pointer mb-2">元の記録 · 最近の3件</summary>{sources}</details>}
   <p className="font-mono text-xs text-muted-foreground">最終確認 {stamp(state.checkedAt)}<br />AI生成 {stamp(result?.generatedAt)}</p>
   {(data.snapshot.warnings?.length || state.reason || data.stateError) ? <details><summary className="text-xs text-warning cursor-pointer">更新の詳細を確認</summary><div className="text-xs text-muted-foreground ds-stack">{data.snapshot.warnings?.map(message => <p key={message}>{message}</p>)}{(state.reason || data.stateError) && <p>{data.stateError ?? state.reason}</p>}</div></details> : null}
 </>;
}
