import { useEffect, useRef, useState } from "react";
import { RecipesPage } from "./pages/RecipesPage";
import { SessionPage } from "./pages/SessionPage";
import { loadSession, newSession, saveSession, Session } from "./session";
import { fetchRecipe } from "./api";
import { primeAudio } from "./alerts";

export function App() {
  const [session, setSession] = useState<Session | null>(() => loadSession());

  const [startError, setStartError] = useState("");
  const [starting, setStarting] = useState(false);
  const startLock = useRef(false);

  // Restored-session path never went through the "start" tap, so audio is
  // still locked; prime on the first interaction or timer alarms stay silent.
  useEffect(() => {
    window.addEventListener("pointerdown", primeAudio, { once: true });
    return () => window.removeEventListener("pointerdown", primeAudio);
  }, []);

  const start = async (files: string[]) => {
    if (startLock.current) return;
    startLock.current = true; setStarting(true); setStartError("");
    primeAudio();
    try {
      const recipes = await Promise.all(files.map(fetchRecipe));
      const s = newSession(files, Object.fromEntries(recipes.map((r) => [r.file, r])));
      saveSession(s);
      setSession(s);
    } catch (e) { setStartError(`調理を開始できませんでした。${e instanceof Error ? e.message : String(e)}`); }
    finally { startLock.current = false; setStarting(false); }
  };

  const update = (s: Session) => {
    saveSession(s);
    setSession(s);
  };

  const end = () => {
    saveSession(null);
    setSession(null);
  };

  return session ? (
    <SessionPage session={session} onUpdate={update} onEnd={end} />
  ) : (
    <><div aria-live="polite">{starting && <p className="notice" role="status">調理用のレシピを準備しています…</p>}{startError && <p className="notice" role="alert">{startError}</p>}</div><RecipesPage onStart={start} starting={starting} /></>
  );
}
