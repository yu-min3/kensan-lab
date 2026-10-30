import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { ArrowUpRight, BookOpen, GitPullRequest, Star, AlertTriangle } from "lucide-react";
import { api, todayISO, type WatchMetricSection } from "../lib/api";
import { watchHeadline } from "../lib/watchExcerpt";
import { TodayDiary } from "../components/today/TodayDiary";
import { Badge } from "../components/ui/badge";
import { Card, CardBody, CardHead } from "../components/ui/card";
import { ErrorState, SkeletonRows } from "../components/ui/states";
import { splitFeedItems, splitFeedSections } from "../components/FeedSections";

export const watchPoll = { refetchInterval: 60_000, refetchIntervalInBackground: true } as const;
export function watchStamp(value?: string) {
 if (!value || value.startsWith("0001-")) return "未取得";
 const date = new Date(value); if (Number.isNaN(date.getTime())) return "日時不明";
 return new Intl.DateTimeFormat("ja-JP", {timeZone:"Asia/Tokyo",month:"numeric",day:"numeric",hour:"2-digit",minute:"2-digit"}).format(date);
}
export function metricValue<T>(section?: WatchMetricSection<T>) { return section?.currentPeriod ? section.data : null; }
export function metricNote<T>(section?: WatchMetricSection<T>) { return !section?.data ? "未取得" : !section.currentPeriod ? "今期分は未取得" : section.stale ? `前回値 · ${watchStamp(section.updatedAt)}` : `${watchStamp(section.updatedAt)} 取得`; }
export function WatchPage() {
 const watch=useQuery({queryKey:["watch"],queryFn:api.watch,...watchPoll});
 const metrics=useQuery({queryKey:["watchMetrics"],queryFn:api.watchMetrics,...watchPoll});
 const stars=useQuery({queryKey:["projectMetrics","kensan-lab"],queryFn:()=>api.projectMetrics("kensan-lab"),...watchPoll});
 const feed=useQuery({queryKey:["feeds","latest"],queryFn:api.latestFeed,...watchPoll});
 const ack=useQuery({queryKey:["feeds","acknowledgements"],queryFn:api.feedAcknowledgements,...watchPoll});
 const star=stars.data?.metrics.find(m=>m.id==="github-stars");
 const published=metricValue(metrics.data?.publications),dev=metricValue(metrics.data?.development);
 const sections=splitFeedSections(feed.data?.feed?.content??"");
 const acknowledged=new Map(ack.data?.items.map(i=>[i.key,i.version])??[]);
 const inbox=sections.filter(s=>["要対応","確認","要確認","確認待ち"].includes(s.title)).flatMap(s=>splitFeedItems(s.content));
 const unconfirmed=inbox.filter(i=>acknowledged.get(i.key)!==i.version);
 const news=sections.filter(s=>["今日のニュース","今日の5本","リリース・定点観測"].includes(s.title)).flatMap(s=>splitFeedItems(s.content));
 const items=unconfirmed.length?unconfirmed:news;
 const data=watch.data;
 const changes=data?.snapshot.projects.slice(0,3).map(p=>{
  const ai=!data.stale?data.result?.items.find(i=>i.project===p.name):undefined;
  return {project:p.name,date:p.date,title:ai?.headline||watchHeadline(ai?.change||p.text)||`${p.name}の記録を確認`,ai:!!ai};
 })??[];
 return <div className="ds-section min-w-0">
  <header className="flex items-baseline justify-between gap-3 flex-wrap"><h1 className="h-serif text-xl font-semibold">今日と、最近の変化</h1><span className="font-mono text-xs text-muted-foreground">{todayISO()} · JST</span></header>
  <section aria-label="成果と開発の指標" className="grid grid-cols-1 sm:grid-cols-3 gap-3">
   <Link to="/watch/activity#publications" className="min-w-0 rounded-lg focus-visible:ring-2 focus-visible:ring-ring"><Card className="h-full"><CardBody className="ds-stack"><div className="flex items-center justify-between text-muted-foreground"><BookOpen size={20}/><ArrowUpRight size={16}/></div><p className="text-sm">今月公開した記事</p><p className="font-mono tnum text-4xl">{metrics.isError?"—":published?.items.length??"—"}<span className="text-sm text-muted-foreground"> 本</span></p><p className="text-xs text-muted-foreground">Zenn · {published?.month??todayISO().slice(0,7)}</p><p className={`text-xs ${metrics.isError||metrics.data?.publications.stale?"text-warning":"text-muted-foreground"}`}>{metrics.isError?"取得失敗":metricNote(metrics.data?.publications)}</p></CardBody></Card></Link>
   <a href="https://github.com/yu-min3/kensan-lab/stargazers" className="min-w-0 rounded-lg focus-visible:ring-2 focus-visible:ring-ring"><Card className="h-full"><CardBody className="ds-stack"><div className="flex items-center justify-between text-brand"><Star size={20}/><ArrowUpRight size={16}/></div><p className="text-sm">kensan-lab Stars</p><p className="font-mono tnum text-4xl">{stars.isError?"—":star?.current??"—"}</p><p className="text-xs text-muted-foreground">{star?.delta.days30!=null?`30日前比 ${star.delta.days30>=0?"+":""}${star.delta.days30}`:"GitHubの保存済み観測値"}</p><p className={`text-xs ${stars.isError||star?.stale?"text-warning":"text-muted-foreground"}`}>{stars.isError?"取得失敗":`${star?.stale?"前回値 · ":""}${watchStamp(star?.updatedAt)} 取得`}</p></CardBody></Card></a>
   <Link to="/watch/activity#development" className="min-w-0 rounded-lg focus-visible:ring-2 focus-visible:ring-ring"><Card className="h-full"><CardBody className="ds-stack"><div className="flex items-center justify-between text-muted-foreground"><GitPullRequest size={20}/><ArrowUpRight size={16}/></div><p className="text-sm">今週のマージPR</p><p className="font-mono tnum text-4xl">{metrics.isError?"—":dev?.items.length??"—"}<span className="text-sm text-muted-foreground"> 件</span></p><p className="text-xs text-muted-foreground">kensan-lab · {dev?.medianHours!=null?`マージ中央値 ${dev.medianHours<1?`${Math.round(dev.medianHours*60)}分`:`${dev.medianHours.toFixed(1)}時間`}`:"マージ時間は未集計"}</p><p className={`text-xs ${metrics.isError||metrics.data?.development.stale?"text-warning":"text-muted-foreground"}`}>{metrics.isError?"取得失敗":metricNote(metrics.data?.development)}</p></CardBody></Card></Link>
  </section>
  <TodayDiary/>
  <div className="grid grid-cols-1 md:grid-cols-2 gap-4 items-start min-w-0">
   <Card className="min-w-0"><CardHead title="プロジェクトの変化" badge={data?.stale?<Badge variant="warning" dot>記録更新あり</Badge>:undefined}/><CardBody className="ds-stack">
    {watch.isPending?<SkeletonRows rows={3}/>:watch.isError&&!data?<ErrorState error={watch.error} onRetry={()=>watch.refetch()}/>:<>
     {data?.state.status==="not_configured"&&<p className="text-xs text-muted-foreground">AI更新は設定前</p>}
     {(watch.isError||data?.stateError||data?.state.status==="failed")&&<p className="text-xs text-warning" role="status">更新を確認できません。保存済みの記録を表示しています。</p>}
     <ul className="divide-y divide-border">{changes.map(c=><li key={c.project} className="py-4 first:pt-0 last:pb-0"><Link to={`/watch/changes/${encodeURIComponent(c.project)}`} className="group ds-stack block rounded-sm focus-visible:ring-2 focus-visible:ring-ring"><span className="flex gap-2 justify-between items-center text-xs text-muted-foreground"><span>{c.project}</span><span className="font-mono">{c.date||"日付なし"}</span></span><span className="flex gap-2 justify-between items-start"><span className="font-semibold text-lg leading-snug line-clamp-2 [overflow-wrap:anywhere] group-hover:text-brand">{c.title}</span><ArrowUpRight size={18} className="text-brand shrink-0"/></span><span className="text-xs text-muted-foreground">{c.ai?"AI要約":"元の記録"}を読む</span></Link></li>)}</ul>
     {!changes.length&&<p className="text-sm text-muted-foreground">表示するプロジェクトの記録がありません。</p>}
     <p className="text-xs text-muted-foreground">最終確認 {watchStamp(data?.state.checkedAt)}</p>
    </>}
   </CardBody></Card>
   <Card className="min-w-0"><CardHead title={unconfirmed.length?"まだ確認していないもの":"新着フィード"} badge={<Badge variant={unconfirmed.length?"warning":"muted"}>{ack.isError||feed.isError||!ack.data?"件数未確認":`${unconfirmed.length||news.length}件`}</Badge>}/><CardBody className="ds-stack">
    {feed.isPending?<SkeletonRows rows={3}/>:feed.isError&&!feed.data?<ErrorState error={feed.error} onRetry={()=>feed.refetch()}/>:<>
     {(feed.isError||feed.data?.stale||feed.data?.stateError)&&<p className="text-xs text-warning inline-flex gap-1" role="status"><AlertTriangle size={14}/>前回成功分を表示しています。</p>}
     {ack.isError&&<p className="text-xs text-warning">確認済み状態を取得できません。未確認の件数は確定していません。</p>}
     <ol className="divide-y divide-border">{items.slice(0,3).map((item,i)=><li key={item.key} className="py-4 first:pt-0 last:pb-0"><Link to="/feed" className="flex gap-3 items-start rounded-sm focus-visible:ring-2 focus-visible:ring-ring"><span className="font-mono tnum text-xl text-muted-foreground" aria-hidden="true">{String(i+1).padStart(2,"0")}</span><span className="font-semibold text-base line-clamp-2 [overflow-wrap:anywhere]">{watchHeadline(item.title)||item.title}</span><ArrowUpRight size={16} className="text-brand shrink-0 ml-auto"/></Link></li>)}</ol>
     {!items.length&&<p className="text-sm text-muted-foreground">表示する新着項目はありません。</p>}
     <p className="text-xs text-muted-foreground">生成 {watchStamp(feed.data?.feed?.generatedAt)}</p>
    </>}
    <Link to="/feed" className="text-sm text-brand">フィード全文を開く →</Link>
   </CardBody></Card>
  </div>
 </div>;
}
