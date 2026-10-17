import { useCallback, useEffect, useRef, useState } from "react";

const key = "konroRouteIndex";
function readRoute(): string | null {
 if (location.hash === "#new") return "";
 if (!location.hash.startsWith("#recipe/")) return null;
 try { return decodeURIComponent(location.hash.slice(8)); } catch { return null; }
}
const hashFor = (file: string | null) => file === null ? "" : file === "" ? "#new" : `#recipe/${encodeURIComponent(file)}`;

/** Recipe URLs survive reload; Back/Forward observes the editor's discard guard. */
export function useRecipeRoute() {
 const [opened,setOpened] = useState(readRoute);
 const guard = useRef<(()=>boolean)|null>(null);
 const current = useRef({ index: Number(history.state?.[key] ?? 0), file: opened });
 const restoring = useRef(false);
 const onGuard = useCallback((value:(()=>boolean)|null)=>{guard.current=value;},[]);
 useEffect(()=>{
  history.replaceState({...history.state,[key]:current.current.index},"",location.href);
  const pop = () => {
   if (restoring.current) { restoring.current=false; return; }
   const nextIndex=Number(history.state?.[key] ?? 0);
   if (guard.current && !guard.current()) {
    const delta=current.current.index-nextIndex;
    if (delta) { restoring.current=true; history.go(delta); }
    else { history.replaceState({...history.state,[key]:current.current.index},"",`${location.pathname}${location.search}${hashFor(current.current.file)}`); }
    return;
   }
   const file=readRoute(); current.current={index:nextIndex,file}; setOpened(file);
  };
  addEventListener("popstate",pop);
  return ()=>removeEventListener("popstate",pop);
 },[]);
 const navigate = useCallback((file:string|null, replace=false)=>{
  const index=current.current.index+(replace?0:1);
  history[replace?"replaceState":"pushState"]({...history.state,[key]:index},"",`${location.pathname}${location.search}${hashFor(file)}`);
  current.current={index,file}; setOpened(file);
 },[]);
 return {opened,navigate,onGuard};
}
