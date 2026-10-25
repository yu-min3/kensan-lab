import type {RecipeMeta} from './api';
import {parseDuration} from './duration';
import {normalizeQuery} from './session';
export type LibrarySort='rating'|'name'|'time';
export type RatingFilter='all'|'high'|'unrated';
const names=new Intl.Collator('ja',{numeric:true,sensitivity:'base'});
const nameKey=(title:string)=>title.normalize('NFKC').replace(/^[\p{P}\p{S}\s]+/u,'');
const byName=(a:RecipeMeta,b:RecipeMeta)=>names.compare(nameKey(a.title),nameKey(b.title))||(a.file<b.file?-1:a.file>b.file?1:0);
const time=(r:RecipeMeta)=>parseDuration(r.cookTime)??Infinity;
export function matchesTags(tags:string[]|null,selected:string[],assignments:Record<string,string>){
 if(selected.includes('__untagged__'))return !tags?.length;
 const groups=new Map<string,string[]>();
 for(const tag of selected){const group=assignments[tag]??'other';groups.set(group,[...(groups.get(group)??[]),tag])}
 return [...groups.values()].every(values=>values.some(tag=>(tags??[]).includes(tag)));
}
export function libraryRecipes(recipes:RecipeMeta[],q:string,tag:string|string[],rating:RatingFilter,sort:LibrarySort,assignments:Record<string,string>={}){
 const query=normalizeQuery(q);
 return recipes.filter(r=>matchesTags(r.tags,Array.isArray(tag)?tag:tag?[tag]:[],assignments)&&(!query||normalizeQuery(r.title).includes(query)||(r.tags??[]).some(t=>normalizeQuery(t).includes(query)))&&(rating!=='high'||(r.rating??0)>=4)&&(rating!=='unrated'||!r.rating)).sort((a,b)=>{
  if(sort==='rating')return (b.rating??0)-(a.rating??0)||byName(a,b);
  if(sort==='time')return time(a)-time(b)||byName(a,b);
  return byName(a,b);
 });
}
