import { useEffect, useState } from 'react';
import { ImageOff, Download, ExternalLink, Heart } from 'lucide-react';
import {Panel,Field} from '../components/Editor';
import {useRecipeSearch} from '../hooks/useRecipeSearch';
import {WebRecipe} from '../api';
import {SourceCredit} from '../components/SourceCredit';
import {useSourcePreview} from '../hooks/useSourcePreview';
import {useFavoriteCreators} from '../hooks/useFavoriteCreators';
import {canFollow,creatorURL,FavoriteCreator} from '../creatorApi';
import {CreatorNotice} from '../components/CreatorNotice';
import {CreatorsPage} from './CreatorsPage';

type Search=ReturnType<typeof useRecipeSearch>;
type Favorites=ReturnType<typeof useFavoriteCreators>;
function SearchCard({result,onImport,favorites:f}:{result:WebRecipe;onImport:(url:string)=>void;favorites:Favorites}) {
 const [failed,setFailed]=useState(false);
 const {ref,info}=useSourcePreview(result.url,result.info);
 const person=info?.creator;
 const favorite=f.creators.find(c=>c.id===person?.id);
 const chef=person?.id?.startsWith('kurashiru:chef:')||person?.id?.startsWith('nhk:');
 const toggle=()=>{if(!person?.id)return;const creator:FavoriteCreator={id:person.id,name:person.name,url:creatorURL(person.id),additional:false};void(favorite?f.remove(favorite):f.add(creator));};
 return <li ref={ref} className="web-recipe-card">
  <button className="web-recipe-pick" onClick={()=>onImport(result.url)} aria-label={`${result.title}を取り込む`}>
   <div className="web-recipe-photo">{result.image&&!failed?<img src={result.image} alt="" loading="lazy" referrerPolicy="no-referrer" onError={()=>setFailed(true)}/>:<span className="web-recipe-no-photo"><ImageOff aria-hidden="true"/>写真なし</span>}</div>
   <div className="web-recipe-body">{favorite&&<span className="recipe-tag">好きな作り手</span>}<h3 className="h-serif">{result.title}</h3>{result.description&&<p className="web-recipe-description">{result.description}</p>}</div>
  </button>
  <div className="web-recipe-meta">
   {person&&<div className="creator-follow-row"><p className="source-creator">{chef?'料理人':'投稿者'}：{person.name}</p>{canFollow(person.id)&&<button className="creator-follow-button" aria-pressed={!!favorite} aria-label={`${person.name}を好きな作り手${favorite?'から外す':'に追加'}`} disabled={f.busy||f.loading||!f.settings} onClick={toggle}><Heart aria-hidden="true" fill={favorite?'currentColor':'none'}/><span>{favorite?'追加済み':'追加'}</span></button>}</div>}
   <SourceCredit info={info} showCreator={false} showPublisher={false}/><p className="recipe-summary">配信元：{info?.publisher?.name||result.siteName}{result.site==='youtube'?' · 動画':''}</p>
   <button className="recipe-action" onClick={()=>onImport(result.url)}><Download aria-hidden="true"/>取り込む</button>
  </div>
  <a className="web-recipe-source" href={result.url} target="_blank" rel="noreferrer">元のページを見る<ExternalLink aria-hidden="true"/></a>
 </li>;
}
export function SearchPage({search:s,onClose,onImport,managing,onManage,onCloseManage}:{search:Search;onClose:()=>void;onImport:(url:string)=>void;managing:boolean;onManage:()=>void;onCloseManage:()=>void}){
 const favorites=useFavoriteCreators();
 useEffect(()=>{if(favorites.settings&&s.creator&&!favorites.creators.some(c=>c.id===s.creator)){s.setCreator('');if(s.q.trim())void s.search('');else s.clearResults();}},[favorites.settings,s.creator]);
 const select=(id:string)=>{s.setCreator(id);if(managing)onCloseManage();if(s.q.trim())void s.search(id);else s.clearResults();};
 if(managing)return <CreatorsPage favorites={favorites} onClose={onCloseManage} onSelect={select}/>;
 return <main className="editor-page search-page">
  <header className="header-row"><h1 className="h-serif">Webでレシピを探す</h1><button className="chip" onClick={onClose}>一覧へ</button></header>
  <div className="creator-search-tools"><button className="recipe-action" onClick={onManage}><Heart aria-hidden="true"/>好きな作り手を管理</button></div>
  <CreatorNotice favorites={favorites}/>
  <div className="creator-filters" role="group" aria-label="作り手を指定"><button className={`chip ${!s.creator?'chip-on':''}`} aria-pressed={!s.creator} onClick={()=>select('')}>すべて</button>{favorites.creators.map(c=><button key={c.id} className={`chip ${s.creator===c.id?'chip-on':''}`} aria-pressed={s.creator===c.id} onClick={()=>select(c.id)}>{c.name}</button>)}</div>
  <Panel>
   <form onSubmit={e=>{e.preventDefault();void s.search()}} aria-label="Webレシピ検索">
    <div className="web-search-query"><Field id="web-query" label="料理名・材料" value={s.q} onChange={s.setQ} maxLength={100}/>
    <button className="recipe-action recipe-action-primary" disabled={!s.q.trim()} type="submit">{s.loading?'検索し直す':'検索する'}</button></div>
    <label htmlFor="web-site">検索するサイト<select id="web-site" value={s.creator?(s.creator.startsWith('youtube:')?'youtube':'kurashiru'):s.site} disabled={!!s.creator} onChange={e=>s.setSite(e.target.value)}><option value="all">主要サイトまとめて</option>{s.sites.map(site=><option key={site.id} value={site.id}>{site.name}</option>)}</select></label>
   </form>
  </Panel>
  {s.loading&&<p role="status"><span className="import-spinner" aria-hidden="true"/>Webで候補を探しています…</p>}
  {s.error&&<p className="notice" role="alert">{s.error}</p>}
  {s.data&&<>
   <h2 className="h-serif">「{s.submitted}」の検索結果</h2>
   {s.data.results.some(r=>r.preferred)&&<p className="recipe-summary">検索内容に合う好きな作り手を先に表示しています。</p>}
   {s.data.warnings.length>0&&<details className="editor-panel" open={!s.data.results.length}><summary>検索の補足（{s.data.warnings.length}件）</summary>{s.data.warnings.map((w,i)=><p key={i}>{w}</p>)}</details>}
   {!s.data.results.length&&<p role="status">候補が見つかりませんでした。言葉やサイトを変えるか、URLから取り込んでください。</p>}
   <ul className="web-results web-recipe-grid">{s.data.results.map(result=><SearchCard key={result.url} result={result} onImport={onImport} favorites={favorites}/>)}</ul>
  </>}
  <Panel><h2 className="h-serif">URLからも取り込めます</h2><p>検索で見つからなければ、レシピのURLや本文を貼り付けてください。</p><button className="chip" onClick={()=>onImport('')}>URL・本文から取り込む</button></Panel>
 </main>;
}
