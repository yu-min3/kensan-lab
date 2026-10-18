import {useEffect,useRef,useState} from 'react';
import {CreatorError,creatorRequest,CreatorMutation,CreatorSettings,FavoriteCreator} from '../creatorApi';
export function useFavoriteCreators(){
 const [settings,setSettings]=useState<CreatorSettings|null>(null),[loading,setLoading]=useState(true),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[undo,setUndo]=useState('');
 const mounted=useRef(true);const gate=useRef(false);
 useEffect(()=>{mounted.current=true;void refresh();return()=>{mounted.current=false};},[]);
 async function refresh(){setLoading(true);try{const value=await creatorRequest<CreatorSettings>();if(mounted.current){setSettings(value);setError('')}}catch(e){if(mounted.current)setError(e instanceof Error?e.message:String(e))}finally{if(mounted.current)setLoading(false)}}
 async function mutate(path:string,method:string,payload:object,message:string){
  if(gate.current||!settings)return false;gate.current=true;setBusy(true);setError('');setNotice('');setUndo('');
  try{const value=await creatorRequest<CreatorMutation>(path,method,{revision:settings.revision,...payload});if(mounted.current){setSettings(value);setUndo(value.undo??'');setNotice(message)};return true}
  catch(e){if(mounted.current){if(e instanceof CreatorError&&e.status===409){try{setSettings(await creatorRequest<CreatorSettings>())}catch{/* keep last list, show conflict */}}setError(e instanceof Error?e.message:String(e))};return false}
  finally{gate.current=false;if(mounted.current)setBusy(false)}
 }
 const add=(creator:FavoriteCreator)=>mutate('','POST',{url:creator.url},`${creator.name}を好きな作り手に追加しました。次の検索から反映します。`);
 const remove=(creator:FavoriteCreator)=>mutate('/'+encodeURIComponent(creator.id),'DELETE',{},`${creator.name}を好きな作り手から外しました。`);
 const additional=(creator:FavoriteCreator,on:boolean)=>mutate('/'+encodeURIComponent(creator.id),'PATCH',{additional:on},'追加でも探す作り手を更新しました。');
 const undoLast=()=>mutate('/undo','POST',{token:undo},'変更を取り消しました。');
 return {settings,creators:settings?.creators??[],loading,busy,error,notice,undo,refresh,add,remove,additional,undoLast};
}
