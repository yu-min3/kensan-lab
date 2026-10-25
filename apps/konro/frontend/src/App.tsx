import { useEffect, useRef, useState } from "react";
import { RecipesPage } from "./pages/RecipesPage";
import { SessionPage } from "./pages/SessionPage";
import { newSession } from "./session";
import {useCookingSession} from './hooks/useCookingSession';
import { fetchRecipe, MealPlan, planRequest, Recipe } from "./api";
import { primeAudio } from "./alerts";
import {useRecipeRoute} from './hooks/useRecipeRoute';
import {Flame} from 'lucide-react';

export function App() {
  const {session,update,replace,error} = useCookingSession();
  const routing=useRecipeRoute();
  useEffect(()=>{if(session&&!location.hash)routing.navigate({page:'cooking'},true);else if(!session&&routing.route.page==='cooking')routing.navigate({page:'list'},true);},[]);
  useEffect(()=>{if(!session&&routing.route.page==='cooking')routing.navigate({page:'list'},true);},[session,routing.route.page]);

  const [startError, setStartError] = useState("");
  const [starting, setStarting] = useState(false);
  const startLock = useRef(false);

  const start = async (files: string[]) => {
    if(session){routing.navigate({page:'cooking'});return;}
    if (startLock.current) return;
    startLock.current = true; setStarting(true); setStartError("");
    primeAudio();
    try {
      const recipes = await Promise.all(files.map(fetchRecipe));
      const s = newSession(files, Object.fromEntries(recipes.map((r) => [r.file, r])));
      if(replace(s))routing.navigate({page:'cooking'});
    } catch (e) { setStartError(`調理を開始できませんでした。${e instanceof Error ? e.message : String(e)}`); }
    finally { startLock.current = false; setStarting(false); }
  };

  const end = () => {if(replace(null))routing.navigate({page:'list'},true);};
  const startPlan=async(plan:MealPlan)=>{
    if(session){routing.navigate({page:'cooking'});return}
    if(startLock.current)return;startLock.current=true;setStarting(true);setStartError('');
    try{primeAudio();const recipes=await planRequest<Recipe[]>('/'+plan.id+'/cooking','POST',{revision:plan.revision});const s=newSession(recipes.map(r=>r.file),Object.fromEntries(recipes.map(r=>[r.file,r])));s.mealPlan={id:plan.id,revision:plan.revision};if(replace(s))routing.navigate({page:'cooking'})}catch(e){setStartError(e instanceof Error?e.message:String(e))}finally{startLock.current=false;setStarting(false)}
  };

  return session&&routing.route.page==='cooking' ? (
    <>{error&&<p className="notice" role="alert">{error}</p>}<SessionPage session={session} onUpdate={update} onEnd={end} onBrowse={()=>routing.navigate({page:'list'})} /></>
  ) : (
    <>{session&&<aside className="session-banner"><span><Flame aria-hidden="true"/>調理中 {session.files.length}品 · タイマー{session.timers.length}件</span><button className="recipe-action" onClick={()=>void routing.navigateChecked({page:'cooking'})}>調理に戻る</button></aside>}<div aria-live="polite">{starting && <p className="notice" role="status">調理用のレシピを準備しています…</p>}{(startError||error) && <p className="notice" role="alert">{startError||error}</p>}</div><RecipesPage onStart={start} starting={starting} routing={routing} cooking={!!session} onStartPlan={startPlan} /></>
  );
}
