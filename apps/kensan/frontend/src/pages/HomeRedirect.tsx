import { Navigate, useSearchParams } from "react-router-dom";

// Retired management pages do not reopen hidden functionality.
export function HomeRedirect({ panel }: { panel: string }) {
  const [params] = useSearchParams();
  const target = new URLSearchParams();
  const date = params.get("date");
  if (date) target.set("date", date);
  const project = panel === "projects" ? params.get("name") ?? params.get("project") : null;
  const hash = project ? `#project-${encodeURIComponent(project)}` : panel === "diary" ? "#diary" : "";
  const search = target.size ? `?${target.toString()}` : "";
  return <Navigate replace to={`/${search}${hash}`} />;
}
