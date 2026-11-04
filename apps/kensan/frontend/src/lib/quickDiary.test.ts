import { expect, it } from "vitest";
import { prepareQuickLine, shiftDailyDate } from "./quickDiary";
import { readIntentions } from "./dailyIntentions";
it("日記の無い文書に追記しても予定へ混入させない",()=>{
 const before="---\ntype: daily\n---\n## 今日やりたいこと\n\n- 散歩する\n";
 const saved=prepareQuickLine(before,"  楽しかった\nありがとう  ","23:30");
 expect(readIntentions(saved.content)).toBe("- 散歩する");
 expect(saved.content).toContain("## 日記\n\n- 23:30 楽しかった ありがとう");
 expect(saved.content.startsWith(before)).toBe(true);
});
it("一行の編集は他の本文・予定・子見出しを保持する",()=>{
 const before="# 日付\n\n## 日記\n\n既存の本文\n\n### 完了タスク\n\n- 作業\n\n## 今日やりたいこと\n\n散歩\n";
 const first=prepareQuickLine(before,"ひと言","09:30");
 const edited=prepareQuickLine(first.content,"書き直した","09:30",first.entry);
 expect(edited.content).toBe(first.content.replace("- 09:30 ひと言","- 09:30 書き直した"));
 expect(edited.content.indexOf("書き直した")).toBeLessThan(edited.content.indexOf("### 完了タスク"));
 expect(readIntentions(edited.content)).toBe("散歩");
 expect(()=>prepareQuickLine(before,"別の内容","09:30","- 09:30 消えた一行")).toThrow();
});
it("コード中の見出しを日記と取り違えず、日付を月年またぎで進める",()=>{
 const before="```md\n## 日記\n```\n";
 expect(prepareQuickLine(before,"本当の日記","09:30").content).toContain("```\n\n## 日記\n");
 expect(shiftDailyDate("2026-12-31",1)).toBe("2027-01-01");
});
