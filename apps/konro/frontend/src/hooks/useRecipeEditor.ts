import { useEffect, useState } from "react";
import { fetchRecipe, Recipe, RecipeEdit, RecipeWriteError, saveRecipe } from "../api";
import { durationMinutes, durationFromMinutes } from "../duration";

type Form = { title: string; servings: string; tags: string; prep: string; cook: string; source: string; rating: string; ingredients: string; steps: string; notes: string };
const fromRecipe = (r: Recipe | null): Form => ({ title: r?.title ?? "", servings: r?.servings ?? "", tags: (r?.tags ?? []).join("\n"), prep: durationMinutes(r?.prepTime), cook: durationMinutes(r?.cookTime), source: r?.source ?? "", rating: String(r?.rating ?? 0), ingredients: (r?.ingredients ?? []).join("\n"), steps: (r?.steps ?? []).join("\n"), notes: r?.notes ?? "" });
const lines = (s: string) => s.split("\n").map((v) => v.trim()).filter(Boolean);
export function useRecipeEditor(file: string | null, onSaved: (r:Recipe)=>void) {
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
      const input: RecipeEdit = { title: form.title.trim(), servings: form.servings, tags: lines(form.tags), prepTime: durationFromMinutes(form.prep, recipe?.prepTime), cookTime: durationFromMinutes(form.cook, recipe?.cookTime), source: form.source, rating: Number(form.rating), ingredients: lines(form.ingredients), steps: lines(form.steps), notes: form.notes.trim() };
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
 return { recipe, form, editing, setEditing, loading, saving, error, conflict, dirty, adopt, discard, change, save, loadLatest };
}
