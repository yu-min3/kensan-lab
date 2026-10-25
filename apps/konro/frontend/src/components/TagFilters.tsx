import {libraryRecipes,LibrarySort,RatingFilter} from '../library';
import type {RecipeMeta} from '../api';
export type Taxonomy={revision:string;assignments:Record<string,string>;groups:{id:string;category:string;label:string}[]};
export function TagFilters({recipes,q,rating,sort,selected,onChange,taxonomy}:{recipes:RecipeMeta[];q:string;rating:RatingFilter;sort:LibrarySort;selected:string[];onChange:(tags:string[])=>void;taxonomy:Taxonomy}){
 const assignments=taxonomy.assignments;
 const all=Array.from(new Set([...recipes.flatMap(r=>r.tags??[]),...selected.filter(t=>t!=='__untagged__')])).sort((a,b)=>a.localeCompare(b,'ja'));
 const toggle=(tag:string)=>onChange(tag==='__untagged__'?(selected.includes(tag)?[]:[tag]):selected.includes(tag)?selected.filter(t=>t!==tag):[...selected.filter(t=>t!=='__untagged__'),tag]);
 return <section className="taxonomy" aria-label="タグで絞り込む"><p className="recipe-summary">同じ分類は「または」、分類の間は「かつ」で絞ります。</p>
 {['料理種類','素材種類','その他'].map(category=><div className="taxonomy-category" key={category}><h2>{category}</h2>{taxonomy.groups.filter(g=>g.category===category).map(group=>{
  const tags=all.filter(t=>(assignments[t]??'other')===group.id);if(!tags.length)return null;
  const other=selected.filter(t=>(assignments[t]??'other')!==group.id&&t!=='__untagged__');
  const candidates=libraryRecipes(recipes,q,other,rating,sort,assignments);
  return <div className="taxonomy-group" key={group.id}><span className="taxonomy-label">{group.label}</span><div className="tag-row" role="group" aria-label={group.label}>{tags.map(tag=><button className={`chip ${selected.includes(tag)?'chip-on':''}`} key={tag} aria-pressed={selected.includes(tag)} onClick={()=>toggle(tag)}>{tag}<span className="tag-count tnum">{candidates.filter(r=>r.tags?.includes(tag)).length}</span></button>)}</div></div>;
 })}</div>)}
 <button className={`chip ${selected.includes('__untagged__')?'chip-on':''}`} aria-pressed={selected.includes('__untagged__')} onClick={()=>toggle('__untagged__')}>タグなし</button>
 {!!selected.length&&<div className="active-tag-filters" aria-label="選択中のタグ">{selected.map(tag=><button className="chip chip-on" key={tag} onClick={()=>toggle(tag)} aria-label={`${tag==='__untagged__'?'タグなし':tag}の条件を解除`}>{tag==='__untagged__'?'タグなし':tag} ×</button>)}</div>}
 </section>;
}
