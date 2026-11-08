import { useCallback, useEffect, useId, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, ApiError, dailyPath, dailySkeleton, todayISO } from "../../lib/api";
import { readIntentions, writeIntentions } from "../../lib/dailyIntentions";
import { shiftDailyDate } from "../../lib/quickDiary";
import { Card, CardBody } from "../ui/card";
import { Button } from "../ui/button";
import { ErrorState, Skeleton } from "../ui/states";
import { MarkdownView } from "../MarkdownView";
import { QuickDiary } from "./QuickDiary";

type Tab="today"|"tomorrow";
const tabs: {id:Tab;label:string;name:string}[]=[{id:"today",label:"今日",name:"今日やりたいこと"},{id:"tomorrow",label:"明日",name:"明日やりたいことを書く"}];
export function TodayDiary({className}:{className?:string}={}) {
 const [today,setToday]=useState(todayISO);const [tab,setTab]=useState<Tab>("today");
 const [editing,setEditing]=useState(false);const [busy,setBusy]=useState(false);const [frozen,setFrozen]=useState(today);
 const id=useId();
 const [diaryEditing,setDiaryEditing]=useState(false);const [diaryFrozen,setDiaryFrozen]=useState(today);
 const diaryDate=diaryEditing?diaryFrozen:today;
 const onDiaryEditing=useCallback((value:boolean)=>{if(value)setDiaryFrozen(diaryDate);setDiaryEditing(value);},[diaryDate]);
 const onDiaryBusy=useCallback((_value:boolean)=>{},[]);
 useEffect(()=>{const update=()=>setToday(todayISO());const timer=window.setInterval(update,30_000);window.addEventListener("focus",update);return()=>{window.clearInterval(timer);window.removeEventListener("focus",update);};},[]);
 const requested=tab==="tomorrow"?shiftDailyDate(today,1):today;
 const date=editing?frozen:requested;
 const onEditing=useCallback((value:boolean)=>{if(value)setFrozen(date);setEditing(value);},[date]);
 const label=date===today?"今日やりたいこと":date===shiftDailyDate(today,1)?"明日やりたいこと":`${date} やりたいこと`;
 function select(next:Tab){if(busy||(next===tab&&date===requested))return;setEditing(false);setTab(next);}
 return <Card className={className}>
  <header className="ds-card flex flex-col sm:flex-row justify-between sm:items-center gap-3 border-b border-border">
   <div><h3 className="h-serif text-base font-semibold">{label}</h3><p className="font-mono text-xs text-muted-foreground">{date} · JST</p></div>
   <div className="flex items-center gap-3 flex-wrap"><div role="tablist" aria-label="記入する内容" className="flex gap-1 border-b border-border">
    {tabs.map((item,i)=><button key={item.id} id={`${id}-${item.id}`} role="tab" type="button" aria-label={item.name} aria-selected={tab===item.id} aria-controls={`${id}-panel`} tabIndex={tab===item.id?0:-1} disabled={busy}
     onClick={()=>select(item.id)} onKeyDown={e=>{let next:number;if(e.key==="ArrowRight")next=(i+1)%tabs.length;else if(e.key==="ArrowLeft")next=(i+tabs.length-1)%tabs.length;else if(e.key==="Home")next=0;else if(e.key==="End")next=tabs.length-1;else return;e.preventDefault();select(tabs[next].id);(e.currentTarget.parentElement!.querySelectorAll('button[role="tab"]')[next] as HTMLButtonElement).focus();}}
     className={`min-h-11 px-3 text-sm whitespace-nowrap border-b-2 focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50 ${tab===item.id?"border-brand text-brand font-semibold":"border-transparent text-muted-foreground"}`}>{item.label}</button>)}
   </div><Link to="/daily" className="text-sm text-brand underline underline-offset-2">日記を開く</Link></div>
  </header>
  <CardBody className="grid grid-cols-1 md:grid-cols-2 gap-4"><div id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-${tab}`} className="ds-stack">
   {editing&&date!==requested&&<p className="text-xs text-warning" role="status">日付が変わりました。入力中の {date} に保存します。</p>}
   <TodayIntentions key={date} date={date} label={label} onEditing={onEditing} onBusy={setBusy}/>
  </div><section aria-label="一行日記を残す" className="ds-stack border-t md:border-t-0 md:border-l border-border pt-4 md:pt-0 md:pl-4">
   {diaryEditing&&diaryDate!==today&&<p className="text-xs text-warning" role="status">日付が変わりました。一行日記は入力を始めた {diaryDate} に保存します。</p>}
   <QuickDiary key={`diary-${diaryDate}`} date={diaryDate} onEditing={onDiaryEditing} onBusy={onDiaryBusy}/>
  </section></CardBody>
 </Card>;
}
function TodayIntentions({date,label,onEditing,onBusy}:{date:string;label:string;onEditing:(v:boolean)=>void;onBusy:(v:boolean)=>void}) {
 const path=dailyPath(date);const client=useQueryClient();
 const file=useQuery({queryKey:["file",path],queryFn:()=>api.file(path),retry:(count,error)=>!(error instanceof ApiError&&error.status===404)&&count<2,refetchInterval:60_000,refetchIntervalInBackground:true});
 const missing=file.error instanceof ApiError&&file.error.status===404;const storageKey=`kensan:intentions:${date}`;
 const [recoverable,setRecoverable]=useState(()=>{try{return !!sessionStorage.getItem(storageKey);}catch{return false;}});
 const [editing,setEditing]=useState(false);const [draft,setDraft]=useState("");const [base,setBase]=useState<{content:string;mtime:string}|null>(null);
 const save=useMutation({mutationFn:async()=>{if(!base)throw new Error("読み込み後に編集してください。");const content=writeIntentions(base.content,draft);if(base.mtime)await api.putFile(path,content,base.mtime);else await api.createFile(path,content);},
  onSuccess:async()=>{await Promise.all([client.invalidateQueries({queryKey:["file",path]}),client.invalidateQueries({queryKey:["dailyList"]})]);try{sessionStorage.removeItem(storageKey);}catch{/* Optional cache. */}setRecoverable(false);setEditing(false);onEditing(false);},onSettled:()=>onBusy(false)});
 function edit(){const content=file.data?.content??dailySkeleton(date);let text=readIntentions(content);let mtime=file.data?.doc.mtime??"";
  try{const saved=JSON.parse(sessionStorage.getItem(storageKey)??"null");if(saved&&typeof saved.text==="string"&&typeof saved.mtime==="string"){text=saved.text;mtime=saved.mtime;}}catch{/* Ignore invalid cache. */}
  setBase({content,mtime});setDraft(text);save.reset();setEditing(true);onEditing(true);
 }
 const intentions=readIntentions(file.data?.content??"");
 return editing?<form className="ds-stack" onSubmit={e=>{e.preventDefault();if(!save.isPending){onBusy(true);save.mutate();}}}>
  <label htmlFor={`intentions-${date}`} className="text-sm">{label}（ダッシュボードに表示）</label>
  <textarea id={`intentions-${date}`} value={draft} rows={4} autoFocus disabled={save.isPending} placeholder="例：午前は記事を書く。午後は散歩に出る。" className="w-full rounded-md border border-border bg-card p-3 text-base focus-visible:ring-2 focus-visible:ring-ring"
   onChange={e=>{const text=e.target.value;setDraft(text);try{sessionStorage.setItem(storageKey,JSON.stringify({text,mtime:base?.mtime??""}));setRecoverable(true);}catch{/* Optional cache. */}}}/>
  <div className="flex flex-wrap gap-2"><Button type="submit" variant="primary" className="min-h-11" loading={save.isPending}>保存する</Button><Button type="button" variant="ghost" className="min-h-11" disabled={save.isPending} onClick={()=>{try{sessionStorage.removeItem(storageKey);}catch{/* Optional cache. */}setRecoverable(false);setEditing(false);onEditing(false);}}>キャンセル</Button></div>
  {save.isError&&<div className="ds-stack"><p role="alert" className="text-sm text-destructive">保存できませんでした。入力は残っています。最新の内容を取り込んでから、保存をやり直せます。</p><Button type="button" variant="outline" className="min-h-11" onClick={async()=>{const latest=await file.refetch();if(latest.isError||!latest.data)return;const mtime=latest.data.doc.mtime;setBase({content:latest.data.content,mtime});try{sessionStorage.setItem(storageKey,JSON.stringify({text:draft,mtime}));}catch{/* Optional cache. */}save.reset();}}>入力を残して最新の内容を取り込む</Button></div>}
 </form>:file.isPending?<Skeleton className="h-20 w-full"/>:file.isError&&!missing?<ErrorState error={file.error} onRetry={()=>file.refetch()}/>:<>
  {intentions?<div aria-label={`${label}の本文`}><MarkdownView content={intentions}/></div>:<p className="text-sm text-muted-foreground">{label}は、まだ書いていません。</p>}
  {recoverable&&<p className="text-xs text-muted-foreground">未保存の入力があります。編集を開くと続きから書けます。</p>}
  <div><Button variant="outline" className="min-h-11" onClick={edit}>{intentions?`${label}を編集`:`${label}を書く`}</Button></div>
 </>;
}
