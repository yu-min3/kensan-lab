import { useState } from "react";
import { RecipesPage } from "./pages/RecipesPage";
import { SessionPage } from "./pages/SessionPage";
import { loadSession, newSession, saveSession, Session } from "./session";
import { primeAudio } from "./alerts";

export function App() {
  const [session, setSession] = useState<Session | null>(() => loadSession());

  const start = (files: string[]) => {
    primeAudio(); // user gesture: unlock audio for timer alarms
    const s = newSession(files);
    saveSession(s);
    setSession(s);
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
    <RecipesPage onStart={start} />
  );
}
