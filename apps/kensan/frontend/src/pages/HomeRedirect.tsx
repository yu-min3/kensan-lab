import { Navigate, useSearchParams } from "react-router-dom";

export function HomeRedirect({ panel }: { panel: string }) {
  const [params] = useSearchParams();
  const target = new URLSearchParams(params);
  if (panel === "projects" && target.has("name")) {
    target.set("project", target.get("name")!);
    target.delete("name");
  }
  if (panel !== "diary") target.set("panel", panel);
  return <Navigate replace to={`/?${target.toString()}#${panel}`} />;
}
