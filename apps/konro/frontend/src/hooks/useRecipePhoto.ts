import {useEffect,useRef,useState} from 'react';
import {fetchRecipe,Recipe,RecipeWriteError,saveRecipePhoto} from '../api';

async function preparePhoto(file:File):Promise<Blob>{
 if(file.size>20*1024*1024)throw new Error('20MB以下の写真を選んでください。');
 if(!/^image\/(jpeg|png|webp|gif|heic|heif)$/i.test(file.type))throw new Error('写真を選んでください（JPEG・PNG・WebPなど）。');
 let bitmap:ImageBitmap;
 try{bitmap=await createImageBitmap(file,{imageOrientation:'from-image'});}catch{throw new Error('この写真を読み込めませんでした。JPEG・PNGなどに変換して選び直してください。');}
 try{
  const scale=Math.min(1,2048/Math.max(bitmap.width,bitmap.height));
  const canvas=document.createElement('canvas');canvas.width=Math.max(1,Math.round(bitmap.width*scale));canvas.height=Math.max(1,Math.round(bitmap.height*scale));
  const ctx=canvas.getContext('2d');if(!ctx)throw new Error('写真を処理できませんでした。');
  ctx.fillStyle='white';ctx.fillRect(0,0,canvas.width,canvas.height);ctx.drawImage(bitmap,0,0,canvas.width,canvas.height);
  return await new Promise<Blob>((resolve,reject)=>canvas.toBlob(blob=>blob?resolve(blob):reject(new Error('写真を処理できませんでした。')),'image/jpeg',0.88));
 }finally{bitmap.close()}
}
export function useRecipePhoto(recipe:Recipe|null,onSaved:(r:Recipe)=>void){
 const [photo,setPhoto]=useState<Blob|null>(null),[preview,setPreview]=useState(''),[busy,setBusy]=useState(false),[preparing,setPreparing]=useState(false),[error,setError]=useState(''),[conflict,setConflict]=useState(false),[saved,setSaved]=useState(false);
 const live=useRef(true);useEffect(()=>{live.current=true;return()=>{live.current=false}},[]);
 useEffect(()=>{if(!photo){setPreview('');return}const url=URL.createObjectURL(photo);setPreview(url);return()=>URL.revokeObjectURL(url)},[photo]);
 const dirty=!!photo||preparing;
 useEffect(()=>{if(!dirty)return;const guard=(e:BeforeUnloadEvent)=>{e.preventDefault();e.returnValue=''};window.addEventListener('beforeunload',guard);return()=>window.removeEventListener('beforeunload',guard)},[dirty]);
 const discard=()=>!dirty||window.confirm('保存していない写真を破棄しますか？');
 const cancel=()=>{setPhoto(null);setError('');setConflict(false);setSaved(false)};
 const pick=async(file:File)=>{setPreparing(true);setError('');setConflict(false);setSaved(false);try{const blob=await preparePhoto(file);if(live.current)setPhoto(blob)}catch(e){if(live.current)setError(e instanceof Error?e.message:String(e))}finally{if(live.current)setPreparing(false)}};
 const save=async()=>{if(!recipe||!photo)return;setBusy(true);setError('');setConflict(false);try{const result=await saveRecipePhoto(recipe.file,recipe.version,photo);setPhoto(null);setSaved(true);onSaved(result)}catch(e){setError(e instanceof Error?e.message:String(e));setConflict(e instanceof RecipeWriteError&&e.status===409)}finally{setBusy(false)}};
 const reload=async()=>{if(!recipe||!discard())return;setBusy(true);try{const result=await fetchRecipe(recipe.file);cancel();onSaved(result)}catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}};
 return {preview,busy,preparing,error,conflict,saved,dirty,discard,cancel,pick,save,reload};
}
