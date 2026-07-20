import { useEffect, useMemo, useState } from "react";
import { fetchRecipes, formatDuration, RecipeMeta } from "../api";
import { normalizeQuery } from "../session";

// Recipe browser + session builder: tap cards to put recipes "on the stove",
// then start the cooking session.
export function RecipesPage({ onStart }: { onStart: (files: string[]) => void }) {
  const [recipes, setRecipes] = useState<RecipeMeta[]>([]);
  const [error, setError] = useState("");
  const [q, setQ] = useState("");
  const [tag, setTag] = useState("");
  const [selected, setSelected] = useState<string[]>([]);

  useEffect(() => {
    fetchRecipes().then(setRecipes).catch((e) => setError(String(e)));
  }, []);

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

  return (
    <div className="recipes-page">
      <header className="list-header">
        <h1>konro</h1>
        <input
          type="search"
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

      <ul className="recipe-grid">
        {shown.map((r) => {
          const idx = selected.indexOf(r.file);
          return (
            <li key={r.file}>
              <button
                className={`recipe-card ${idx >= 0 ? "recipe-card-on" : ""}`}
                onClick={() => toggle(r.file)}
              >
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
            </li>
          );
        })}
      </ul>

      {selected.length > 0 && (
        <div className="start-bar">
          <button className="start-button" onClick={() => onStart(selected)}>
            🔥 {selected.length} 品でコンロを開始
          </button>
        </div>
      )}
    </div>
  );
}
