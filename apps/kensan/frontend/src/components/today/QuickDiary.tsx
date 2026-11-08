import { useEffect, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, dailyPath, dailySkeleton } from "../../lib/api";
import { prepareQuickLine, quickLineWasSaved } from "../../lib/quickDiary";
import { Button } from "../ui/button";

type Draft={text:string;entry?:string;at?:string};
type Base={content:string;mtime:string};
type Prepared=ReturnType<typeof prepareQuickLine>;
export function QuickDiary({date,onEditing,onBusy}:{date:string;onEditing:(editing:boolean)=>void;onBusy:(busy:boolean)=>void}) {
 const key=`kensan:quick-diary:${date}`;
 const [draft,setDraft]=useState<Draft>(()=>{try{const value=JSON.parse(sessionStorage.getItem(key)??"null");return value&&typeof value.text==="string"&&(!value.entry||typeof value.entry==="string")?value:{text:""};}catch{return {text:""};}});
 useEffect(()=>{if(draft.text||draft.entry)onEditing(true);},[onEditing]);
 const [last,setLast]=useState<{entry:string;text:string;at:string}|null>(null);const [recovering,setRecovering]=useState(false);
 const attempt=useRef<{base:Base;at:string;prepared?:Prepared}|null>(null);
 const client=useQueryClient();const guard=useRef(false);
 function change(next:Draft){setDraft(next);try{if(next.text||next.entry)sessionStorage.setItem(key,JSON.stringify(next));else sessionStorage.removeItem(key);}catch{/* Optional recovery cache. */}onEditing(!!next.text||!!next.entry);}
 async function finish(saved:Prepared){setLast({entry:saved.entry,text:saved.text,at:saved.at});change({text:""});attempt.current=null;await Promise.all([client.invalidateQueries({queryKey:["file",dailyPath(date)]}),client.invalidateQueries({queryKey:["dailyList"]}),client.invalidateQueries({queryKey:["today"]})]);}
 const save=useMutation({
  mutationFn:async()=>{
   if(!attempt.current){let base:Base;try{const f=await api.file(dailyPath(date));base={content:f.content,mtime:f.doc.mtime};}catch(e){if(!(e instanceof ApiError)||e.status!==404)throw e;base={content:dailySkeleton(date),mtime:""};}
    const at=draft.at??new Intl.DateTimeFormat("ja-JP",{timeZone:"Asia/Tokyo",hour:"2-digit",minute:"2-digit",hourCycle:"h23"}).format(new Date());attempt.current={base,at};}
   const {base,at}=attempt.current;const prepared=prepareQuickLine(base.content,draft.text,at,draft.entry);attempt.current.prepared=prepared;
   try{if(base.mtime)await api.putFile(dailyPath(date),prepared.content,base.mtime);else await api.createFile(dailyPath(date),prepared.content);}
   catch(e){const latest=await api.file(dailyPath(date)).catch(()=>null);if(!latest||!quickLineWasSaved(base.content,latest.content,prepared.entry,draft.entry))throw e;}
   return prepared;
  },onSuccess:finish,onSettled:()=>{guard.current=false;onBusy(false);},
 });
 async function recover(){
  if(guard.current)return;guard.current=true;setRecovering(true);onBusy(true);
  try{const latest=await api.file(dailyPath(date)).catch(e=>{if(e instanceof ApiError&&e.status===404)return null;throw e;});const previous=attempt.current;
   if(latest&&previous?.prepared&&quickLineWasSaved(previous.base.content,latest.content,previous.prepared.entry,draft.entry))await finish(previous.prepared);
   else attempt.current=null;
   save.reset();
  }catch{/* Keep the existing error and input while offline. */}finally{guard.current=false;setRecovering(false);onBusy(false);}
 }
 const busy=save.isPending||recovering;
 return <form className="ds-stack" onSubmit={event=>{event.preventDefault();if(!draft.text.trim()||guard.current)return;guard.current=true;onBusy(true);save.mutate();}}>
  <label htmlFor={`quick-diary-${date}`} className="text-sm font-medium">{draft.entry?"一行日記を編集":"いまの気持ちを一行で"}</label>
  <div className="flex items-center gap-2"><input type="text" id={`quick-diary-${date}`} aria-label="一行日記" value={draft.text} maxLength={500} disabled={busy}
   onChange={e=>{save.reset();change({...draft,text:e.target.value});}}
   placeholder="気づいたことをひと言" className="ds-control min-w-0 w-full rounded-md border border-border bg-card px-3 text-base focus-visible:ring-2 focus-visible:ring-ring"/>
  <Button type="submit" variant="outline" className="shrink-0" loading={save.isPending} disabled={recovering||!draft.text.trim()}>{draft.entry?"変更を保存":"日記に残す"}</Button></div>
  {(draft.entry||last)&&<div className="flex flex-wrap items-center gap-3">
   {draft.entry&&<Button type="button" variant="ghost" className="min-h-11" disabled={busy} onClick={()=>{attempt.current=null;change({text:""});save.reset();}}>編集をやめる</Button>}
   {last&&!draft.text&&!draft.entry&&<><span role="status" className="text-xs text-success">日記に残しました</span><Button type="button" variant="ghost" className="min-h-11" onClick={()=>{attempt.current=null;change({text:last.text,entry:last.entry,at:last.at});save.reset();}}>今の一行を編集</Button></>}
  </div>}
  {save.isError&&<div className="ds-stack"><p role="alert" className="text-sm text-destructive">保存できませんでした。入力は残っています。日記で保存済みか確認してから、再読み込みできます。</p><Button type="button" variant="outline" className="min-h-11" loading={recovering} onClick={recover}>入力を残して再読み込み</Button></div>}
  <p className="text-xs text-muted-foreground">{date} の日記へ保存します。本文は「日記を開く」から読み返せます。</p>
 </form>;
}
