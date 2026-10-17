import { useEffect, useMemo, useState } from "react";
import { fetchRecipes, formatDuration, importRecipeZip, RecipeMeta } from "../api";
import { RecipePage } from "./RecipePage";
import { ImportPage } from "./ImportPage";
import { useRecipeRoute } from "../hooks/useRecipeRoute";
import { normalizeQuery } from "../session";

// Recipe browser + session builder: tap cards to put recipes "on the stove",
// then start the cooking session.
export function RecipesPage({ onStart, starting = false }: { onStart: (files: string[]) => void; starting?: boolean }) {
  const [webImport, setWebImport] = useState(false);
  const { opened, navigate: setOpened, onGuard } = useRecipeRoute();
  const [recipes, setRecipes] = useState<RecipeMeta[]>([]);
  const [error, setError] = useState("");
  const [q, setQ] = useState("");
  const [tag, setTag] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [importing, setImporting] = useState(false);
  const [importMsg, setImportMsg] = useState("");

  useEffect(() => {
    fetchRecipes().then(setRecipes).catch((e) => setError(String(e)));
  }, []);

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

  const topTags = useMemo(() => {
    const count = new Map<string, number>();
    for (const r of recipes) for (const t of r.tags ?? []) count.set(t, (count.get(t) ?? 0) + 1);
    return [...count.entries()].sort((a, b) => b[1] - a[1]).slice(0, 12).map(([t]) => t);
  }, [recipes]);

  const shown = useMemo(() => {
    const nq = normalizeQuery(q);
    return recipes.filter(
      (r) =>
        (!tag || (r.tags ?? []).includes(tag)) &&
        (!nq ||
          normalizeQuery(r.title).includes(nq) ||
          (r.tags ?? []).some((t) => normalizeQuery(t).includes(nq))),
    );
  }, [recipes, q, tag]);

  const toggle = (file: string) =>
    setSelected((sel) => (sel.includes(file) ? sel.filter((f) => f !== file) : [...sel, file]));

  if (opened !== null) return <RecipePage onGuard={onGuard} key={opened} file={opened || null} selected={selected.includes(opened)} onBack={() => setOpened(null)} onToggle={toggle} onSaved={(r) => {
    setOpened(r.file, true); setImportMsg(`✓ ${r.title} を保存しました`);
    fetchRecipes().then(setRecipes).catch((e) => setError(String(e)));
  }} />;

  if (webImport) return <ImportPage onClose={() => setWebImport(false)} onSaved={(file, title) => {
    setWebImport(false); setImportMsg(`✓ ${title} を保存しました`);
    setQ(""); setTag(""); setSelected((s) => s.includes(file) ? s : [...s, file]);
    fetchRecipes().then(setRecipes).catch((e) => setError(String(e)));
  }} />;

  return (
    <div className="recipes-page">
      <header className="list-header">
        <div className="header-row">
          <h1 className="h-serif">konro</h1>
          <div className="editor-actions"><button className="chip chip-on" onClick={() => setWebImport(true)}>レシピを追加</button>
          <button className="chip" onClick={() => setOpened("")}>手入力で作成</button>
          <label className="chip import-chip">
            {importing ? "取込中…" : "ZIP取込"}
            <input
              type="file"
              accept=".zip,application/zip"
              hidden
              disabled={importing}
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) importZip(f);
                e.target.value = "";
              }}
            />
          </label></div>
        </div>
        {importMsg && (
          <p
            className={`import-msg ${importMsg.startsWith("✓") ? "" : "error"}`}
            onClick={() => setImportMsg("")}
          >
            {importMsg}
          </p>
        )}
        <input
          type="search"
          aria-label="レシピを検索"
          placeholder="レシピを検索"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <div className="tag-row">
          {topTags.map((t) => (
            <button
              key={t}
              className={`chip ${tag === t ? "chip-on" : ""}`}
              onClick={() => setTag(tag === t ? "" : t)}
            >
              {t}
            </button>
          ))}
        </div>
      </header>

      {error && <p className="error">{error}</p>}

      {!error && recipes.length > 0 && shown.length === 0 && (
        <div className="empty-state">
          <p>「{q || tag}」に合うレシピが見つかりませんでした</p>
          <button className="chip" onClick={() => { setQ(""); setTag(""); }}>
            検索をクリア
          </button>
        </div>
      )}

      {!error && recipes.length === 0 && <div className="empty-state"><p>最初のレシピを追加しましょう</p><button className="chip" onClick={() => setWebImport(true)}>URL・本文から取り込む</button></div>}
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
                  <div className="thumb-placeholder">🍳</div>
                )}
                {idx >= 0 && <span className="burner-badge">{idx + 1}</span>}
                <div className="card-body">
                  <div className="card-title">{r.title}</div>
                  <div className="card-meta">
                    {formatDuration(r.cookTime) && <span>⏱ {formatDuration(r.cookTime)}</span>}
                    {r.rating ? <span>{"★".repeat(r.rating)}</span> : null}
                  </div>
                </div>
              </button>
              <button className={`recipe-select ${idx >= 0 ? "chip-on" : ""}`} aria-label={`${r.title}を調理に${idx >= 0 ? "選択済み" : "選ぶ"}`} aria-pressed={idx >= 0} onClick={() => toggle(r.file)}>{idx >= 0 ? "✓ 調理に選択済み" : "調理に選ぶ"}</button>
              </article>
            </li>
          );
        })}
      </ul>

      {selected.length > 0 && (
        <div className="start-bar">
          <button className="start-button" disabled={starting} onClick={() => onStart(selected)}>
            🔥 {selected.length} 品でコンロを開始
          </button>
        </div>
      )}
    </div>
  );
}
