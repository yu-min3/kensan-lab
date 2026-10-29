import { unified } from "unified";
import remarkParse from "remark-parse";

const title = "今日やりたいこと";

function section(content: string) {
  const prefix = content.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/)?.[0] ?? "";
  const body = content.slice(prefix.length);
  const nodes = unified().use(remarkParse).parse(body).children;
  const index = nodes.findIndex(node => node.type === "heading" && node.depth === 2 &&
    node.children.length === 1 && node.children[0].type === "text" && node.children[0].value === title);
  if (index < 0) return null;
  const start = nodes[index].position!.end.offset! + prefix.length;
  const next = nodes.slice(index + 1).find(node => node.type === "heading" && node.depth <= 2);
  return { start, end: next ? next.position!.start.offset! + prefix.length : content.length };
}

export function readIntentions(content: string): string {
  const span = section(content);
  return span ? content.slice(span.start, span.end).trim() : "";
}

// Keep the frontmatter, free-form diary and all other sections byte-for-byte.
export function writeIntentions(content: string, value: string): string {
  const span = section(content);
  const newline = content.includes("\r\n") ? "\r\n" : "\n";
  const body = value.trim().replace(/\r?\n/g, newline);
  const replacement = `${newline}${newline}${body}${newline}${newline}`;
  const updated = !span ? `${content}${content.endsWith(newline) ? newline : newline + newline}## ${title}${replacement}`
    : content.slice(0, span.start) + replacement + content.slice(span.end);
  if (readIntentions(updated) !== body) throw new Error("今日やりたいことの欄を確認してください。大見出しはこの欄では使えません。");
  return updated;
}
