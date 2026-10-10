import type { Task } from "../../lib/api";
import { Card, CardHead, CardBody } from "../ui/card";
import { Button } from "../ui/button";
import { taskKey } from "./model";

export function TriageCard({
  task,
  count,
  reviewed,
  busy,
  onChoose,
}: {
  task: Task | null;
  count: number;
  reviewed: boolean;
  busy: boolean;
  onChoose: (task: Task, action: "today" | "later" | "skip") => void;
}) {
  return (
    <Card>
      <CardHead
        title="溜まっているものを、ひとつだけ"
        sub={`${count}件 · 未確認を先に、確認したものは後ろへ`}
      />
      <CardBody className="ds-stack">
        {task ? (
          <>
            <p className="h-serif text-lg font-semibold">{task.display}</p>
            <p className="text-xs text-muted-foreground">
              {task.project} · いつか
            </p>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => onChoose(task, "today")}
              >
                今日やる
              </Button>
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => onChoose(task, "later")}
              >
                あとで
              </Button>
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => onChoose(task, "skip")}
              >
                捨てる
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              捨てた項目は下の「見送ったタスク」から戻せます。
            </p>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">
            {reviewed
              ? "今日の仕分けは済みました。また明日。"
              : "未整理のタスクはありません。"}
          </p>
        )}
      </CardBody>
    </Card>
  );
}

export function TaskRecovery({
  tasks,
  busy,
  onRestore,
}: {
  tasks: Task[];
  busy: boolean;
  onRestore: (task: Task) => void;
}) {
  if (tasks.length === 0) return null;
  return (
    <details className="text-sm">
      <summary className="cursor-pointer text-muted-foreground">
        見送ったタスク {tasks.length}件 · 復元
      </summary>
      <div className="ds-stack mt-3">
        {tasks.map((t) => (
          <div className="flex items-center gap-3" key={taskKey(t)}>
            <span className="flex-1">{t.display}</span>
            <Button
              variant="ghost"
              disabled={busy}
              onClick={() => onRestore(t)}
            >
              戻す
            </Button>
          </div>
        ))}
      </div>
    </details>
  );
}
