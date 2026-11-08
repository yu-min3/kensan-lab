import { unified } from "unified";
import remarkParse from "remark-parse";

type Node = { type: string; value?: string; children?: Node[] };
const parser = unified().use(remarkParse);
const metadata = /^(?:出典|URL|更新日時|受信日時|ポイント)\s*[:：]?/;
function text(node: Node): string {
  if (node.type === "image" || node.type === "html") return "";
  return node.value ?? node.children?.map(text).join(node.type === "paragraph" ? "" : " ") ?? "";
}
// A source excerpt, not a generated claim. Full Markdown stays behind a disclosure/link.
export function watchExcerpt(markdown: string, limit = 130): string {
  const root = parser.parse(markdown) as Node;
  const paragraphs: string[] = [];
  function visit(node: Node) {
    if (node.type === "paragraph") {
      const value = text(node).replace(/\s+/g, " ").trim();
      if (value && !metadata.test(value)) paragraphs.push(value.replace(/^結論\s*[:：]\s*/, ""));
    } else node.children?.forEach(visit);
  }
  visit(root);
  const value = paragraphs[0] ?? "";
  const chars = Array.from(value);
  return chars.length > limit ? chars.slice(0, limit).join("") + "…" : value;
}

export function watchHeadline(markdown: string): string {
  const strong = markdown.match(/\*\*([^*\n]+)\*\*/)?.[1];
  const first = (strong || watchExcerpt(markdown, 500)).split(/[。\n]/)[0].replace(/\s*[（(](?:kensan-lab|PR|commit|#[0-9]).*$/, "").trim();
  return Array.from(first).length > 44 ? Array.from(first).slice(0,44).join("") + "…" : first;
}
