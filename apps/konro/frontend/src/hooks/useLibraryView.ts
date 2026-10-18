import {useEffect,useLayoutEffect,useRef,useState} from 'react';
import type {LibrarySort,RatingFilter} from '../library';
const viewKey='konro.library-view',sortKey='konro.library-sort';
type View={q:string;tag:string;rating:RatingFilter;selected:string[];scroll:number};
function readView():View{
 const empty:View={q:'',tag:'',rating:'all',selected:[],scroll:0};
 try{const value=JSON.parse(sessionStorage.getItem(viewKey)??'null');if(value&&typeof value.q==='string'&&typeof value.tag==='string'&&['all','high','unrated'].includes(value.rating)&&Array.isArray(value.selected)&&value.selected.length<=1000&&value.selected.every((f:unknown)=>typeof f==='string'&&f.length<2048)&&Number.isFinite(value.scroll)&&value.scroll>=0)return {...value,selected:Array.from(new Set<string>(value.selected))};}catch{/* Optional browser cache. */}
 return empty;
}
function readSort():LibrarySort{try{const value=localStorage.getItem(sortKey);if(value==='name'||value==='time')return value;}catch{/* Default applies when storage is unavailable. */}return 'rating'}
export function useLibraryView(visible:boolean,loaded:boolean){
 const [initial]=useState(readView);
 const [q,rawQ]=useState(initial.q),[tag,rawTag]=useState(initial.tag),[rating,rawRating]=useState<RatingFilter>(initial.rating),[sort,rawSort]=useState<LibrarySort>(readSort),[selected,setSelected]=useState(initial.selected);
 const scroll=useRef(initial.scroll),latest=useRef({q,tag,rating,selected});latest.current={q,tag,rating,selected};
 const save=()=>{try{sessionStorage.setItem(viewKey,JSON.stringify({...latest.current,scroll:scroll.current}))}catch{/* Storage limits never block browsing. */}};
 useEffect(save,[q,tag,rating,selected]);
 useEffect(()=>{try{localStorage.setItem(sortKey,sort)}catch{/* Keep the selected sort in memory. */}},[sort]);
 useEffect(()=>{const previous=history.scrollRestoration;history.scrollRestoration='manual';return()=>{history.scrollRestoration=previous}},[]);
 useLayoutEffect(()=>{
  if(!visible||!loaded)return;
  window.scrollTo({top:scroll.current,behavior:'instant'});
  let timer=0;
  const capture=()=>{scroll.current=window.scrollY;window.clearTimeout(timer);timer=window.setTimeout(save,100)};
  window.addEventListener('scroll',capture,{passive:true});
  return()=>{window.removeEventListener('scroll',capture);window.clearTimeout(timer);save()};
 },[visible,loaded]);
 const capturePosition=()=>{if(visible&&loaded){scroll.current=window.scrollY;save()}};
 const resetScroll=()=>{scroll.current=0;if(visible)window.scrollTo({top:0,behavior:'instant'})};
 const setQ=(value:string)=>{resetScroll();rawQ(value)};
 const setTag=(value:string)=>{resetScroll();rawTag(value)};
 const setRating=(value:RatingFilter)=>{resetScroll();rawRating(value)};
 const setSort=(value:LibrarySort)=>{resetScroll();rawSort(value)};
 const clearFilters=()=>{resetScroll();rawQ('');rawTag('');rawRating('all')};
 return {capturePosition,q,setQ,tag,setTag,rating,setRating,sort,setSort,selected,setSelected,clearFilters};
}
