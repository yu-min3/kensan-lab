import { useEffect, useRef, useState } from "react";
import { draftRequest, ImportContent, ImportDraft } from "../api";

const storageKey = "konro.import-draft";
const reasons: Record<string, string> = {
  quota_wait: "利用枠が空くのを待っています。自動での再実行はしません。時間をおいて再確認してください。",
  auth_required: "AIのログイン確認が必要です。管理者が接続を確認してから再試行してください。",
  billing_blocked: "追加課金OFFを確認できないため停止しています。管理者が接続を確認するまで実行しません。",
  interrupted: "AIの接続先が処理中に再起動しました。再試行できます。",
  invalid_output: "AIの応答を読み取れませんでした。下書きはそのままです。もう一度試すか、手入力で編集できます。",
  execution_failed: "AI側で処理に失敗しました。下書きはそのままです。もう一度試すか、手入力で編集できます。",
  numbers_changed: "原文にない数値を検出したため、整形結果を採用していません。下書きを確認・編集できます。",
  invalid_recipe: "AIの応答がレシピの形式に合いませんでした。下書きはそのままです。再試行または手入力できます。",
};
const empty: ImportContent = { title: "", servings: "", ingredients: [], steps: [] };

export function ImportPage({ onClose, onSaved }: { onClose: () => void; onSaved: (file: string, title: string) => void }) {
  const [kind, setKind] = useState<"url" | "text">("url");
  const [input, setInput] = useState("");
  const [draft, setDraft] = useState<ImportDraft | null>(null);
  const [content, setContent] = useState<ImportContent>(empty);
  const [ingredients, setIngredients] = useState("");
  const [steps, setSteps] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [pollError, setPollError] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [resume, setResume] = useState(() => localStorage.getItem(storageKey) ?? "");
  const [loading, setLoading] = useState(!!resume);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const adopt = (d: ImportDraft) => {
    setPollError("");
    setDraft(d); setContent(d.content); setIngredients((d.content.ingredients ?? []).join("\n"));
    setSteps((d.content.steps ?? []).join("\n")); setConfirmed(false);
    localStorage.setItem(storageKey, d.id);
  };
  useEffect(() => {
    if (!resume) return;
    let live = true;
    draftRequest(`/${resume}`).then((d) => {
      if (!live) return;
      if (d.saved_file) { localStorage.removeItem(storageKey); setResume(""); }
      else adopt(d);
    }).catch((e) => { if (live) setError(String(e.message)); })
      .finally(() => { if (live) setLoading(false); });
    return () => { live = false; };
  }, [resume]);
  const active = !!draft && !draft.detached && (draft.pending || ["queued", "running", "waiting"].includes(draft.job?.state ?? ""));
  useEffect(() => {
    if (!draft || !active || busy) return;
    let live = true;
    let timer = 0;
    const poll = () => {
      draftRequest(`/${draft.id}`).then((d) => {
        if (!live) return;
        setPollError("");
        if (d.revision !== draft.revision) adopt(d); else setDraft(d);
      }).catch(() => { if (live) setPollError("接続できず、AIの状態を確認できません。通信が戻ると自動で確認します。"); })
        .finally(() => { if (live) timer = window.setTimeout(poll, delay); });
    };
    const delay = draft.job?.state === "waiting" ? 10000 : 2000;
    timer = window.setTimeout(poll, delay);
    return () => { live = false; window.clearTimeout(timer); };
  }, [draft, active, busy]);

  const perform = async (fn: () => Promise<void>) => {
    setBusy(true); setError("");
    try { await fn(); } catch (e) { if (mounted.current) setError(e instanceof Error ? e.message : String(e)); }
    finally { if (mounted.current) setBusy(false); }
  };
  const edited = (): ImportContent => ({ ...content, ingredients: ingredients.split("\n").map((v) => v.trim()).filter(Boolean), steps: steps.split("\n").map((v) => v.trim()).filter(Boolean) });
  const persist = async () => {
    if (!draft) throw new Error("下書きがありません");
    const d = await draftRequest(`/${draft.id}`, "PATCH", { revision: draft.revision, content: edited() });
    adopt(d); return d;
  };
  const start = async () => {
    setSubmitting(true);
    try {
      await perform(async () => {
        const d = await persist();
        try { adopt(await draftRequest(`/${d.id}/normalize`, "POST", { revision: d.revision, mode: "steps" })); }
        catch (e) {
          // A lost response can leave a submitted job: recover its persisted ID.
          try { adopt(await draftRequest(`/${d.id}`)); } catch { /* next reload retries the same key */ }
          throw e;
        }
      });
    } finally { setSubmitting(false); }
  };
  const action = (name: string) => perform(async () => {
    if (draft) adopt(await draftRequest(`/${draft.id}/${name}`, "POST", { revision: draft.revision }));
  });
  const discardView = () => { localStorage.removeItem(storageKey); setResume(""); setDraft(null); setInput(""); setError(""); };

  const state = submitting ? "sending" : draft?.detached ? "manual" : draft?.pending ? "sending" : draft?.job?.state ?? "draft";
  const failed = state === "failed";
  const titles: Record<string, string> = { sending: "AIに送信中", queued: "AIの順番待ち", running: "AIが抽出・整形中", waiting: "AIは待機中", succeeded: "AIの整形が完了", failed: "AIの抽出・整形に失敗しました", cancelled: "AIを取り消しました", manual: "手入力で編集中", draft: "下書きを作成しました" };
  const descriptions: Record<string, string> = { sending: "送信状況を確認しています。", queued: "まだ抽出は始まっていません。順番が来ると実行します。", running: "材料と手順を整えています。この画面を閉じても処理は続きます。", waiting: "現在は処理していません。", succeeded: "原文と照らし合わせて確認・編集してから保存してください。", failed: "処理は停止しています。自動では再試行しません。", cancelled: "処理は停止しました。下書きを編集できます。", manual: "手入力に切り替えました。AIの結果は取り込みません。", draft: "必要ならAIで材料と手順を抽出できます。" };
  const processing = ["sending", "queued", "running"].includes(state) && !pollError;
  return <main className="import-page">
    <header className="header-row"><h1 className="h-serif">レシピを取り込む</h1><button className="chip" disabled={busy} onClick={() => { if (draft && !active) perform(async () => { await persist(); onClose(); }); else onClose(); }}>一覧へ</button></header>
    {loading && <p role="status">前の下書きを読み込んでいます…</p>}
    {error && <p className="import-notice" role="alert">{error}</p>}
    {!draft && !loading && <section className="import-panel">
      <p>URLを貼るだけで材料と手順を取り込めます。取得できないサイトは、本文を貼り付けてください。</p>
      <div className="import-actions" role="group" aria-label="入力方法">
        <button className={`chip ${kind === "url" ? "chip-on" : ""}`} onClick={() => { setKind("url"); setInput(""); }}>WebサイトのURL</button>
        <button className={`chip ${kind === "text" ? "chip-on" : ""}`} onClick={() => { setKind("text"); setInput(""); }}>本文を貼り付ける</button>
      </div>
      <label>{kind === "url" ? "レシピのURL" : "レシピの本文"}
        {kind === "url" ? <input type="text" inputMode="url" autoCapitalize="none" autoCorrect="off" value={input} onChange={(e) => setInput(e.target.value)} placeholder="例: www.sirogohan.com/recipe/…" />
          : <textarea rows={10} value={input} onChange={(e) => setInput(e.target.value)} />}
      </label>
      <button className="start-button" disabled={busy || !input.trim()} onClick={() => perform(async () => adopt(await draftRequest("", "POST", kind === "url" ? { url: input.trim(), text: "" } : { url: "", text: input }))) }>{busy ? "取り込み中…" : "下書きを作る"}</button>
      {resume && <button className="chip" onClick={discardView}>別のレシピを取り込む</button>}
    </section>}
    {draft && <>
      <section className={`import-panel import-status ${failed ? "import-status-failed" : ""}`} aria-label="取り込み状態" aria-busy={processing}>
        <div role={failed ? "alert" : "status"} aria-live={failed ? "assertive" : "polite"}>
          <h2 className="import-status-title"><span className={processing ? "import-spinner" : "import-state-mark"} aria-hidden="true">{processing ? "" : failed ? "×" : state === "succeeded" ? "✓" : state === "waiting" ? "Ⅱ" : "·"}</span>{pollError ? "AIの状態を確認できません" : titles[state] ?? "AIの状態を確認しています"}</h2>
          <p>{pollError || descriptions[state]}</p>
          {!draft.detached && !submitting && !draft.pending && draft.job?.reason && <p>{reasons[draft.job.reason] ?? "処理を完了できませんでした。再試行または手入力できます。"}</p>}
        </div>
        {draft.warnings?.map((w, i) => <p className="import-notice" key={i}>{w}</p>)}
        {draft.agent_available ? <>
          <div className="import-actions">
            {failed && <button className="chip" disabled={busy} onClick={() => document.getElementById("import-editor")?.scrollIntoView({ behavior: "smooth" })}>手入力で編集する</button>}
            {!active && <button className="chip" disabled={busy} onClick={start}>{failed || state === "cancelled" ? "AIでもう一度試す" : "AIで抽出・整形"}</button>}
            {active && draft.job?.state === "waiting" && <button className="chip" disabled={busy} onClick={() => action("retry")}>利用枠を再確認</button>}
            {active && <><button className="chip" disabled={busy} onClick={() => action("cancel")}>AIを取り消す</button>
              <button className="chip" disabled={busy} onClick={() => action("manual")}>AIの結果を使わず編集する</button></>}
          </div>
        </> : <p>AIは未接続です。材料と手順を入力して保存できます。</p>}
      </section>
      <details className="import-panel"><summary>取り込み元を確認</summary>
        {draft.source.url && <p><a href={draft.source.url} target="_blank" rel="noreferrer">元のページを開く</a></p>}
        <pre>{draft.source.text || draft.source.raw}</pre>
        {draft.source.url && <details><summary>取得したHTML</summary><pre>{draft.source.raw}</pre></details>}
      </details>
      <section className="import-panel" id="import-editor">
        <fieldset disabled={busy || active}>
          <legend>レシピを確認・編集</legend>
          <label>タイトル<input value={content.title} maxLength={300} onChange={(e) => { setContent({ ...content, title: e.target.value }); setConfirmed(false); }} /></label>
          <label>何人分<input value={content.servings} onChange={(e) => { setContent({ ...content, servings: e.target.value }); setConfirmed(false); }} /></label>
          <label><span id="import-ingredients-label">材料（1行に1項目）</span><textarea aria-labelledby="import-ingredients-label" rows={6} value={ingredients} onChange={(e) => { setIngredients(e.target.value); setConfirmed(false); }} /></label>
          <label><span id="import-steps-label">手順（1行に1つの手順）</span><textarea aria-labelledby="import-steps-label" rows={8} value={steps} onChange={(e) => { setSteps(e.target.value); setConfirmed(false); }} /></label>
          <label className="import-confirm"><input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />分量・時間・温度と手順を確認しました</label>
          <div className="import-actions">
            <button className="chip" onClick={() => perform(async () => { await persist(); onClose(); })}>下書きを保存して戻る</button>
            <button className="start-button" disabled={!confirmed || !content.title.trim() || !ingredients.trim() || !steps.trim()} onClick={() => perform(async () => {
              const d = await persist(); const saved = await draftRequest(`/${d.id}/save`, "POST", { revision: d.revision });
              localStorage.removeItem(storageKey); onSaved(saved.saved_file!, saved.content.title);
            })}>レシピとして保存</button>
          </div>
        </fieldset>
      </section>
      {!active && <button className="chip" disabled={busy} onClick={discardView}>別のレシピを取り込む</button>}
    </>}
  </main>;
}
