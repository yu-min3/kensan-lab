import { useEffect, useLayoutEffect, useRef } from "react";
import { ArrowLeft, Pencil, Plus, Check } from "lucide-react";
import { formatDuration, Recipe } from "../api";
import { useRecipeEditor } from "../hooks/useRecipeEditor";
import {RecipePhoto} from "../components/RecipePhoto";
import {useRecipePhoto} from "../hooks/useRecipePhoto";
import {SourceCredit} from "../components/SourceCredit";
import { Field, Panel } from "../components/Editor";

export function RecipePage({ file, selected, onBack, onToggle, onSaved, onGuard }: {
  file: string | null; selected: boolean; onBack: () => void; onToggle: (file: string) => void; onSaved: (recipe: Recipe) => void; onGuard?: (guard: (()=>boolean) | null)=>void;
}) {
  const { recipe, form, editing, setEditing, loading, saving, error, conflict, dirty, adopt, discard, change, save, loadLatest } = useRecipeEditor(file,onSaved);
  const photo=useRecipePhoto(recipe,r=>{adopt(r);onSaved(r)});
  useEffect(() => { onGuard?.(() => !saving && !photo.busy && !photo.preparing && discard() && photo.discard()); return () => onGuard?.(null); }, [onGuard, dirty, saving, photo.dirty, photo.busy, photo.preparing]);
  const editButton = useRef<HTMLButtonElement>(null);
  const formElement = useRef<HTMLFormElement>(null);
  const readPosition = useRef(0);
  const wasEditing = useRef(editing);
  useLayoutEffect(() => {
    if (loading) return;
    if (editing) {
      window.scrollTo({ top: 0, behavior: "instant" });
      formElement.current?.querySelector<HTMLInputElement>("#recipe-title")?.focus({ preventScroll: true });
    } else if (wasEditing.current) {
      window.scrollTo({ top: readPosition.current, behavior: "instant" });
      editButton.current?.focus({ preventScroll: true });
    }
    wasEditing.current = editing;
  }, [editing, loading]);
  const beginEdit = () => { readPosition.current = window.scrollY; setEditing(true); };
  const cancelEdit = () => { if (!discard()) return; if (recipe) adopt(recipe); else onBack(); };
  const field = (name: keyof typeof form, label: string, options: {rows?:number;type?:string;max?:number} = {}) => <Field id={`recipe-${name}`} label={label} value={form[name]} onChange={(value)=>change(name,value)} rows={options.rows} type={options.type} maxLength={options.max} />;
  return <main className="recipe-detail editor-page">
    <header className="recipe-toolbar" aria-label={editing ? "レシピの編集操作" : "レシピの操作"}>
      {editing ? <>
        <button type="button" className="recipe-action recipe-action-ghost" disabled={saving} onClick={cancelEdit}>キャンセル</button>
        <h1 className="recipe-toolbar-title">{file ? "レシピを編集" : "レシピを作成"}</h1>
        <button type="submit" form="recipe-edit-form" className="recipe-action recipe-action-primary" aria-label="レシピを保存" disabled={loading || saving || !form.title.trim()}>{saving ? "保存中…" : "保存"}</button>
      </> : <>
        <button type="button" className="recipe-action recipe-action-ghost" disabled={photo.busy||photo.preparing} onClick={()=>{if(photo.discard()){photo.cancel();onBack()}}}><ArrowLeft aria-hidden="true" />一覧へ</button>
        {!loading && recipe && <button type="button" className="recipe-action" ref={editButton} disabled={photo.dirty||photo.busy||photo.preparing} onClick={beginEdit}><Pencil aria-hidden="true" />編集</button>}
      </>}
    </header>
    {loading && <p role="status">レシピを読み込んでいます…</p>}
    {error && <div className="notice recipe-error" role="alert"><p>{error}</p>{conflict && <><p>入力は残しています。変更内容を控えてから最新版を確認してください。</p><button className="chip" disabled={saving} onClick={() => void loadLatest()}>{file ? "最新版を読み込む" : "作成済みのレシピを確認"}</button></>}</div>}
    {!loading && editing && <form id="recipe-edit-form" ref={formElement} className="editor-panel" onSubmit={(e) => { e.preventDefault(); void save(); }}>
      <fieldset disabled={saving}>
        <legend>{dirty ? "変更はまだ保存されていません" : "レシピの内容"}</legend>
        {field("title", "タイトル", { max: 300 })}
        {field("servings", "何人分", { max: 200 })}
        {field("ingredients", "材料（1行に1項目）", { rows: 7 })}
        {field("steps", "手順（1行に1つの手順）", { rows: 10 })}
        {field("notes", "メモ", { rows: 5 })}
        <details><summary>分類・時間・出典</summary>
          {field("tags", "タグ（1行に1つ）", { rows: 3 })}
          {field("prep", "下準備時間（分）", { type: "number" })}
          {field("cook", "調理時間（分）", { type: "number" })}
          {field("source", "出典（URL・書籍など）", { max: 2048 })}
          <label htmlFor="recipe-rating"><span id="recipe-rating-label">お気に入り度</span><select aria-labelledby="recipe-rating-label" id="recipe-rating" value={form.rating} onChange={(e) => change("rating", e.target.value)}>{[0, 1, 2, 3, 4, 5].map((n) => <option key={n} value={n}>{n ? "★".repeat(n) : "未設定"}</option>)}</select></label>
        </details>

      </fieldset>
    </form>}
    {!loading && !editing && recipe && <>
      <Panel>
        <h1 className="h-serif recipe-name">{recipe.title}</h1>
        <SourceCredit info={recipe.sourceInfo}/>
        <p className="recipe-summary">{[recipe.servings, recipe.prepTime && `下準備 ${formatDuration(recipe.prepTime)}`, recipe.cookTime && `調理 ${formatDuration(recipe.cookTime)}`, recipe.rating ? "自分の評価 " + "★".repeat(recipe.rating) : ""].filter(Boolean).join(" · ")}</p>
        {!!recipe.tags?.length && <div className="detail-tags">{recipe.tags.map((tag, i) => <span className="recipe-tag" key={i}>{tag}</span>)}</div>}
        <RecipePhoto image={recipe.images?.[0]} photo={photo}/>
        <button type="button" className={`recipe-action ${photo.dirty ? "" : "recipe-action-primary"} recipe-cook`} disabled={photo.busy||photo.preparing} aria-pressed={selected} onClick={() => onToggle(recipe.file)}>{selected ? <Check aria-hidden="true" /> : <Plus aria-hidden="true" />}{selected ? "追加済み・外す" : "調理に追加"}</button>
      </Panel>
      <Panel><h2 className="h-serif">材料</h2>{recipe.ingredients?.length ? <ul className="detail-ingredients">{recipe.ingredients.map((s, i) => <li key={i}>{s}</li>)}</ul> : <p className="recipe-summary">材料はまだ登録されていません</p>}</Panel>
      <Panel><h2 className="h-serif">手順</h2>{recipe.steps?.length ? <ol className="detail-steps">{recipe.steps.map((s, i) => <li key={i}>{s}</li>)}</ol> : <p className="recipe-summary">手順はまだ登録されていません</p>}</Panel>
      {recipe.notes && <Panel><h2 className="h-serif">メモ</h2><p className="detail-notes">{recipe.notes}</p></Panel>}
      {recipe.source && <Panel><h2 className="h-serif">出典</h2>{/^https?:\/\//i.test(recipe.source) ? <a href={recipe.source} target="_blank" rel="noreferrer">{recipe.source}</a> : <p>{recipe.source}</p>}</Panel>}
    </>}
  </main>;
}
