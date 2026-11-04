import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useLocation, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { WatchUpdateStatus } from "../components/WatchUpdateStatus";
import { DailyMergeChart } from "../components/DailyMergeChart";
import { MarkdownView } from "../components/MarkdownView";
import { Card, CardBody, CardHead } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { ErrorState, SkeletonRows } from "../components/ui/states";
import { watchHeadline } from "../lib/watchExcerpt";
import { metricNote, metricValue, watchPoll, watchStamp, WatchWarnings } from "./WatchPage";
export function WatchChangePage(){
 const {project}=useParams();const q=useQuery({queryKey:["watch"],queryFn:api.watch,...watchPoll});
 useEffect(()=>{window.scrollTo(0,0);},[project]);
 const p=q.data?.snapshot.projects.find(p=>p.name===project);const summaries=q.data?.result?.items.filter(i=>i.project===project)??[];
 return <div className="ds-section min-w-0"><Link to="/watch" className="text-brand text-sm">← 常設表示へ</Link><WatchWarnings warnings={q.data?.snapshot.warnings}/>
 {q.isPending?<SkeletonRows rows={5}/>:q.isError&&!q.data?<ErrorState error={q.error} onRetry={()=>q.refetch()}/>:!p?<p>このプロジェクトの記録はありません。</p>:<>
 {q.data&&<WatchUpdateStatus data={q.data} readError={q.isError}/>}
 <header><p className="text-sm text-muted-foreground">{p.name} · 記録 {p.date||"日付なし"}</p><h1 className="h-serif text-2xl font-semibold">{!q.data?.stale&&summaries[0]?.headline||watchHeadline(p.text)||p.name}</h1></header>
 {q.isError&&<p className="text-warning" role="status">更新を取得できません。前回の表示を残しています。</p>}
 <Card><CardHead title="最新の元の記録"/><CardBody><MarkdownView content={p.text||"新しい記録はありません。"}/>{p.next&&<p className="text-sm text-muted-foreground">次の節目：{p.next}</p>}<Link to={`/#project-${encodeURIComponent(p.name)}`} className="text-brand text-sm">プロジェクト全体を開く →</Link></CardBody></Card>
 {summaries.length>0&&<Card><CardHead title={q.data?.stale?"前回成功したAI要約":"AIによる変化の要約"} badge={q.data?.stale?<Badge variant="warning" dot>元の記録に更新あり</Badge>:undefined}/><CardBody className="ds-stack">{q.data?.stale&&<p className="text-sm text-warning">AI要約のあとに記録が変わっています。上の元の記録が最新です。</p>}{summaries.map((item,i)=><article key={i} className="ds-stack"><p className="font-semibold">{item.change}</p>{item.meaning&&<p>{item.meaning}</p>}{item.decision&&<p className="text-warning">判断待ち：{item.decision}</p>}<div className="flex flex-wrap gap-3">{item.sourceIds.map(id=>{const s=q.data?.result?.sources.find(s=>s.id===id);return s?<Link key={id} to={s.project?`/#project-${encodeURIComponent(s.project)}`:"/feed"} className="text-brand text-sm">根拠：{s.project||"フィード"} {s.date}</Link>:null;})}</div></article>)}<p className="text-xs text-muted-foreground">AI生成 {watchStamp(q.data?.result?.generatedAt)} ／ 最終確認 {watchStamp(q.data?.state.checkedAt)}</p></CardBody></Card>}
 </>}
 </div>;
}
export function WatchActivityPage(){
 const location=useLocation();
 const q=useQuery({queryKey:["watchMetrics"],queryFn:api.watchMetrics,...watchPoll});
 useEffect(()=>{if(q.isSuccess){if(location.hash)document.getElementById(location.hash.slice(1))?.scrollIntoView();else window.scrollTo(0,0);}},[q.isSuccess,location.hash]);
 const pub=metricValue(q.data?.publications),dev=metricValue(q.data?.development);
 return <div className="ds-section min-w-0"><Link to="/watch" className="text-brand text-sm">← 常設表示へ</Link><h1 className="h-serif text-2xl font-semibold">公開と開発の記録</h1>
 {q.isPending?<SkeletonRows rows={5}/>:q.isError?<ErrorState error={q.error} onRetry={()=>q.refetch()}/>:<>
 <section id="publications" className="scroll-mt-6"><Card><CardHead title="今月公開した記事" sub={`Zenn · ${pub?.month??"今月分は未取得"}`}/><CardBody className="ds-stack"><p className="font-mono text-4xl">{pub?.items.length??"—"}<span className="text-base"> 本</span></p><p className="text-sm text-muted-foreground">公開日時をJSTの暦月で集計。下書き・既存記事の更新は含みません。対象はZennです。</p>{pub?.items.length?<ul className="ds-stack">{pub.items.map(a=><li key={a.url}><a href={a.url} className="text-brand">{a.title} ↗</a><p className="text-xs text-muted-foreground">公開 {watchStamp(a.at)}</p></li>)}</ul>:<p className="text-sm">{pub?"今月の公開記事はありません。":"今月分の観測値を取得できていません。"}</p>}<p className="text-xs text-muted-foreground">{metricNote(q.data?.publications)}</p>{q.data?.publications.error&&<p className="text-warning text-sm">{q.data.publications.error}。前回値を保持しています。</p>}</CardBody></Card></section>
 <section id="development" className="scroll-mt-6"><Card><CardHead title="今週の開発" sub={`${q.data?.repo||"リポジトリ未設定"} · ${dev?.week??"今週分は未取得"}〜（JST・月曜始まり）`}/><CardBody className="ds-stack"><div className="grid grid-cols-2 gap-4"><div><p className="text-sm text-muted-foreground">マージPR</p><p className="font-mono text-4xl">{dev?.items.length??"—"}<span className="text-base"> 件</span></p></div><div><p className="text-sm text-muted-foreground">作成→マージの中央値</p><p className="font-mono text-4xl">{dev?.medianHours?.toFixed(1)??"—"}<span className="text-base"> 時間</span></p></div></div>
 {dev&&<DailyMergeChart days={dev.days} week={dev.week} updatedAt={q.data?.development.updatedAt}/>}
 <p className="text-sm text-muted-foreground">マージ日時が今週のPRを集計。却下・未マージは除外。時間はPR作成からマージまでで、作業時間や本番配備時間ではありません。</p>
 <ul className="divide-y divide-border">{dev?.items.map(p=><li key={p.url} className="py-3"><a href={p.url} className="text-brand">{p.title} ↗</a><p className="text-xs text-muted-foreground">マージ {watchStamp(p.mergedAt)}</p></li>)}</ul><p className="text-xs text-muted-foreground">{metricNote(q.data?.development)}</p>{q.data?.development.error&&<p className="text-warning text-sm">{q.data.development.error}。前回値を保持しています。</p>}
 </CardBody></Card></section>
 </>}
 </div>;
}
