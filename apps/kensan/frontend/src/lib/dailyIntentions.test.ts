import { expect, it } from "vitest";
import { readIntentions, writeIntentions } from "./dailyIntentions";

it("preserves frontmatter and private sections while changing intentions", () => {
  const before = "---\ntype: daily\n---\n# 2026-10-29\n\n秘密の前文\n\n## 今日やりたいこと\n\n- 買い物\n\n## 日記\n\n秘密の本文\n";
  const updated = writeIntentions(before, "- 散歩");
  expect(updated).toBe(before.replace("- 買い物", "- 散歩"));
  expect(readIntentions(updated)).toBe("- 散歩");
});
it("ignores headings inside fenced code and blockquotes", () => {
  for (const example of ["```md\n## 今日やりたいこと\n秘密\n```", "> ## 今日やりたいこと\n> 秘密"]) {
    expect(readIntentions(example)).toBe("");
  }
});
it("stops at setext headings and preserves nested headings and fenced code", () => {
  expect(readIntentions("## 今日やりたいこと\n\n散歩\n\n日記\n----\n秘密")).toBe("散歩");
  expect(readIntentions("## 今日やりたいこと\n### 朝\n散歩\n```\n## サンプル\n```\n## 日記\n秘密")).toContain("## サンプル");
});
it("adds an explicit section without modifying the existing diary", () => {
  const before = "# 今日\n\n自由な日記\n";
  expect(readIntentions(before)).toBe("");
  expect(writeIntentions(before, "散歩")).toBe(before + "\n## 今日やりたいこと\n\n散歩\n\n");
});
it("preserves CRLF and refuses section-breaking edits or an unclosed fence", () => {
  const before = "## 今日やりたいこと\r\n\r\n前\r\n\r\n## 日記\r\n秘密\r\n";
  expect(writeIntentions(before, "後")).toBe(before.replace("前", "後"));
  expect(() => writeIntentions(before, "## 新しい大見出し\n後")).toThrow();
  expect(() => writeIntentions("```\n未完のコード", "散歩")).toThrow();
});
