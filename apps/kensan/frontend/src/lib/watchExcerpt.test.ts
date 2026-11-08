import { expect, it } from "vitest";
import { watchExcerpt, watchHeadline } from "./watchExcerpt";
it("強調された見出しを取り、PR番号の注記と後続の文を外す",()=>{
 expect(watchHeadline("**kensanの常設画面を更新（PR #123）**。詳しい変更\n次の文")).toBe("kensanの常設画面を更新");
 expect(watchHeadline("最初の見出し。\n後続の説明")).toBe("最初の見出し");
});
it("Markdownを本文へ変換し空白を整え、出典だけの段落を抜粋しない",()=>{
 expect(watchExcerpt("出典: https://example.com\n\n結論: **確認した変更**  を\n記録した。\n\n別の段落")).toBe("確認した変更 を 記録した。");
});
