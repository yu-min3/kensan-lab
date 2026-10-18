import {useEffect,useRef,useState} from 'react';
import {searchSites,searchWebRecipes,SearchSite,SearchResults} from '../api';

const cacheKey='konro.web-search';
function readSearch(){
 try{const raw=sessionStorage.getItem(cacheKey);if(!raw||raw.length>256000)return null;const v=JSON.parse(raw);if(typeof v.q==='string'&&typeof v.site==='string'&&typeof v.creator==='string'&&typeof v.submitted==='string'&&(!v.data||(Array.isArray(v.data.results)&&Array.isArray(v.data.warnings))))return v;}catch{/* session cache is optional */}
 return null;
}
export function useRecipeSearch(open:boolean){
 const [cached]=useState(readSearch);
 const [sites,setSites]=useState<SearchSite[]>([]),[q,setQ]=useState<string>(cached?.q??''),[site,setSite]=useState<string>(cached?.site??'all');
 const [data,setData]=useState<SearchResults|null>(cached?.data??null),[loading,setLoading]=useState(false),[error,setError]=useState(''),[submitted,setSubmitted]=useState<string>(cached?.submitted??'');
 const [creator,setCreator]=useState<string>(cached?.creator??'');
 useEffect(()=>{try{sessionStorage.setItem(cacheKey,JSON.stringify({q,site,creator,submitted,data}))}catch{/* storage may be full or disabled */}},[q,site,creator,submitted,data]);
 const current=useRef<AbortController|null>(null);
 useEffect(()=>{
  if(!open||sites.length)return;
  let live=true;searchSites().then(s=>{if(live)setSites(s)}).catch(e=>{if(live)setError(e.message)});
  return()=>{live=false};
 },[open,sites.length]);
 useEffect(()=>()=>current.current?.abort(),[]);
 const search=async(selectedCreator=creator)=>{
  current.current?.abort();const request=new AbortController();current.current=request;
  setLoading(true);setError('');setData(null);setSubmitted(q.trim());
  try {const result=await searchWebRecipes(q.trim(),site,request.signal,selectedCreator);if(current.current===request)setData(result);}
  catch(e){if(!request.signal.aborted)setError(e instanceof Error?e.message:String(e));}
  finally{if(current.current===request)setLoading(false);}
 };
 const clearResults=()=>{current.current?.abort();current.current=null;setData(null);setSubmitted('');setLoading(false);};
 return {clearResults,creator,setCreator,sites,q,setQ,site,setSite,data,loading,error,submitted,search};
}
