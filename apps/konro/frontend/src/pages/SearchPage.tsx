import { useState } from 'react';
import { ImageOff, Download, ExternalLink } from 'lucide-react';
import {Panel,Field} from '../components/Editor';
import {useRecipeSearch} from '../hooks/useRecipeSearch';
import {WebRecipe} from '../api';
import {SourceCredit} from '../components/SourceCredit';
import {useSourcePreview} from '../hooks/useSourcePreview';

type Search=ReturnType<typeof useRecipeSearch>;
function SearchCard({result,onImport}:{result:WebRecipe;onImport:(url:string)=>void}) {
 const [failed,setFailed]=useState(false);
 const {ref,info}=useSourcePreview(result.url,result.info);
 return <li ref={ref} className="web-recipe-card">
  <button className="web-recipe-pick" onClick={()=>onImport(result.url)} aria-label={`${result.title}を取り込む`}>
   <div className="web-recipe-photo">{result.image&&!failed?<img src={result.image} alt="" loading="lazy" referrerPolicy="no-referrer" onError={()=>setFailed(true)}/>:<span className="web-recipe-no-photo"><ImageOff aria-hidden="true"/>写真なし</span>}</div>
   <div className="web-recipe-body">{result.preferred&&<span className="recipe-tag">好きな作り手</span>}<h3 className="h-serif">{result.title}</h3><SourceCredit info={info} showPublisher={false}/><p className="recipe-summary">配信元：{info?.publisher?.name||result.siteName}{result.site==='youtube'?' · 動画':''}</p>{result.description&&<p className="web-recipe-description">{result.description}</p>}<span className="web-recipe-import"><Download aria-hidden="true"/>取り込む</span></div>
  </button>
  <a className="web-recipe-source" href={result.url} target="_blank" rel="noreferrer">元のページを見る<ExternalLink aria-hidden="true"/></a>
 </li>;
}
export function SearchPage({search:s,onClose,onImport}:{search:Search;onClose:()=>void;onImport:(url:string)=>void}){
 return <main className="editor-page search-page">
  <header className="header-row"><h1 className="h-serif">Webでレシピを探す</h1><button className="chip" onClick={onClose}>一覧へ</button></header>
  <Panel>
   <form onSubmit={e=>{e.preventDefault();void s.search()}} aria-label="Webレシピ検索">
    <div className="web-search-query"><Field id="web-query" label="料理名・材料" value={s.q} onChange={s.setQ} maxLength={100}/>
    <button className="recipe-action recipe-action-primary" disabled={!s.q.trim()} type="submit">{s.loading?'検索し直す':'検索する'}</button></div>
    <label htmlFor="web-site">検索するサイト<select id="web-site" value={s.site} onChange={e=>s.setSite(e.target.value)}><option value="all">主要サイトまとめて</option>{s.sites.map(site=><option key={site.id} value={site.id}>{site.name}</option>)}</select></label>
   </form>
  </Panel>
  {s.loading&&<p role="status"><span className="import-spinner" aria-hidden="true"/>Webで候補を探しています…</p>}
  {s.error&&<p className="notice" role="alert">{s.error}</p>}
  {s.data&&<>
   <h2 className="h-serif">「{s.submitted}」の検索結果</h2>
   {s.data.results.some(r=>r.preferred)&&<p className="recipe-summary">検索内容に合う好きな作り手を先に表示しています。</p>}
   {s.data.warnings.length>0&&<details className="editor-panel" open={!s.data.results.length}><summary>検索の補足（{s.data.warnings.length}件）</summary>{s.data.warnings.map((w,i)=><p key={i}>{w}</p>)}</details>}
   {!s.data.results.length&&<p role="status">候補が見つかりませんでした。言葉やサイトを変えるか、URLから取り込んでください。</p>}
   <ul className="web-results web-recipe-grid">{s.data.results.map(result=><SearchCard key={result.url} result={result} onImport={onImport}/>)}</ul>
  </>}
  <Panel><h2 className="h-serif">URLからも取り込めます</h2><p>検索で見つからなければ、レシピのURLや本文を貼り付けてください。</p><button className="chip" onClick={()=>onImport('')}>URL・本文から取り込む</button></Panel>
 </main>;
}
