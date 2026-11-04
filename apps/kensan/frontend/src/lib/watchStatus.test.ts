import { expect, it } from "vitest";
import type { WatchView } from "./api";
import { watchSchedule, watchStatus } from "./watchStatus";

const at = (value: string) => Date.parse(`${value}+09:00`);
const view = (status: string, checkedAt: string): WatchView => ({ snapshot: { hash: "x", projects: [] }, state: { status, checkedAt }, result: null, stale: false });

it("夜間は翌朝まで遅延と扱わず、予定から15分を過ぎた未確認を検出する", () => {
 const v = view("unchanged", "2026-11-04T22:00:00+09:00");
 expect(watchSchedule(at("2026-11-04T22:01:00")).next).toBe(at("2026-11-05T08:00:00"));
 expect(watchStatus(v, at("2026-11-05T07:50:00")).warning).toBe(false);
 expect(watchStatus(v, at("2026-11-05T08:14:59")).overdue).toBe(false);
 expect(watchStatus(v, at("2026-11-05T08:15:00")).label).toBe("定期確認が遅れています");
});
it("年末年始もJSTの8時を次回予定とする", () => {
 expect(watchSchedule(at("2026-12-31T23:59:59")).next).toBe(at("2027-01-01T08:00:00"));
});
it("確認が成功していてもwaitingを正常にせず、認証と利用枠の対応を区別する", () => {
 const v = view("waiting", "2026-11-05T10:00:00+09:00");
 v.state.reasonCode = "billing_blocked";
 const s = watchStatus(v, at("2026-11-05T10:01:00"));
 expect(s.label).toBe("AI更新は要再開");expect(s.warning).toBe(true);expect(s.overdue).toBe(false);
 expect(s.help).toContain("認証・利用条件");expect(s.help).not.toContain("残高不足");
 v.state.reasonCode = "quota_wait";expect(watchStatus(v, at("2026-11-05T10:01:00")).label).toBe("利用枠の回復待ち");
});
it("差分なしの古い生成日時を故障扱いせず、更新のある結果と通信失敗を分ける", () => {
 const v = view("unchanged", "2026-11-05T10:00:00+09:00");
 v.result={hash:"x",generatedAt:"2026-10-01T08:00:00+09:00",items:[],sources:[]};
 expect(watchStatus(v, at("2026-11-05T10:01:00")).label).toBe("新しい記録なし");
 v.stale=true;expect(watchStatus(v, at("2026-11-05T10:01:00")).label).toBe("新しい記録あり");
 expect(watchStatus(v, at("2026-11-05T10:01:00"),true).label).toBe("取得を確認できません");
});
