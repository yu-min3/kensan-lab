import {Panel,Field} from '../components/Editor';
import {useRecipeSearch} from '../hooks/useRecipeSearch';

type Search=ReturnType<typeof useRecipeSearch>;
export function SearchPage({search:s,onClose,onImport}:{search:Search;onClose:()=>void;onImport:(url:string)=>void}){
 return <main className="editor-page search-page">
  <header className="header-row"><h1 className="h-serif">Webでレシピを探す</h1><button className="chip" onClick={onClose}>一覧へ</button></header>
  <Panel>
   <p>料理名や材料から探して、そのまま取り込めます。</p>
   <form onSubmit={e=>{e.preventDefault();void s.search()}} aria-label="Webレシピ検索">
    <Field id="web-query" label="料理名・材料" value={s.q} onChange={s.setQ} maxLength={100}/>
    <label htmlFor="web-site">検索するサイト<select id="web-site" value={s.site} onChange={e=>s.setSite(e.target.value)}><option value="all">主要サイトまとめて</option>{s.sites.map(site=><option key={site.id} value={site.id}>{site.name}</option>)}</select></label>
    <p className="recipe-summary">まとめて検索：デリッシュ・クラシル・白ごはん・きょうの料理・YouTube</p>
    <button className="start-button" disabled={!s.q.trim()} type="submit">{s.loading?'検索し直す':'検索する'}</button>
   </form>
  </Panel>
  {s.loading&&<p role="status"><span className="import-spinner" aria-hidden="true"/>Webで候補を探しています…</p>}
  {s.error&&<p className="notice" role="alert">{s.error}</p>}
  {s.data&&<>
   <h2 className="h-serif">「{s.submitted}」の検索結果</h2>
   {s.data.warnings.length>0&&<details className="editor-panel" open={!s.data.results.length}><summary>検索の補足（{s.data.warnings.length}件）</summary>{s.data.warnings.map((w,i)=><p key={i}>{w}</p>)}</details>}
   {!s.data.results.length&&<p role="status">候補が見つかりませんでした。言葉やサイトを変えるか、URLから取り込んでください。</p>}
   <ul className="web-results">{s.data.results.map(result=><li key={result.url}><Panel>
    <p className="recipe-summary">{result.siteName}{result.site==='youtube'?' · 動画の概要欄から取り込み':''}</p>
    <h3>{result.title}</h3>
    <div className="editor-actions"><button className="chip chip-on" onClick={()=>onImport(result.url)}>このレシピを取り込む</button><a href={result.url} target="_blank" rel="noreferrer">元のページを見る</a></div>
   </Panel></li>)}</ul>
  </>}
  <Panel><h2 className="h-serif">見つからないとき</h2><p>取得制限やページの変更で検索できない場合は、「レシピを追加」からURL・本文を貼り付けてください。YouTubeは概要欄を読み取ります。字幕・音声・映像は取得しません。</p><button className="chip" onClick={()=>onImport('')}>URL・本文から取り込む</button></Panel>
 </main>;
}
