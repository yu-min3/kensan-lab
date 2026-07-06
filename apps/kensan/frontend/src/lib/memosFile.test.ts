import { describe, expect, it } from "vitest";
import { parseMemos, serializeMemos } from "./memosFile";

const FULL = `---
type: memo
tags: [memo]
---

## Pinned

大事なメモ

## Scratch

ブロック1

ブロック2
複数行の
つづき

## Archive

古いもの
`;

describe("parseMemos", () => {
  it("Scratch のブロックを空行区切りで抜き出す", () => {
    const p = parseMemos(FULL);
    expect(p.blocks).toEqual(["ブロック1", "ブロック2\n複数行の\nつづき"]);
    expect(p.pinned).toEqual(["大事なメモ"]);
    expect(p.before).toContain("## Pinned");
    expect(p.after).toBe("## Archive\n\n古いもの");
  });

  it("Scratch 見出しが無ければ全文を before に、blocks は空", () => {
    const p = parseMemos("---\ntype: memo\n---\n\n本文だけ\n");
    expect(p.blocks).toEqual([]);
    expect(p.before).toBe("---\ntype: memo\n---\n\n本文だけ");
    expect(p.after).toBe("");
  });

  it("空ファイルでも壊れない", () => {
    const p = parseMemos("");
    expect(p.blocks).toEqual([]);
    expect(p.pinned).toEqual([]);
  });
});

describe("serializeMemos", () => {
  it("parse → serialize のラウンドトリップで構造が保たれる", () => {
    const p = parseMemos(FULL);
    const out = serializeMemos(p, p.blocks);
    const p2 = parseMemos(out);
    expect(p2.blocks).toEqual(p.blocks);
    expect(p2.pinned).toEqual(p.pinned);
    expect(p2.after).toBe(p.after);
  });

  it("ブロックの追加が Scratch に反映される", () => {
    const p = parseMemos(FULL);
    const out = serializeMemos(p, ["新メモ", ...p.blocks]);
    expect(parseMemos(out).blocks).toEqual(["新メモ", "ブロック1", "ブロック2\n複数行の\nつづき"]);
  });

  it("全ブロック削除で Scratch が空になる", () => {
    const p = parseMemos(FULL);
    const out = serializeMemos(p, []);
    expect(parseMemos(out).blocks).toEqual([]);
    expect(out).toContain("## Scratch");
  });
});
