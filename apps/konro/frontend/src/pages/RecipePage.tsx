import { useEffect, useState } from "react";
import { fetchRecipe, formatDuration, Recipe, RecipeEdit, RecipeWriteError, saveRecipe } from "../api";

type Form = { title: string; servings: string; tags: string; prep: string; cook: string; source: string; rating: string; ingredients: string; steps: string; notes: string };
const minutes = (value = "") => {
  const m = value.match(/^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$/);
  return m ? String(Number(m[1] || 0) * 60 + Number(m[2] || 0) + Number(m[3] || 0) / 60) : "";
};
const fromRecipe = (r: Recipe | null): Form => ({ title: r?.title ?? "", servings: r?.servings ?? "", tags: (r?.tags ?? []).join("\n"), prep: minutes(r?.prepTime), cook: minutes(r?.cookTime), source: r?.source ?? "", rating: String(r?.rating ?? 0), ingredients: (r?.ingredients ?? []).join("\n"), steps: (r?.steps ?? []).join("\n"), notes: r?.notes ?? "" });
const lines = (s: string) => s.split("\n").map((v) => v.trim()).filter(Boolean);
const duration = (value: string, old = "") => {
  if (value === minutes(old)) return old;
  if (!value.trim()) return "";
  const n = Number(value);
  if (!Number.isFinite(n) || n < 0 || n > 100000) throw new Error("時間は0〜100000分で入力してください");
  const seconds = Math.round(n * 60), mins = Math.floor(seconds / 60), rest = seconds % 60;
  return `PT${mins ? `${mins}M` : ""}${rest || !mins ? `${rest}S` : ""}`;
};
export function RecipePage({ file, selected, onBack, onToggle, onSaved }: {
  file: string | null; selected: boolean; onBack: () => void; onToggle: (file: string) => void; onSaved: (recipe: Recipe) => void;
}) {
  const [recipe, setRecipe] = useState<Recipe | null>(null);
  const [form, setForm] = useState<Form>(() => fromRecipe(null));
  const [baseline, setBaseline] = useState(() => JSON.stringify(fromRecipe(null)));
  const [editing, setEditing] = useState(!file);
  const [loading, setLoading] = useState(!!file);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [createID] = useState(() => crypto.randomUUID().replaceAll("-", ""));
  const [reload, setReload] = useState(0);
  const dirty = editing && JSON.stringify(form) !== baseline;
  const adopt = (r: Recipe) => { const f = fromRecipe(r); setRecipe(r); setForm(f); setBaseline(JSON.stringify(f)); setEditing(false); setError(""); setConflict(false); };
  useEffect(() => {
    if (!file) return;
    let live = true; setLoading(true);
    fetchRecipe(file).then((r) => { if (live) adopt(r); }).catch((e) => { if (live) setError(e.message); }).finally(() => { if (live) setLoading(false); });
    return () => { live = false; };
  }, [file, reload]);
  useEffect(() => {
    if (!dirty) return;
    const guard = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = ""; };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [dirty]);
  const discard = () => !dirty || window.confirm("保存していない変更を破棄しますか？");
  const change = (key: keyof Form, value: string) => setForm((f) => ({ ...f, [key]: value }));
  const save = async () => {
    setSaving(true); setError(""); setConflict(false);
    try {
      const input: RecipeEdit = { title: form.title.trim(), servings: form.servings, tags: lines(form.tags), prepTime: duration(form.prep, recipe?.prepTime), cookTime: duration(form.cook, recipe?.cookTime), source: form.source, rating: Number(form.rating), ingredients: lines(form.ingredients), steps: lines(form.steps), notes: form.notes.trim() };
      const r = await saveRecipe(file, recipe?.version ?? "", input, createID);
      adopt(r); onSaved(r);
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); setConflict(e instanceof RecipeWriteError && e.status === 409); }
    finally { setSaving(false); }
  };
  const loadLatest = async () => {
    if (!discard()) return;
    if (file) { setReload((n) => n + 1); return; }
    setLoading(true);
    try { const r = await fetchRecipe(`imported/${createID}`); adopt(r); onSaved(r); }
    catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  };
  const field = (name: keyof Form, label: string, options: { rows?: number; type?: string; max?: number } = {}) => <label htmlFor={`recipe-${name}`}><span id={`recipe-${name}-label`}>{label}</span>
    {options.rows ? <textarea id={`recipe-${name}`} aria-labelledby={`recipe-${name}-label`} rows={options.rows} value={form[name]} onChange={(e) => change(name, e.target.value)} />
      : <input id={`recipe-${name}`} type={options.type ?? "text"} min={options.type === "number" ? 0 : undefined} max={options.type === "number" ? 100000 : undefined} step={options.type === "number" ? "any" : undefined} maxLength={options.max} value={form[name]} onChange={(e) => change(name, e.target.value)} />}
  </label>;
  return <main className="recipe-detail import-page">
    <header className="header-row"><h1 className="h-serif">{editing ? file ? "レシピを編集" : "レシピを作成" : "レシピを見る"}</h1><button className="chip" disabled={saving} onClick={() => { if (discard()) onBack(); }}>一覧へ</button></header>
    {loading && <p role="status">レシピを読み込んでいます…</p>}
    {error && <div className="import-notice recipe-error" role="alert"><p>{error}</p>{conflict && <><p>入力は残しています。変更内容を控えてから最新版を確認してください。</p><button className="chip" disabled={saving} onClick={() => void loadLatest()}>{file ? "最新版を読み込む" : "作成済みのレシピを確認"}</button></>}</div>}
    {!loading && editing && <form className="import-panel" onSubmit={(e) => { e.preventDefault(); void save(); }}>
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
        <div className="import-actions"><button type="submit" className="start-button" disabled={!form.title.trim() || saving}>{saving ? "保存中…" : "レシピを保存"}</button>
          <button type="button" className="chip" onClick={() => { if (!discard()) return; if (recipe) adopt(recipe); else onBack(); }}>キャンセル</button></div>
      </fieldset>
    </form>}
    {!loading && !editing && recipe && <>
      {recipe.images?.[0] && <img className="recipe-cover" src={`/${recipe.images[0]}`} alt="" />}
      <section className="import-panel">
        <h2 className="h-serif">{recipe.title}</h2>
        <p className="recipe-summary">{[recipe.servings, recipe.prepTime && `下準備 ${formatDuration(recipe.prepTime)}`, recipe.cookTime && `調理 ${formatDuration(recipe.cookTime)}`, recipe.rating ? "★".repeat(recipe.rating) : ""].filter(Boolean).join(" · ")}</p>
        {!!recipe.tags?.length && <div className="detail-tags">{recipe.tags.map((tag, i) => <span className="chip" key={i}>{tag}</span>)}</div>}
        <div className="import-actions"><button className="chip" onClick={() => setEditing(true)}>編集する</button><button className={`chip ${selected ? "chip-on" : ""}`} aria-pressed={selected} onClick={() => onToggle(recipe.file)}>{selected ? "調理から外す" : "調理に追加"}</button></div>
      </section>
      <section className="import-panel"><h2 className="h-serif">材料</h2>{recipe.ingredients?.length ? <ul className="detail-ingredients">{recipe.ingredients.map((s, i) => <li key={i}>{s}</li>)}</ul> : <p className="recipe-summary">材料はまだ登録されていません</p>}</section>
      <section className="import-panel"><h2 className="h-serif">手順</h2>{recipe.steps?.length ? <ol className="detail-steps">{recipe.steps.map((s, i) => <li key={i}>{s}</li>)}</ol> : <p className="recipe-summary">手順はまだ登録されていません</p>}</section>
      {recipe.notes && <section className="import-panel"><h2 className="h-serif">メモ</h2><p className="detail-notes">{recipe.notes}</p></section>}
      {recipe.source && <section className="import-panel"><h2 className="h-serif">出典</h2>{/^https?:\/\//i.test(recipe.source) ? <a href={recipe.source} target="_blank" rel="noreferrer">{recipe.source}</a> : <p>{recipe.source}</p>}</section>}
    </>}
  </main>;
}
