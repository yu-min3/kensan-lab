import { describe, expect, it } from "vitest";
import { daysUntil, projectGroups, ranked } from "./model";
import type { Task, TodayView } from "../../lib/api";

describe("today selectors", () => {
  it("ranks deadlines before priorities, treating missing priorities last", () => {
    const task = (display: string, priority?: number, due?: string) => ({ display, priority, due }) as Task;
    const input = [task("none"), task("priority", 1), task("due", 99, "2026-09-09")];
    expect(ranked(input).map(t => t.display)).toEqual(["due", "priority", "none"]);
    expect(input[0].display).toBe("none");
  });
  it("uses explicit focus IDs; does not infer North Star membership from prose", () => {
    const view = { goals: { focus: [{ project: "lab", title: "lab" }, { title: "life を優先しない" }] }, board: { today: [] }, routines: [], projects: [{ name: "lab", status: "active" }, { name: "life", status: "active" }, { name: "old", status: "paused" }] } as unknown as TodayView;
    expect(projectGroups(view).focus.map(p => p.name)).toEqual(["lab"]);
    expect(projectGroups(view).other.map(p => p.name)).toEqual(["life"]);
  });
  it("keeps an explicitly selected task visible even in a paused project", () => {
    const view = { goals: {}, board: { today: [{ project: "paused" }] }, routines: [], projects: [{ name: "paused", status: "paused" }] } as unknown as TodayView;
    expect(projectGroups(view).other).toHaveLength(1);
  });
  it("handles deadlines without a browser timezone shift", () => {
    expect(daysUntil("2026-09-09", "2026-09-08")).toBe(1);
    expect(daysUntil("2026-09-07", "2026-09-08")).toBe(-1);
    expect(daysUntil("", "2026-09-08")).toBeNull();
  });
});
