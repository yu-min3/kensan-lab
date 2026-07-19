import { useEffect, useMemo, useRef, useState } from "react";
import { fetchRecipe, formatDuration, Recipe } from "../api";
import { formatRemaining, Session, timerCandidates } from "../session";
import { alarm } from "../alerts";
import { holdWakeLock, releaseWakeLock } from "../wakelock";

// The cooking view: bottom burner tabs, one recipe panel, cross-tab timers.
export function SessionPage({
  session,
  onUpdate,
  onEnd,
}: {
  session: Session;
  onUpdate: (s: Session) => void;
  onEnd: () => void;
}) {
  const [recipes, setRecipes] = useState<Record<string, Recipe>>({});
  const [now, setNow] = useState(Date.now());
  const [confirmEnd, setConfirmEnd] = useState(false);
  const [showIngredients, setShowIngredients] = useState(true);
  const firedRef = useRef(new Set<string>());

  useEffect(() => {
    holdWakeLock();
    return releaseWakeLock;
  }, []);

  useEffect(() => {
    for (const f of session.files) {
      fetchRecipe(f)
        .then((r) => setRecipes((m) => ({ ...m, [f]: r })))
        .catch(() => {});
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.files.join("|")]);

  // clock tick: drives countdowns and fires alarms exactly once per timer
  useEffect(() => {
    const iv = setInterval(() => {
      const t = Date.now();
      setNow(t);
      for (const timer of session.timers) {
        if (!timer.acknowledged && timer.endsAt <= t && !firedRef.current.has(timer.id)) {
          firedRef.current.add(timer.id);
          alarm();
        }
      }
    }, 500);
    return () => clearInterval(iv);
  }, [session.timers]);

  const active = session.files[session.active];
  const recipe = recipes[active];

  const checkedSteps = session.steps[active] ?? [];
  const currentStep = useMemo(() => {
    const steps = recipe?.steps ?? [];
    for (let i = 0; i < steps.length; i++) if (!checkedSteps[i]) return i;
    return steps.length;
  }, [recipe, checkedSteps]);

  const toggleStep = (i: number) => {
    const steps = [...(session.steps[active] ?? [])];
    steps[i] = !steps[i];
    onUpdate({ ...session, steps: { ...session.steps, [active]: steps } });
  };

  const toggleIngredient = (i: number) => {
    const ing = [...(session.ingredients[active] ?? [])];
    ing[i] = !ing[i];
    onUpdate({ ...session, ingredients: { ...session.ingredients, [active]: ing } });
  };

  const addTimer = (label: string, seconds: number) => {
    onUpdate({
      ...session,
      timers: [
        ...session.timers,
        {
          id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          recipe: active,
          label,
          endsAt: Date.now() + seconds * 1000,
          acknowledged: false,
        },
      ],
    });
  };

  const dismissTimer = (id: string) =>
    onUpdate({ ...session, timers: session.timers.filter((t) => t.id !== id) });

  const liveTimers = session.timers.filter((t) => !t.acknowledged);

  return (
    <div className="session-page">
      <header className="session-header">
        {confirmEnd ? (
          <button className="end-button end-confirm" onClick={onEnd}>
            本当に終了する
          </button>
        ) : (
          <button className="end-button" onClick={() => { setConfirmEnd(true); setTimeout(() => setConfirmEnd(false), 3000); }}>
            終了
          </button>
        )}
        <div className="timer-bar">
          {liveTimers.map((t) => {
            const remaining = t.endsAt - now;
            const fired = remaining <= 0;
            return (
              <button
                key={t.id}
                className={`timer-chip ${fired ? "timer-fired" : ""}`}
                onClick={() => dismissTimer(t.id)}
                title={recipes[t.recipe]?.title}
              >
                {fired ? "🔔" : "⏱"} {t.label} {fired ? "完了!" : formatRemaining(remaining)}
              </button>
            );
          })}
        </div>
      </header>

      <main className="recipe-panel">
        {!recipe ? (
          <p className="loading">読み込み中…</p>
        ) : (
          <>
            <h2 className="recipe-title">{recipe.title}</h2>
            <div className="recipe-info">
              {recipe.servings && <span>{recipe.servings}</span>}
              {formatDuration(recipe.cookTime) && <span>⏱ {formatDuration(recipe.cookTime)}</span>}
              {recipe.source && (
                <a href={recipe.source} target="_blank" rel="noreferrer">
                  出典
                </a>
              )}
            </div>

            <section>
              <button className="section-toggle" onClick={() => setShowIngredients(!showIngredients)}>
                材料 {showIngredients ? "▾" : "▸"}
              </button>
              {showIngredients && (
                <ul className="ingredients">
                  {(recipe.ingredients ?? []).map((ing, i) => (
                    <li key={i}>
                      <label className={session.ingredients[active]?.[i] ? "done" : ""}>
                        <input
                          type="checkbox"
                          checked={session.ingredients[active]?.[i] ?? false}
                          onChange={() => toggleIngredient(i)}
                        />
                        {ing}
                      </label>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section>
              <h3 className="section-label">手順</h3>
              <ol className="steps">
                {(recipe.steps ?? []).map((step, i) => (
                  <li
                    key={i}
                    className={
                      checkedSteps[i] ? "step step-done" : i === currentStep ? "step step-current" : "step"
                    }
                  >
                    <button className="step-body" onClick={() => toggleStep(i)}>
                      <span className="step-num">{checkedSteps[i] ? "✓" : i + 1}</span>
                      <span className="step-text">{step}</span>
                    </button>
                    {i === currentStep && timerCandidates(step).length > 0 && (
                      <div className="step-timers">
                        {timerCandidates(step).map((c) => (
                          <button key={c.label} className="chip" onClick={() => addTimer(c.label, c.seconds)}>
                            ⏱ {c.label}
                          </button>
                        ))}
                      </div>
                    )}
                  </li>
                ))}
              </ol>
              {recipe.notes && (
                <>
                  <h3 className="section-label">メモ</h3>
                  <p className="notes">{recipe.notes}</p>
                </>
              )}
            </section>
          </>
        )}
      </main>

      <nav className="burner-tabs">
        {session.files.map((f, i) => {
          const r = recipes[f];
          const steps = r?.steps ?? [];
          const done = (session.steps[f] ?? []).filter(Boolean).length;
          const timers = liveTimers.filter((t) => t.recipe === f);
          const nextFire = timers.length ? Math.min(...timers.map((t) => t.endsAt)) - now : null;
          return (
            <button
              key={f}
              className={`burner-tab ${i === session.active ? "burner-active" : ""}`}
              onClick={() => onUpdate({ ...session, active: i })}
            >
              <span className="burner-name">{r?.title ?? f}</span>
              <span className="burner-sub">
                {steps.length > 0 ? `${done}/${steps.length}` : "–"}
                {nextFire !== null && (
                  <em className={nextFire <= 0 ? "burner-timer burner-timer-fired" : "burner-timer"}>
                    {nextFire <= 0 ? "🔔" : ` ${formatRemaining(nextFire)}`}
                  </em>
                )}
              </span>
            </button>
          );
        })}
      </nav>
    </div>
  );
}
