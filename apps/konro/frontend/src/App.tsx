import { useRef, useState } from "react";
import { RecipesPage } from "./pages/RecipesPage";
import { SessionPage } from "./pages/SessionPage";
import { newSession } from "./session";
import {useCookingSession} from './hooks/useCookingSession';
import { fetchRecipe } from "./api";
import { primeAudio } from "./alerts";

export function App() {
  const {session,update,replace,error} = useCookingSession();

  const [startError, setStartError] = useState("");
  const [starting, setStarting] = useState(false);
  const startLock = useRef(false);

  const start = async (files: string[]) => {
    if (startLock.current) return;
    startLock.current = true; setStarting(true); setStartError("");
    primeAudio();
    try {
      const recipes = await Promise.all(files.map(fetchRecipe));
      const s = newSession(files, Object.fromEntries(recipes.map((r) => [r.file, r])));
      replace(s);
    } catch (e) { setStartError(`調理を開始できませんでした。${e instanceof Error ? e.message : String(e)}`); }
    finally { startLock.current = false; setStarting(false); }
  };

  const end = () => {replace(null);};

  return session ? (
    <>{error&&<p className="notice" role="alert">{error}</p>}<SessionPage session={session} onUpdate={update} onEnd={end} /></>
  ) : (
    <><div aria-live="polite">{starting && <p className="notice" role="status">調理用のレシピを準備しています…</p>}{(startError||error) && <p className="notice" role="alert">{startError||error}</p>}</div><RecipesPage onStart={start} starting={starting} /></>
  );
}
