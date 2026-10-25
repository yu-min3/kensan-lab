import { useCallback, useEffect, useRef, useState } from 'react';

export type Route = {page:'list'} | {page:'cooking'} | {page:'recipe';file:string} | {page:'search'} | {page:'creators'} | {page:'import';url:string;draft?:string;fromSearch:boolean};
export type LeaveGuard=()=>boolean|Promise<boolean>;
const key='konroRouteIndex', previousKey='konroPreviousRoute';
function readRoute():Route {
 const [path,query='']=location.hash.slice(1).split('?');
 if(path==='new')return {page:'recipe',file:''};
 if(path.startsWith('recipe/')){try{return {page:'recipe',file:decodeURIComponent(path.slice(7))}}catch{return {page:'list'}}}
 if(path==='search'||path==='creators'||path==='cooking')return {page:path};
 if(path==='import'){const p=new URLSearchParams(query);return {page:'import',url:p.get('url')??'',draft:/^[a-f0-9]{32}$/.test(p.get('draft')??'')?p.get('draft')!:undefined,fromSearch:p.get('from')==='search'}};
 return {page:'list'};
}
const hashFor=(r:Route):string=>{
 switch(r.page){
  case 'list':return '#list';
  case 'recipe':return r.file?`#recipe/${encodeURIComponent(r.file)}`:'#new';
  case 'search':case 'creators':case 'cooking':return '#'+r.page;
  case 'import':{const p=new URLSearchParams();if(r.url)p.set('url',r.url);if(r.draft)p.set('draft',r.draft);if(r.fromSearch)p.set('from','search');return '#import'+(p.size?'?'+p:'')}
 }
};

/** One history for recipe, search, creator and import screens. */
export function useRecipeRoute(){
 const [route,setRoute]=useState(readRoute);
 const guard=useRef<LeaveGuard|null>(null);
 const current=useRef({index:Number(history.state?.[key]??0),route});
 const restoring=useRef(false),checking=useRef(false),bypass=useRef(false);
 const requested=useRef(current.current);
 const onGuard=useCallback((value:LeaveGuard|null)=>{guard.current=value},[]);
 useEffect(()=>{
  history.replaceState({...history.state,[key]:current.current.index},'',location.href);
  const pop=()=>{
   if(restoring.current){restoring.current=false;return}
   requested.current={index:Number(history.state?.[key]??0),route:readRoute()};
   if(bypass.current){bypass.current=false;current.current=requested.current;setRoute(current.current.route);return}
   if(checking.current)return;
   checking.current=true;
   Promise.resolve().then(()=>guard.current?.()??true).catch(()=>false).then(allow=>{
    checking.current=false;const next=requested.current;
    if(allow){current.current=next;setRoute(next.route);return}
    const delta=current.current.index-next.index;
    if(delta){restoring.current=true;history.go(delta)}
    else history.replaceState({...history.state,[key]:current.current.index},'',`${location.pathname}${location.search}${hashFor(current.current.route)}`);
   });
  };
  addEventListener('popstate',pop);return()=>removeEventListener('popstate',pop);
 },[]);
 const navigate=useCallback((next:Route,replace=false)=>{
  const index=current.current.index+(replace?0:1);
  history[replace?'replaceState':'pushState']({...history.state,[key]:index,...(!replace?{[previousKey]:hashFor(current.current.route)}:{})},'',`${location.pathname}${location.search}${hashFor(next)}`);
  current.current={index,route:next};setRoute(next);
 },[]);
 const back=useCallback((fallback:Route,alreadySaved=false)=>{
  if(current.current.index>0&&history.state?.[previousKey]===hashFor(fallback)){bypass.current=alreadySaved;history.back()}else navigate(fallback);
 },[navigate]);
 const navigateChecked=useCallback(async(next:Route)=>{if(await(guard.current?.()??true))navigate(next)},[navigate]);
 return {route,navigate,navigateChecked,back,onGuard};
}
