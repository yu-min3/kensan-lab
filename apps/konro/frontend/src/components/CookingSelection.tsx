import {useEffect,useState} from 'react';
import {Flame,ChevronDown,X} from 'lucide-react';
import type {RecipeMeta} from '../api';
export function CookingSelection({selected,recipes,starting,cooking=false,onRemove,onClear,onStart}:{selected:string[];recipes:RecipeMeta[];starting:boolean;cooking?:boolean;onRemove:(file:string)=>void;onClear:()=>void;onStart:()=>void}){
 const [expanded,setExpanded]=useState(()=>matchMedia('(min-width: 1000px)').matches);
 useEffect(()=>{const media=matchMedia('(min-width: 1000px)');const update=()=>setExpanded(media.matches);media.addEventListener('change',update);return()=>media.removeEventListener('change',update)},[]);
 const title=(file:string)=>recipes.find(r=>r.file===file)?.title??file;
 if(!selected.length)return <section className="library-selection library-selection-empty" aria-label="今回調理するレシピ"><div className="cooking-selection"><h2><Flame aria-hidden="true"/>今回の調理</h2><p className="recipe-summary">作りたい料理を選ぶと、ここで確認してコンロを開始できます。</p></div></section>;
 return <section className="library-selection" data-selected="true" aria-label="今回調理するレシピ"><div className="cooking-selection">
  <details className="selection-details" open={expanded} onToggle={e=>setExpanded(e.currentTarget.open)}>
   <summary><span className="selection-heading"><Flame aria-hidden="true"/><span role="heading" aria-level={2} aria-live="polite">今回調理するレシピ（{selected.length}品）</span><ChevronDown aria-hidden="true"/></span>{!expanded&&<span className="selection-summary-names">{selected.slice(0,2).map(title).join('、')}{selected.length>2&&` ほか${selected.length-2}品`}</span>}</summary>
   <div className="selection-tools"><button className="chip" disabled={starting} onClick={onClear}>すべて外す</button></div>
   <ol className="cooking-selection-list">{selected.map((file,i)=><li key={file}><span className="selection-order tnum">{i+1}</span><span>{title(file)}</span><button className="chip" disabled={starting} aria-label={`${title(file)}を今回の調理から外す`} onClick={()=>onRemove(file)}><X aria-hidden="true"/>外す</button></li>)}</ol>
  </details>
  <button className="start-button" disabled={starting} onClick={onStart}><Flame aria-hidden="true"/>{cooking?'調理に戻る':starting?'準備中…':`${selected.length}品でコンロを開始`}</button>
 </div></section>;
}
