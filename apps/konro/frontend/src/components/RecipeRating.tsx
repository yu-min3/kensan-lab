import {useEffect,useRef,useState} from 'react';
import {fetchRecipe,Recipe,saveRecipe} from '../api';

export function RecipeRating({recipe,disabled,onSaved,onBusy}:{recipe:Recipe;disabled:boolean;onSaved:(r:Recipe)=>void;onBusy:(value:boolean)=>void}){
 const [value,setValue]=useState(recipe.rating??0),[undo,setUndo]=useState<number|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false),[message,setMessage]=useState('');
 const gate=useRef(false),live=useRef(true);
 useEffect(()=>{live.current=true;return()=>{live.current=false}},[]);
 useEffect(()=>{setValue(recipe.rating??0)},[recipe.version]);
 const save=async(rating:number,rollback=false)=>{
  if(gate.current)return;gate.current=true;setBusy(true);onBusy(true);setValue(rating);setError('');setMessage('');
  try{
   const r=await saveRecipe(recipe.file,recipe.version,{title:recipe.title,servings:recipe.servings??'',tags:recipe.tags??[],prepTime:recipe.prepTime??'',cookTime:recipe.cookTime??'',source:recipe.source??'',rating,ingredients:recipe.ingredients??[],steps:recipe.steps??[],notes:recipe.notes??''},'');
   if(live.current){setUndo(rollback?null:recipe.rating??0);setMessage(rollback?'評価を元に戻しました':'評価を保存しました');onSaved(r)}
  }catch(e){if(live.current)setError(e instanceof Error?e.message:String(e))}finally{gate.current=false;if(live.current){setBusy(false);onBusy(false)}}
 };
 return <section className="recipe-rating-editor" aria-label="自分の評価を変更"><label>自分の評価<select aria-label="自分の評価" value={value} disabled={disabled||busy} onChange={e=>void save(Number(e.target.value))}>{[0,1,2,3,4,5].map(n=><option value={n} key={n}>{n?`${n} / 5`:'未評価'}</option>)}</select></label>
  {busy&&<p role="status">評価を保存しています…</p>}{message&&<p role="status">{message}</p>}
  {error&&<div role="alert"><p>{error} 選んだ評価は残しています。</p><button className="chip" disabled={disabled||busy} onClick={()=>void save(value)}>再試行</button><button className="chip" disabled={disabled||busy} onClick={()=>{void fetchRecipe(recipe.file).then(r=>{onSaved(r);setError('')}).catch(e=>setError(e.message))}}>最新版を確認</button></div>}
  {undo!==null&&<button className="chip" disabled={disabled||busy} onClick={()=>void save(undo,true)}>評価を元に戻す</button>}
 </section>;
}
