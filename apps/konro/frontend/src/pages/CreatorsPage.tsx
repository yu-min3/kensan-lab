import {useRef,useState} from 'react';
import {Heart,Search,ExternalLink} from 'lucide-react';
import {creatorRequest,FavoriteCreator,creatorSite} from '../creatorApi';
import {useFavoriteCreators} from '../hooks/useFavoriteCreators';
import {CreatorNotice} from '../components/CreatorNotice';
import {Panel} from '../components/Editor';
type Favorites=ReturnType<typeof useFavoriteCreators>;
export function CreatorsPage({favorites:f,onClose,onSelect}:{favorites:Favorites;onClose:()=>void;onSelect:(id:string)=>void}){
 const [url,setURL]=useState(''),[resolved,setResolved]=useState<FavoriteCreator|null>(null),[resolving,setResolving]=useState(false),[error,setError]=useState('');const request=useRef(0);
 const additionalCount=f.creators.filter(c=>c.additional).length;
 const check=async()=>{const key=++request.current;setResolving(true);setResolved(null);setError('');try{const value=await creatorRequest<FavoriteCreator>('/resolve','POST',{url});if(request.current===key)setResolved(value)}catch(e){if(request.current===key)setError(e instanceof Error?e.message:String(e))}finally{if(request.current===key)setResolving(false)}};
 return <main className="editor-page creators-page">
  <header className="header-row"><h1 className="h-serif">好きな作り手</h1><button className="chip" onClick={onClose}>検索へ戻る</button></header>
  <CreatorNotice favorites={f}/>
  <Panel><h2 className="h-serif">公式URLから追加</h2><p>チャンネル・料理人の名前を確認してから追加できます。</p>
   <form onSubmit={e=>{e.preventDefault();void check()}}>
    <label htmlFor="creator-url">公式チャンネル・料理人のURL<input id="creator-url" inputMode="url" autoCapitalize="none" autoCorrect="off" value={url} onChange={e=>{setURL(e.target.value);setResolved(null);request.current++;setResolving(false);setError('')}} placeholder="youtube.com/@… または kurashiru.com/chefs/…"/></label>
    <p className="recipe-summary">対応：YouTubeチャンネル、クラシルの料理人プロフィール。動画やレシピ単体のURLは使いません。</p>
    <button className="recipe-action recipe-action-primary" disabled={resolving||f.busy||!url.trim()} type="submit">{resolving?'確認中…':'作り手を確認'}</button>
   </form>
   {error&&<p className="notice" role="alert">{error}</p>}
   {resolved&&<div className="creator-confirmation"><h3>{resolved.name}</h3><p>{creatorSite(resolved.id)}</p><a href={resolved.url} target="_blank" rel="noreferrer">公式プロフィールを開く<ExternalLink aria-hidden="true"/></a>
    {f.creators.some(c=>c.id===resolved.id)?<p role="status">追加済みです</p>:<button className="recipe-action" disabled={f.busy||f.loading||!f.settings} onClick={()=>void f.add(resolved)}>この作り手を追加</button>}
   </div>}
  </Panel>
  <section aria-labelledby="favorite-list-title"><h2 id="favorite-list-title" className="h-serif">追加した作り手（{f.creators.length}人）</h2>
   <p className="recipe-summary">「追加でも探す」は最大3人。通常の検索に現れたお気に入りは、全員を優先します。</p>
   {f.loading&&<p role="status">一覧を読み込んでいます…</p>}
   {!f.loading&&f.settings&&!f.creators.length&&<p>まだ登録されていません。公式URLや検索カードから追加できます。</p>}
   <ul className="creator-list">{f.creators.map(creator=><li className="editor-panel" key={creator.id}>
    <h3><Heart aria-hidden="true"/>{creator.name}</h3><p className="recipe-summary">{creatorSite(creator.id)}</p>
    <label className="creator-additional"><input type="checkbox" checked={creator.additional} disabled={f.busy||f.loading||(!creator.additional&&additionalCount>=3)} onChange={e=>void f.additional(creator,e.target.checked)}/>追加でも探す</label>
    <div className="editor-actions"><button className="recipe-action" onClick={()=>onSelect(creator.id)}><Search aria-hidden="true"/>この作り手で探す</button><button className="chip" disabled={f.busy||f.loading} aria-label={`${creator.name}を好きな作り手から外す`} onClick={()=>void f.remove(creator)}>外す</button><a href={creator.url} target="_blank" rel="noreferrer">公式プロフィール</a></div>
   </li>)}</ul>
  </section>
 </main>;
}
