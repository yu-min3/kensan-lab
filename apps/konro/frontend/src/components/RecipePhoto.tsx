import {useRef} from 'react';
import {Camera,ImagePlus,Upload} from 'lucide-react';
import {useRecipePhoto} from '../hooks/useRecipePhoto';
export function RecipePhoto({image,photo}:{image?:string;photo:ReturnType<typeof useRecipePhoto>}){
 const input=useRef<HTMLInputElement>(null),blocked=photo.busy||photo.preparing;
 return <section className="recipe-photo" aria-label="レシピの写真" aria-busy={blocked}>
  {(photo.preview||image)&&<img className="recipe-cover" src={photo.preview||`/${image}`} alt={photo.preview?'保存前の写真プレビュー':'料理の写真'}/>}
  <input ref={input} hidden type="file" accept="image/jpeg,image/png,image/webp,image/gif,image/heic,image/heif" aria-label="料理の写真を選ぶ" disabled={blocked} onChange={e=>{const file=e.target.files?.[0];e.target.value='';if(file)void photo.pick(file)}}/>
  {photo.error&&<div className="notice recipe-error" role="alert"><p>{photo.error}</p>{photo.conflict&&<><p>写真はまだ保存されていません。最新版を確認してから選び直してください。</p><button type="button" className="recipe-action" disabled={blocked} onClick={()=>void photo.reload()}>最新版を確認</button></>}</div>}
  {(blocked||photo.dirty||photo.saved)&&<p className="recipe-summary" role="status">{photo.preparing?'写真を準備しています…':photo.busy?'写真を保存しています…':photo.dirty?'写真はまだ保存されていません':'写真を保存しました。'}</p>}
  <div className="editor-actions">
   <button type="button" className="recipe-action recipe-action-ghost" disabled={blocked} onClick={()=>input.current?.click()}><ImagePlus aria-hidden="true"/>{photo.dirty?'別の写真を選ぶ':image?'写真を変更':'写真を追加'}</button>
   {photo.dirty&&<><button type="button" className="recipe-action" disabled={blocked} onClick={photo.cancel}>写真をキャンセル</button><button type="button" className="recipe-action recipe-action-primary" disabled={blocked||photo.conflict} onClick={()=>void photo.save()}><Upload aria-hidden="true"/>{photo.busy?'保存中…':'写真を保存'}</button></>}
  </div>
 </section>;
}
