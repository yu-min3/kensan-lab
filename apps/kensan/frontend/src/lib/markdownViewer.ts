const WORKSPACE_ROOTS = new Set([
  "Attachments", "books", "daily", "docs", "inbox", "notes", "projects", "reviews",
]);

export function resolveMarkdownPath(currentPath: string, href: string): string | null {
  if (!href || href.startsWith("#") || /^(?:https?:|mailto:|tel:)/i.test(href)) return null;

  const [rawPath] = href.split(/[?#]/, 1);
  if (!rawPath.toLowerCase().endsWith(".md")) return null;

  const decoded = decodeURIComponent(rawPath).replace(/^\/+/, "");
  const first = decoded.split("/", 1)[0];
  const base = WORKSPACE_ROOTS.has(first)
    ? []
    : currentPath.split("/").slice(0, -1);

  const parts: string[] = [];
  for (const part of [...base, ...decoded.split("/")]) {
    if (!part || part === ".") continue;
    if (part === "..") {
      if (parts.length === 0) return null;
      parts.pop();
      continue;
    }
    parts.push(part);
  }
  return parts.join("/");
}

export function viewerURL(path: string): string {
  return `/view?path=${encodeURIComponent(path)}`;
}
