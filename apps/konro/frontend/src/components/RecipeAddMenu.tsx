import {useEffect,useRef} from 'react';
import {Plus,ChevronDown,Link,FilePenLine,Archive} from 'lucide-react';
export function RecipeAddMenu({onImport,onNew,onZip,importing}:{onImport:()=>void;onNew:()=>void;onZip:(file:File)=>void;importing:boolean}){
 const menu=useRef<HTMLDetailsElement>(null),input=useRef<HTMLInputElement>(null);
 const close=()=>{if(menu.current)menu.current.open=false};
 useEffect(()=>{
  const outside=(e:PointerEvent)=>{if(!menu.current?.contains(e.target as Node))close()};
  const escape=(e:KeyboardEvent)=>{if(e.key==='Escape'&&menu.current?.open){close();menu.current.querySelector('summary')?.focus()}};
  document.addEventListener('pointerdown',outside);document.addEventListener('keydown',escape);
  return()=>{document.removeEventListener('pointerdown',outside);document.removeEventListener('keydown',escape)};
 },[]);
 const choose=(fn:()=>void)=>{close();fn()};
 return <details className="recipe-add-menu" ref={menu}>
  <summary className="recipe-action"><Plus aria-hidden="true"/>追加<ChevronDown aria-hidden="true"/></summary>
  <div className="recipe-add-options" aria-label="レシピの追加方法">
   <button className="recipe-action recipe-action-ghost" onClick={()=>choose(onImport)}><Link aria-hidden="true"/>URL・本文から取り込む</button>
   <button className="recipe-action recipe-action-ghost" onClick={()=>choose(onNew)}><FilePenLine aria-hidden="true"/>手入力で作成</button>
   <button className="recipe-action recipe-action-ghost" disabled={importing} onClick={()=>choose(()=>input.current?.click())}><Archive aria-hidden="true"/>{importing?'ZIP取込中…':'ZIP取込'}</button>
   <input ref={input} aria-label="レシピZIPファイル" type="file" accept=".zip,application/zip" hidden onChange={e=>{const file=e.target.files?.[0];if(file)onZip(file);e.target.value=''}}/>
  </div>
 </details>;
}
