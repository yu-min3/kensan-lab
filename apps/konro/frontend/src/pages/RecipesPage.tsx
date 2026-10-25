import {Clock,ChefHat} from 'lucide-react';
import {RecipeAddMenu} from '../components/RecipeAddMenu';
import {PersonalRating} from '../components/PersonalRating';
import {CookingSelection} from '../components/CookingSelection';
import { useEffect, useMemo, useState } from "react";
import { fetchRecipes, formatDuration, importRecipeZip, RecipeMeta } from "../api";
import { RecipePage } from "./RecipePage";
import { SearchPage } from "./SearchPage";
import { useRecipeSearch } from "../hooks/useRecipeSearch";
import { ImportPage } from "./ImportPage";
import { useRecipeRoute,Route } from "../hooks/useRecipeRoute";
import {libraryRecipes,LibrarySort} from "../library";
import {useLibraryView} from "../hooks/useLibraryView";
import {TagFilters,Taxonomy} from '../components/TagFilters';

// Recipe browser + session builder: tap cards to put recipes "on the stove",
// then start the cooking session.
export function RecipesPage({ onStart, starting = false, routing, cooking=false }: { onStart: (files: string[]) => void; starting?: boolean; routing:ReturnType<typeof useRecipeRoute>; cooking?:boolean }) {
  const { route, navigate:routeNavigate, back, onGuard } = routing;
  const discovering=route.page==='search'||route.page==='creators';
  const search=useRecipeSearch(discovering);
  const setOpened=(file:string|null,replace=false)=>navigate(file===null?{page:'list'}:{page:'recipe',file},replace);
  const [recipes, setRecipes] = useState<RecipeMeta[]>([]);
  const [error, setError] = useState("");
  const [loaded,setLoaded]=useState(false);
  const {capturePosition,q,setQ,tags,setTags,rating,setRating,sort,setSort,selected,setSelected,clearFilters}=useLibraryView(route.page==='list',loaded);
  const [taxonomy,setTaxonomy]=useState<Taxonomy>({revision:'',assignments:{},groups:[{id:'cuisine',category:'料理種類',label:'系統'},{id:'dish',category:'料理種類',label:'品目'},{id:'ingredient',category:'素材種類',label:'素材'},{id:'other',category:'その他',label:'自由タグ'}]});
  const [taxonomyError,setTaxonomyError]=useState('');
  useEffect(()=>{fetch('/api/v1/tag-taxonomy').then(async r=>{if(!r.ok)throw new Error('タグ分類を読み込めません。タグはその他に表示しています。');setTaxonomy(await r.json())}).catch(e=>setTaxonomyError(e.message));},[]);
  const navigate=(next:Route,replace=false)=>{capturePosition();routeNavigate(next,replace)};
  const [importing, setImporting] = useState(false);
  const [importMsg, setImportMsg] = useState("");

  useEffect(() => {
    fetchRecipes().then(setRecipes).catch((e) => setError(String(e))).finally(()=>setLoaded(true));
  }, []);
  useEffect(()=>{if(!loaded||error)return;setSelected(current=>{const next=current.filter(file=>recipes.some(r=>r.file===file));return next.length===current.length?current:next})},[recipes,loaded,error]);

  const importZip = async (file: File) => {
    setImporting(true);
    setImportMsg("");
    try {
      const rep = await importRecipeZip(file);
      const drift = rep.unknownProps?.length
        ? `｜⚠ 未知フィールド ${rep.unknownProps.length} 種（形式が変わったかも）`
        : "";
      setImportMsg(`✓ ${rep.recipes} 品を取り込みました（画像 ${rep.images} 枚）${rep.skipped ? `｜編集済み ${rep.skipped} 品を保持` : ""}${drift}`);
      setRecipes(await fetchRecipes());
    } catch (e) {
      setImportMsg(`取込に失敗しました: ${String(e)}`);
    } finally {
      setImporting(false);
    }
  };

  const shown=useMemo(()=>libraryRecipes(recipes,q,tags,rating,sort,taxonomy.assignments),[recipes,q,tags,rating,sort,taxonomy]);

  const toggle = (file: string) =>
    setSelected((sel) => (sel.includes(file) ? sel.filter((f) => f !== file) : [...sel, file]));

  if (route.page === "recipe") { const opened=route.file; return <RecipePage onGuard={onGuard} key={opened} file={opened || null} selected={selected.includes(opened)} onBack={() => back({page:"list"})} onToggle={toggle} onSaved={(r) => {
    setOpened(r.file, true); setImportMsg(`✓ ${r.title} を保存しました`);
    fetchRecipes().then(setRecipes).catch((e) => setError(String(e)));
  }} />;

  }
  if (discovering) return <SearchPage search={search} managing={route.page==='creators'} onManage={()=>navigate({page:'creators'})} onCloseManage={()=>back({page:'search'})} onClose={()=>back({page:'list'})} onImport={(url)=>navigate({page:'import',url,fromSearch:true})} />;

  if (route.page==='import') return <ImportPage key={route.url||route.draft||'manual'} initialURL={route.url} initialDraft={route.draft} returnToSearch={route.fromSearch} onGuard={onGuard} onDraft={(id)=>navigate({...route,draft:id},true)} onClose={()=>back({page:route.fromSearch?'search':'list'},true)} onSaved={(file, title) => {
    navigate({page:'list'},true); setImportMsg(`✓ ${title} を保存しました`);
    setQ(""); setTag(""); setSelected((s) => s.includes(file) ? s : [...s, file]);
    fetchRecipes().then(setRecipes).catch((e) => setError(String(e)));
  }} />;

  return (
    <div className="recipes-page">
      <header className="list-header">
        <div className="header-row">
          <h1 className="h-serif">konro</h1>
          <div className="editor-actions"><button className="recipe-action" onClick={()=>navigate({page:"search"})}>Webで探す</button><RecipeAddMenu importing={importing} onImport={()=>navigate({page:'import',url:'',fromSearch:false})} onNew={()=>setOpened('')} onZip={file=>void importZip(file)}/></div>
        </div>
        {importMsg && (
          <p
            className={`import-msg ${importMsg.startsWith("✓") ? "" : "error"}`}
            onClick={() => setImportMsg("")}
          >
            {importMsg}
          </p>
        )}
        <div className="library-search-tools">
          <label className="library-query" htmlFor="library-query"><span>レシピを探す</span><input id="library-query" type="search" aria-label="レシピを検索" placeholder="料理名・タグで検索" value={q} onChange={e=>setQ(e.target.value)}/></label>
          <label htmlFor="library-sort"><span id="library-sort-label">並べ替え</span><select aria-labelledby="library-sort-label" id="library-sort" value={sort} onChange={e=>setSort(e.target.value as LibrarySort)}><option value="rating">評価が高い順</option><option value="name">名前順</option><option value="time">調理時間が短い順</option></select></label>
        </div>
        <div className="library-rating-filters" role="group" aria-label="自分の評価で絞る">{([['all','すべて'],['high','★4以上'],['unrated','未評価']] as const).map(([value,label])=><button key={value} className={`chip ${rating===value?'chip-on':''}`} aria-pressed={rating===value} onClick={()=>setRating(value)}>{label}</button>)}</div>
        {taxonomyError&&<p className="notice" role="alert">{taxonomyError}</p>}
        <TagFilters recipes={recipes} q={q} rating={rating} sort={sort} selected={tags} onChange={setTags} taxonomy={taxonomy}/>
      </header>

      {error && <p className="error">{error}</p>}

      <div className="library-result-info"><p className="recipe-summary" aria-live="polite">{loaded?`${shown.length}品 / 全${recipes.length}品`:'レシピを読み込んでいます…'} · 星は自分の評価</p>{(q||tags.length>0||rating!=='all')&&<button className="chip" onClick={clearFilters}>絞り込みを解除</button>}</div>
      <div className="library-layout"><div className="library-results">
      {!error && loaded && recipes.length > 0 && shown.length === 0 && (
        <div className="empty-state">
          <p>今の絞り込みに合うレシピが見つかりませんでした</p>
          <button className="chip" onClick={clearFilters}>
            検索をクリア
          </button>
        </div>
      )}

      {!error && loaded && recipes.length === 0 && <div className="empty-state"><p>最初のレシピを追加しましょう</p><button className="chip" onClick={() => navigate({page:"import",url:"",fromSearch:false})}>URL・本文から取り込む</button></div>}
      <ul className="recipe-grid">
        {shown.map((r) => {
          const idx = selected.indexOf(r.file);
          return (
            <li key={r.file}>
              <article className={`recipe-card ${idx >= 0 ? "recipe-card-on" : ""}`}>
              <button className="recipe-open" aria-label={`${r.title}を開く`} onClick={() => setOpened(r.file)}>
                {r.images?.[0] ? (
                  <img src={`/${r.images[0]}`} alt="" loading="lazy" />
                ) : (
                  <div className="thumb-placeholder"><ChefHat aria-hidden="true"/><span>写真なし</span></div>
                )}
                {idx >= 0 && <span className="burner-badge">{idx + 1}</span>}
                <div className="card-body">
                  <div className="card-title">{r.title}</div>
                  <div className="card-meta">
                    {formatDuration(r.cookTime) && <span className="recipe-duration"><Clock aria-hidden="true"/>{formatDuration(r.cookTime)}</span>}
                  </div>
                  <PersonalRating rating={r.rating}/>

                </div>
              </button>
              <button disabled={starting} className={`recipe-select ${idx >= 0 ? "chip-on" : ""}`} aria-label={`${r.title}を調理に${idx >= 0 ? "選択済み" : "選ぶ"}`} aria-pressed={idx >= 0} onClick={() => toggle(r.file)}>{idx >= 0 ? "✓ 調理に選択済み" : "調理に選ぶ"}</button>
              </article>
            </li>
          );
        })}
      </ul>

      </div><CookingSelection selected={selected} recipes={recipes} starting={starting} cooking={cooking} onRemove={toggle} onClear={()=>setSelected([])} onStart={()=>onStart(selected)}/></div>
    </div>
  );
}
