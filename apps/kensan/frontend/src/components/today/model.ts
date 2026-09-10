import type { Task, TodayView } from "../../lib/api";

export function ranked(tasks: Task[]): Task[] {
  return [...tasks].sort((a, b) => {
    if (a.due !== b.due) return (a.due || "9999").localeCompare(b.due || "9999");
    return (a.priority || Number.MAX_SAFE_INTEGER) - (b.priority || Number.MAX_SAFE_INTEGER);
  });
}
export function projectGroups(view: TodayView) {
  const focus = new Set((view.goals.focus ?? []).map(f => f.project).filter(Boolean));
  const todayProjects = new Set((view.board.today ?? []).map(t => t.project));
  const visible = view.projects.filter(p => p.status === "active" || focus.has(p.name) || todayProjects.has(p.name) || view.routines.some(r => r.project === p.name));
  return { focus: visible.filter(p => focus.has(p.name)), other: visible.filter(p => !focus.has(p.name)) };
}
export const taskKey = (t: Task) => t.id || `${t.file}:${t.line}`;
export function daysUntil(deadline: string, today: string) {
  const result = (Date.parse(deadline) - Date.parse(today)) / 86400000;
  return Number.isFinite(result) ? Math.ceil(result) : null;
}
