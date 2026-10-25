import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { fetchRecipe, formatDuration, Recipe } from "../api";
import { formatRemaining, isIngredientGroup, isPinnedSession, pinLegacySession, Session, tabName, timerCandidates } from "../session";
import { holdWakeLock, releaseWakeLock } from "../wakelock";
import {Bell, Clock, X, Undo2, Check, ChevronDown, ChevronRight, AlertTriangle} from 'lucide-react';
import {Dialog} from '../components/Dialog';

// The cooking view: bottom burner tabs, one recipe panel, cross-tab timers.
export function SessionPage({
  session,
  onUpdate,
  onEnd,
  onBrowse,
}: {
  session: Session;
  onUpdate: (s: Session | ((previous:Session)=>Session)) => void;
  onEnd: () => void;
  onBrowse: () => void;
}) {
  const [loaded, setRecipes] = useState<Record<string, Recipe>>({});
  const pinned = isPinnedSession(session);
  const recipes = pinned ? session.recipes! : loaded;
  const [loadErrors, setLoadErrors] = useState<Record<string, string>>({});
  const [now, setNow] = useState(Date.now());
  const [confirmEnd, setConfirmEnd] = useState(false);
  const [timerDetail, setTimerDetail] = useState<string | null>(null);

  useEffect(() => {
    holdWakeLock();
    return releaseWakeLock;
  }, []);

  const loadRecipe = (f: string) => {
    setLoadErrors((e) => ({ ...e, [f]: "" }));
    fetchRecipe(f)
      .then((r) => setRecipes((m) => ({ ...m, [f]: r })))
      .catch((err) => setLoadErrors((e) => ({ ...e, [f]: String(err) })));
  };

  useEffect(() => {
    if (!pinned) session.files.forEach(loadRecipe);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.files.join("|"), pinned]);

  // clock tick: drives countdowns and fires alarms exactly once per timer
  useEffect(() => {
    const iv = setInterval(() => {
      const t = Date.now();
      setNow(t);
    }, 500);
    return () => clearInterval(iv);
  }, [session.timers]);

  const active = session.files[session.active];
  const recipe = recipes[active];
  const latest=useRef(session);latest.current=session;
  const showIngredients=session.views?.[active]?.ingredientsOpen??true;
  useLayoutEffect(()=>{
    if(!pinned)return;
    const previous=history.scrollRestoration;history.scrollRestoration='manual';
    let position=latest.current.views?.[active]?.scroll??0;
    window.scrollTo({top:position,behavior:'instant'});
    let timer=0;
    const save=()=>onUpdate(s=>({...s,views:{...s.views,[active]:{scroll:position,ingredientsOpen:s.views?.[active]?.ingredientsOpen??true}}}));
    const capture=()=>{position=window.scrollY;window.clearTimeout(timer);timer=window.setTimeout(save,150)};
    window.addEventListener('scroll',capture,{passive:true});
    return()=>{window.clearTimeout(timer);window.removeEventListener('scroll',capture);save();history.scrollRestoration=previous;};
  },[active,pinned,onUpdate]);

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

  const addTimer = (label: string, seconds: number, note: string) => {
    onUpdate({
      ...session,
      timers: [
        ...session.timers,
        {
          id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          recipe: active,
          label,
          note,
          endsAt: Date.now() + seconds * 1000,
          acknowledged: false,
        },
      ],
    });
  };

  const dismissTimer = (id: string) => {
    const timer = session.timers.find(t => t.id === id);
    if (!timer) return;
    onUpdate({...session, stoppedTimer:timer, timers:session.timers.filter(t => t.id !== id)});
    setTimerDetail(null);
  };
  const restoreTimer = () => {
    if (!session.stoppedTimer) return;
    onUpdate({...session, timers:[...session.timers, session.stoppedTimer], stoppedTimer:undefined});
  };

  const liveTimers = session.timers.filter((t) => !t.acknowledged);

  return (
    <div className="session-page">
      <header className="session-header">
        <button className="end-button" onClick={onBrowse}>一覧へ</button>
        <button className="end-button" onClick={() => setConfirmEnd(true)}>終了</button>
        <div className="timer-bar">
          {liveTimers.map((t) => {
            const remaining = t.endsAt - now;
            const fired = remaining <= 0;
            return (
              <button
                key={t.id}
                className={`timer-chip ${fired ? "timer-fired" : ""}`}
                onClick={() => setTimerDetail(t.id)}
                aria-label={`${recipes[t.recipe]?.title??t.recipe} ${t.label}のタイマー詳細`}
                title={recipes[t.recipe]?.title}
              >
                {fired ? <Bell aria-hidden="true"/> : <Clock aria-hidden="true"/>} {t.label} {fired ? "完了!" : formatRemaining(remaining)}
                {t.note && <span className="timer-note">{t.note}</span>}
              </button>
            );
          })}
        </div>
      </header>
      {session.stoppedTimer&&<div className="timer-undo" role="status"><span>直前のタイマーを停止・確認しました。</span><button className="chip" onClick={restoreTimer}><Undo2 aria-hidden="true"/>取り消す</button></div>}
      {timerDetail&&(()=>{
        const t=session.timers.find(t=>t.id===timerDetail);
        if(!t)return null;
        const fired=t.endsAt<=now;
        return <Dialog title="タイマー詳細" onDismiss={()=>setTimerDetail(null)}>
          <h3>{recipes[t.recipe]?.title??t.recipe}</h3><p>{t.note}</p><p className="timer-detail-time tnum">{fired?'完了':formatRemaining(t.endsAt-now)}</p>
          <div className="editor-actions"><button autoFocus className="recipe-action" onClick={()=>setTimerDetail(null)}><X aria-hidden="true"/>閉じる</button><button className="recipe-action" onClick={()=>dismissTimer(t.id)}>{fired?'確認済みにする':'停止'}</button></div>
        </Dialog>;
      })()}
      {confirmEnd&&<Dialog title="調理を終了しますか？" onDismiss={()=>setConfirmEnd(false)}><p>終了すると、この調理のチェックとタイマーを削除します。</p><div className="editor-actions"><button autoFocus className="recipe-action" onClick={()=>setConfirmEnd(false)}>調理を続ける</button><button className="recipe-action" onClick={onEnd}>終了する（進捗・タイマーは消えます）</button></div></Dialog>}

      <main className="recipe-panel">
        {!pinned ? <section className="editor-panel" aria-label="以前の調理セッションの確認">
          <h2 className="h-serif">調理中のレシピを確認してください</h2>
          <p>以前の版では開始時の内容を記録していません。最新の内容で再開すると、材料と手順のチェックをやり直します。動いているタイマーは引き継ぎます。</p>
          {session.files.map((f) => <p key={f}>{loaded[f]?.title ?? f}{loadErrors[f] && <><span role="alert"> {loadErrors[f]}</span><button className="chip" onClick={() => loadRecipe(f)}>再読み込み</button></>}</p>)}
          <button className="start-button" disabled={!session.files.every((f) => loaded[f])} onClick={() => onUpdate(pinLegacySession(session, loaded))}>最新の内容で再開する</button>
        </section> : !recipe && loadErrors[active] ? (
          <div className="load-error">
            <p>レシピを読み込めませんでした</p>
            <p className="muted-small">{loadErrors[active]}</p>
            <button className="chip" onClick={() => loadRecipe(active)}>
              再試行
            </button>
          </div>
        ) : !recipe ? (
          <p className="loading">読み込み中…</p>
        ) : (
          <>
            <h2 className="recipe-title h-serif">{recipe.title}</h2>
            <div className="recipe-info">
              {recipe.servings && <span>{recipe.servings}</span>}
              {formatDuration(recipe.cookTime) && <span><Clock aria-hidden="true"/> {formatDuration(recipe.cookTime)}</span>}
              {recipe.source && /^https?:\/\//i.test(recipe.source) && (
                <a href={recipe.source} target="_blank" rel="noreferrer">
                  出典
                </a>
              )}
            </div>

            <section>
              <button className="section-toggle" aria-expanded={showIngredients} onClick={() => onUpdate(s=>({...s,views:{...s.views,[active]:{scroll:window.scrollY,ingredientsOpen:!showIngredients}}}))}>
                材料 {showIngredients ? <ChevronDown aria-hidden="true"/> : <ChevronRight aria-hidden="true"/>}
              </button>
              {showIngredients && (
                <ul className="ingredients">
                  {(recipe.ingredients ?? []).map((ing, i) =>
                    isIngredientGroup(ing) ? (
                      <li key={i} className="ing-group">
                        {ing}
                      </li>
                    ) : (
                      <li key={i} className="ing-item">
                        <label className={session.ingredients[active]?.[i] ? "done" : ""}>
                          <input
                            type="checkbox"
                            checked={session.ingredients[active]?.[i] ?? false}
                            onChange={() => toggleIngredient(i)}
                          />
                          {ing}
                        </label>
                      </li>
                    ),
                  )}
                </ul>
              )}
            </section>

            <section>
              <h3 className="section-label">手順</h3>
              {currentStep<(recipe.steps?.length??0)&&<button className="chip next-step" onClick={()=>document.querySelector('.step-current')?.scrollIntoView({block:'center',behavior:'instant'})}>次の手順へ</button>}
              {(recipe.steps ?? []).length === 0 && (
                <p className="no-steps">取り込み元 に手順が未登録のレシピです（材料のみ）。出典リンクから手順を確認してください。</p>
              )}
              <ol className="steps">
                {(recipe.steps ?? []).map((step, i) => (
                  <li
                    key={i}
                    className={
                      checkedSteps[i] ? "step step-done" : i === currentStep ? "step step-current" : "step"
                    }
                  >
                    <button className="step-body" aria-pressed={checkedSteps[i]??false} onClick={() => toggleStep(i)}>
                      <span className="step-num">{checkedSteps[i] ? <Check aria-hidden="true"/> : i + 1}</span>
                      <span className="step-text">{step}</span>
                    </button>
                    {i === currentStep && timerCandidates(step).length > 0 && (
                      <div className="step-timers">
                        {timerCandidates(step).map((c) => (
                          <button
                            key={c.label}
                            className="chip"
                            onClick={() => addTimer(c.label, c.seconds, step.slice(0, 8))}
                          >
                            <Clock aria-hidden="true"/> {c.label}
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

      <nav className="burner-tabs" aria-label="調理する料理">
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
              aria-pressed={i===session.active}
              aria-label={`${r?.title??f} ${done}/${steps.length}手順完了`}
              onClick={() => onUpdate({ ...session, active: i })}
            >
              <span className="burner-name">{tabName(r?.title ?? f)}</span>
              <span className="burner-sub">
                {loadErrors[f] && !r ? <AlertTriangle aria-label="読み込み失敗"/> : steps.length > 0 ? `${done}/${steps.length}` : r ? "材料のみ" : "–"}
                {nextFire !== null && (
                  <em className={nextFire <= 0 ? "burner-timer burner-timer-fired" : "burner-timer"}>
                    {nextFire <= 0 ? <Bell aria-label="タイマー完了"/> : ` ${formatRemaining(nextFire)}`}
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
