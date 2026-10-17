import {useEffect,useRef,useState} from 'react';
import {searchSites,searchWebRecipes,SearchSite,SearchResults} from '../api';

export function useRecipeSearch(open:boolean){
 const [sites,setSites]=useState<SearchSite[]>([]),[q,setQ]=useState(''),[site,setSite]=useState('all');
 const [data,setData]=useState<SearchResults|null>(null),[loading,setLoading]=useState(false),[error,setError]=useState(''),[submitted,setSubmitted]=useState('');
 const current=useRef<AbortController|null>(null);
 useEffect(()=>{
  if(!open||sites.length)return;
  let live=true;searchSites().then(s=>{if(live)setSites(s)}).catch(e=>{if(live)setError(e.message)});
  return()=>{live=false};
 },[open,sites.length]);
 useEffect(()=>()=>current.current?.abort(),[]);
 const search=async()=>{
  current.current?.abort();const request=new AbortController();current.current=request;
  setLoading(true);setError('');setData(null);setSubmitted(q.trim());
  try {const result=await searchWebRecipes(q.trim(),site,request.signal);if(current.current===request)setData(result);}
  catch(e){if(!request.signal.aborted)setError(e instanceof Error?e.message:String(e));}
  finally{if(current.current===request)setLoading(false);}
 };
 return {sites,q,setQ,site,setSite,data,loading,error,submitted,search};
}
