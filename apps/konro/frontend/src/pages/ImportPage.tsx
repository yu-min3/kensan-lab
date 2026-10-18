import { draftRequest } from "../api";
import { useImportJob, clearImportDraft } from "../hooks/useImportJob";
import {SourceCredit} from "../components/SourceCredit";
import { Field, Panel } from "../components/Editor";

const reasons: Record<string, string> = {
  quota_wait: "利用枠が空くのを待っています。自動での再実行はしません。時間をおいて再確認してください。",
  auth_required: "抽出サービスに接続できません。読み取れた内容は残しています。手入力するか、時間をおいて再試行してください。",
  billing_blocked: "抽出サービスを利用できないため停止しています。読み取れた内容を編集できます。",
  interrupted: "抽出サービスとの接続が中断しました。再試行できます。",
  invalid_output: "抽出結果を読み取れませんでした。下書きはそのままです。もう一度試すか、手入力で編集できます。",
  execution_failed: "材料と手順を読み取れませんでした。下書きはそのままです。もう一度試すか、手入力で編集できます。",
  numbers_changed: "原文にない数値を検出したため、整形結果を採用していません。下書きを確認・編集できます。",
  invalid_recipe: "抽出結果がレシピの形式に合いませんでした。下書きはそのままです。再試行または手入力できます。",
};
export function ImportPage({ onClose, onSaved, initialURL="" }: { initialURL?:string; onClose: () => void; onSaved: (file: string, title: string) => void }) {
  const {create,savedDrafts,restoreDraft,kind,setKind,input,setInput,draft,content,setContent,ingredients,setIngredients,steps,setSteps,submitting,pollError,confirmed,setConfirmed,busy,error,resume,loading,active,perform,persist,start,action,discardView} = useImportJob(initialURL);
  const complete = !!content.title.trim() && !!ingredients.trim() && !!steps.trim();
  const state = submitting ? "sending" : draft?.detached ? "manual" : draft?.pending ? "sending" : draft?.job?.state ?? "draft";
  const failed = state === "failed";
  const titles: Record<string, string> = { sending: "材料と手順を読み取っています", queued: "取り込みの順番を待っています", running: "材料と手順を読み取っています", waiting: "取り込みは待機中です", succeeded: complete ? "取り込めました" : "一部を取り込めませんでした", failed: "取り込みに失敗しました", cancelled: "取り込みを中止しました", manual: "内容を入力・編集できます", draft: complete ? "取り込めました" : "一部を取り込めませんでした" };
  const descriptions: Record<string, string> = { sending: "ページから読み取れなかった部分を、別の方法で抽出しています。", queued: "受付済みです。順番が来ると自動で読み取ります。", running: "この画面を閉じても処理は続きます。", waiting: "現在は処理していません。時間をおいて再開するか、手入力できます。", succeeded: complete ? "材料と手順を確認して保存してください。" : "元のページを確認し、足りない材料・手順を入力してください。", failed: "処理は停止しています。再試行するか、読み取れた内容を編集できます。", cancelled: "処理は停止しました。読み取れた内容は残しています。", manual: "後から届く抽出結果で上書きされることはありません。", draft: complete ? "材料と手順を確認して保存してください。" : "読み取れた内容は残しています。元のページを確認して入力できます。" };
  const processing = ["sending", "queued", "running"].includes(state) && !pollError;
  return <main className="editor-page">
    <header className="header-row"><h1 className="h-serif">レシピを取り込む</h1><button className="chip" disabled={busy || loading} onClick={() => { if (draft && !active) perform(async () => { await persist(); onClose(); }); else onClose(); }}>{initialURL ? "検索結果へ" : "一覧へ"}</button></header>
    {loading && <Panel><p role="status"><span className="import-spinner" aria-hidden="true"/>{initialURL ? "レシピを取り込んでいます…" : "前の下書きを読み込んでいます…"}</p><p>材料と手順を読み取っています。完了すると確認画面へ進みます。</p></Panel>}
    {error && <p className="notice" role="alert">{error}</p>}
    {!draft && !loading && <Panel>
      <p>URLを貼るだけで材料と手順を取り込めます。取得できないサイトは、本文を貼り付けてください。</p>
      <div className="editor-actions" role="group" aria-label="入力方法">
        <button className={`chip ${kind === "url" ? "chip-on" : ""}`} onClick={() => { setKind("url"); setInput(""); }}>WebサイトのURL</button>
        <button className={`chip ${kind === "text" ? "chip-on" : ""}`} onClick={() => { setKind("text"); setInput(""); }}>本文を貼り付ける</button>
      </div>
      <label>{kind === "url" ? "レシピのURL" : "レシピの本文"}
        {kind === "url" ? <input type="text" inputMode="url" autoCapitalize="none" autoCorrect="off" value={input} onChange={(e) => setInput(e.target.value)} placeholder="例: www.sirogohan.com/recipe/…" />
          : <textarea rows={10} value={input} onChange={(e) => setInput(e.target.value)} />}
      </label>
      <button className="start-button" disabled={busy || !input.trim()} onClick={create}>{busy ? "取り込み中…" : "取り込む"}</button>
      {!!savedDrafts.length&&<details><summary>保存した下書き（{savedDrafts.length}件）</summary><ul className="web-results">{savedDrafts.map(ref=><li key={ref.id}><button className="chip" disabled={busy} onClick={()=>restoreDraft(ref.id)}>{ref.title}</button>{ref.url&&<p className="recipe-summary">{ref.url}</p>}</li>)}</ul></details>}
      {resume && <button className="chip" onClick={discardView}>別のレシピを取り込む</button>}
    </Panel>}
    {draft && <>
      <Panel className={`import-status ${failed ? "import-status-failed" : ""}`} aria-label="取り込み状態" aria-busy={processing}>
        <div role={failed ? "alert" : "status"} aria-live={failed ? "assertive" : "polite"}>
          <h2 className="import-status-title"><span className={processing ? "import-spinner" : "import-state-mark"} aria-hidden="true">{processing ? "" : failed ? "×" : state === "succeeded" ? "✓" : state === "waiting" ? "Ⅱ" : "·"}</span>{pollError ? "取り込みの状態を確認できません" : titles[state] ?? "取り込み状況を確認しています"}</h2>
          <p>{pollError || descriptions[state]}</p>
          {!draft.detached && !submitting && !draft.pending && draft.job?.reason && <p>{reasons[draft.job.reason] ?? "処理を完了できませんでした。再試行または手入力できます。"}</p>}
        </div>
        {!!draft.warnings?.length && <details><summary>取り込みの補足</summary>{draft.warnings.map((w,i)=><p key={i}>{w}</p>)}</details>}
        {draft.agent_available ? <>
          <div className="editor-actions">
            {failed && <button className="chip" disabled={busy} onClick={() => document.getElementById("import-editor")?.scrollIntoView({ behavior: "smooth" })}>手入力で編集する</button>}
            {!active && !complete && <button className="chip" disabled={busy} onClick={start}>取り込みを再試行</button>}
            {active && draft.job?.state === "waiting" && <button className="chip" disabled={busy} onClick={() => action("retry")}>取り込みを再開</button>}
            {active && <><button className="chip" disabled={busy} onClick={() => action("cancel")}>取り込みを中止</button>
              <button className="chip" disabled={busy} onClick={() => action("manual")}>手入力に切り替える</button></>}
          </div>
        </> : !complete && <p>自動抽出を利用できません。読み取れた内容を編集して保存できます。</p>}
      </Panel>
      <details className="editor-panel"><summary>取り込み元を確認</summary>
        {draft.source.url && <p><a href={draft.source.url} target="_blank" rel="noreferrer">元のページを開く</a></p>}
        <pre>{draft.source.text || draft.source.raw}</pre>
        {draft.source.url && <details><summary>取得したHTML</summary><pre>{draft.source.raw}</pre></details>}
      </details>
      {!active && <Panel id="import-editor">
        <fieldset disabled={busy || active}>
          <legend>レシピを確認・編集</legend>
          <SourceCredit info={draft.source.info}/>
          <Field id="import-title" label="タイトル" value={content.title} maxLength={300} onChange={(title)=>{setContent({...content,title});setConfirmed(false);}} />
          <Field id="import-servings" label="何人分" value={content.servings} onChange={(servings)=>{setContent({...content,servings});setConfirmed(false);}} />
          <Field id="import-ingredients" label="材料（1行に1項目）" rows={6} value={ingredients} onChange={(value)=>{setIngredients(value);setConfirmed(false);}} />
          <Field id="import-steps" label="手順（1行に1つの手順）" rows={8} value={steps} onChange={(value)=>{setSteps(value);setConfirmed(false);}} />
          <label className="confirm-field"><input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />分量・時間・温度と手順を確認しました</label>
          <div className="editor-actions">
            <button className="chip" onClick={() => perform(async () => { await persist(); onClose(); })}>下書きを保存して戻る</button>
            <button className="start-button" disabled={!confirmed || !content.title.trim() || !ingredients.trim() || !steps.trim()} onClick={() => perform(async () => {
              const d = await persist(); const saved = await draftRequest(`/${d.id}/save`, "POST", { revision: d.revision });
              clearImportDraft(d.id); onSaved(saved.saved_file!, saved.content.title);
            })}>レシピとして保存</button>
          </div>
        </fieldset>
      </Panel>}
      {!active && <button className="chip" disabled={busy} onClick={discardView}>別のレシピを取り込む</button>}
    </>}
  </main>;
}
