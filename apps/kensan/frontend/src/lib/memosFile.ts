// memos.md の ## Scratch を空行区切りのブロック = メモカードとして扱う純粋関数群。
// FAB（FloatingMemoButton）とメモページ（MemoCards）が共有する。

export interface ParsedMemos {
  before: string; // ## Scratch より前（frontmatter + Pinned 等）
  blocks: string[]; // ## Scratch のブロック（空行区切り）
  after: string; // ## Scratch より後の見出し以降（通常は空）
  pinned: string[]; // ## Pinned のブロック（表示用）
}

function blocksOf(body: string): string[] {
  return body
    .split(/\n\s*\n/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function sectionBody(content: string, heading: string): string {
  const m = content.match(new RegExp(`^## ${heading}[^\\n]*$`, "m"));
  if (!m || m.index === undefined) return "";
  const rest = content.slice(m.index + m[0].length);
  const next = rest.search(/\n## /);
  return next === -1 ? rest : rest.slice(0, next);
}

export function parseMemos(content: string): ParsedMemos {
  const m = content.match(/^## Scratch[^\n]*$/m);
  if (!m || m.index === undefined) {
    return {
      before: content.replace(/\s*$/, ""),
      blocks: [],
      after: "",
      pinned: blocksOf(sectionBody(content, "Pinned")),
    };
  }
  const before = content.slice(0, m.index);
  const rest = content.slice(m.index + m[0].length);
  const next = rest.search(/\n## /);
  const body = next === -1 ? rest : rest.slice(0, next);
  const after = next === -1 ? "" : rest.slice(next).trim();
  return { before, blocks: blocksOf(body), after, pinned: blocksOf(sectionBody(content, "Pinned")) };
}

export function serializeMemos(p: ParsedMemos, blocks: string[]): string {
  const head = p.before.replace(/\s*$/, "");
  const body = blocks.join("\n\n");
  let out = `${head}\n\n## Scratch\n\n${body}${body ? "\n" : ""}`;
  if (p.after) out += `\n${p.after}\n`;
  return out;
}
