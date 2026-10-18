import {useEffect,useRef,useState} from 'react';
import type {SourceInfo} from '../api';
// Metadata is progressively loaded for visible cards, with two requests at a time.
const cache=new Map<string,{task:Promise<SourceInfo|null>;expires:number}>();
const queue:Array<()=>void>=[];let running=0;
function preview(url:string){
 const old=cache.get(url);if(old&&old.expires>Date.now())return old.task;
 const task=new Promise<SourceInfo|null>(resolve=>{
  const run=()=>{running++;fetch('/api/v1/discovery/preview?'+new URLSearchParams({url}),{signal:AbortSignal.timeout(7000)}).then(r=>r.ok?r.json():null).then(resolve,()=>resolve(null)).finally(()=>{running--;queue.shift()?.();});};
  if(running<2)run();else queue.push(run);
 });
 if(cache.size>=128)cache.delete(cache.keys().next().value!);cache.set(url,{task,expires:Date.now()+600000});return task;
}
export function useSourcePreview(url:string,initial?:SourceInfo){
 const ref=useRef<HTMLLIElement>(null);const [info,setInfo]=useState<SourceInfo|undefined>(initial);
 useEffect(()=>{
  let live=true;const el=ref.current;if(!el||new URL(url).hostname.endsWith('youtube.com'))return;
  const observer=new IntersectionObserver(entries=>{if(!entries.some(e=>e.isIntersecting))return;observer.disconnect();void preview(url).then(value=>{if(live&&value)setInfo({...initial,...value,creator:value.creator?.id?value.creator:initial?.creator??value.creator});});},{rootMargin:'160px'});
  observer.observe(el);return()=>{live=false;observer.disconnect();};
 },[url]);
 return {ref,info};
}
