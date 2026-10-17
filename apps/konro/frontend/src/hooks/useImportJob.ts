import { useEffect, useRef, useState } from "react";
import { draftRequest, ImportContent, ImportDraft } from "../api";
const storageKey = "konro.import-draft";
export const clearImportDraft = () => localStorage.removeItem(storageKey);
const empty: ImportContent = { title: "", servings: "", ingredients: [], steps: [] };

export function useImportJob() {
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

 return {kind,setKind,input,setInput,draft,content,setContent,ingredients,setIngredients,steps,setSteps,submitting,pollError,confirmed,setConfirmed,busy,error,resume,loading,active,perform,persist,start,action,discardView,adopt};
}
