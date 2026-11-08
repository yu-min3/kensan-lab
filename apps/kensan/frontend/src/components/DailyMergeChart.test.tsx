import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { DailyMergeChart } from "./DailyMergeChart";
afterEach(() => { cleanup(); vi.useRealTimers(); });
it("JSTの最終取得日までの日別件数を示し、未取得と未到来を0にしない", () => {
  vi.useFakeTimers(); vi.setSystemTime(new Date("2026-10-31T01:00:00+09:00"));
  render(<DailyMergeChart days={[0,2,0,1,0,0,0]} week="2026-10-26" updatedAt="2026-10-29T23:00:00+09:00"/>);
  expect(screen.getByRole("img",{name:"10/26（月）: 0件"})).toBeTruthy();
  expect(screen.getByRole("img",{name:"10/27（火）: 2件"})).toBeTruthy();
  expect(screen.getByRole("img",{name:"10/30（金）: 未取得"})).toBeTruthy();
  expect(screen.getByRole("img",{name:"10/31（土）: 未取得"})).toBeTruthy();
  expect(screen.getByRole("img",{name:"11/01（日）: 未到来"})).toBeTruthy();
});
it("UTCで前日でもJSTの取得日までを表示し、全日0でも棒の値を壊さない", () => {
  vi.useFakeTimers(); vi.setSystemTime(new Date("2026-10-30T16:00:00Z"));
  render(<DailyMergeChart days={[0,0,0,0,0,0,0]} week="2026-10-26" updatedAt="2026-10-30T15:05:00Z"/>);
  expect(screen.getByRole("img",{name:"10/31（土）: 0件"})).toBeTruthy();
  expect(screen.getAllByRole("img").length).toBe(7);
});
