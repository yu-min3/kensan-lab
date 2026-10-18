import { useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { api, type TodayView } from "../../lib/api";
import { useWorkspaceRefresh } from "../../hooks/useWorkspaceRefresh";
import { Button } from "../ui/button";
import { Card, CardBody, CardHead } from "../ui/card";
import { useToast } from "../ui/toast";

// 日記はホームへ。今日画面には実績の入力を残す。
export function DiaryLine({ view }: { view: TodayView }) {
  const projects = view.projects.filter((p) => p.status === "active").map((p) => p.name);
  const focus = (view.goals.focus ?? []).map((f) => f.project).find((p) => p && projects.includes(p));
  const [text, setText] = useState("");
  const [project, setProject] = useState(focus ?? projects[0] ?? "");
  const refresh = useWorkspaceRefresh();
  const toast = useToast();
  const save = useMutation({
    mutationFn: (entry: { project: string; text: string }) => api.recordDone(entry.project, entry.text),
    onSuccess: (_, entry) => {
      setText("");
      toast({ title: "やったことを残しました", desc: entry.project || "todo.md" });
    },
    onSettled: refresh,
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    const line = text.trim();
    if (line && !save.isPending) save.mutate({ project, text: line });
  };
  const { planned, unplanned } = view.doneToday;
  return (
    <Card>
      <CardHead title="やったこと" />
      <CardBody className="ds-stack">
        <form onSubmit={submit} className="flex flex-col sm:flex-row gap-2">
          <label htmlFor="done-project" className="sr-only">
            プロジェクト
          </label>
          <select
            id="done-project"
            disabled={save.isPending}
            value={project}
            onChange={(e) => setProject(e.target.value)}
            className="rounded-md border border-border bg-card px-2 h-10 text-sm sm:max-w-[12rem]"
          >
            {projects.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
            <option value="">プロジェクト外</option>
          </select>
          <label htmlFor="done-line" className="sr-only">
            やったこと（1 行）
          </label>
          <input
            id="done-line"
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={200}
            disabled={save.isPending}
            enterKeyHint="send"
            autoComplete="off"
            placeholder="予定になかったことも、そのまま"
            className="min-w-0 flex-1 rounded-md border border-border bg-card px-3 h-10 min-h-[2.5rem] text-base focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
          <Button type="submit" variant="primary" loading={save.isPending} disabled={!text.trim()}>
            残す
          </Button>
        </form>
        {save.isError && (
          <p role="alert" className="text-xs text-destructive">
            {save.error.message}
          </p>
        )}
        <p className="text-xs text-muted-foreground" data-testid="done-split">
          今日の実績<span className="font-mono tnum"> {planned + unplanned} </span>件
        </p>
      </CardBody>
    </Card>
  );
}
