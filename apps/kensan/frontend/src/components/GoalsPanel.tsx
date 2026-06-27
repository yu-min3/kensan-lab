import { useQuery } from "@tanstack/react-query";
import { Compass } from "lucide-react";
import { api } from "../lib/api";
import { Card, CardHead, CardBody } from "./ui/card";
import { ErrorState, Skeleton } from "./ui/states";

// North Star と今期のフォーカスを常時表示（R5）。実体は goals.md。読み取り専用。
export function GoalsPanel() {
  const goals = useQuery({ queryKey: ["goals"], queryFn: api.goals });

  return (
    <Card>
      <CardHead title="North Star" sub="goals.md · 今期のフォーカス" />
      <CardBody className="ds-section">
        {goals.isPending ? (
          <Skeleton className="h-28 w-full" />
        ) : goals.isError ? (
          <ErrorState error={goals.error} onRetry={() => goals.refetch()} />
        ) : (
          <>
            <p className="h-serif text-lg font-semibold leading-snug flex items-start gap-2">
              <Compass size={18} className="text-brand mt-1 shrink-0" />
              <span>{goals.data.northStar || "（goals.md の ## North Star を設定）"}</span>
            </p>
            {goals.data.focus && goals.data.focus.length > 0 && (
              <ul className="ds-stack !gap-2">
                {goals.data.focus.map((f, i) => (
                  <li key={i} className="flex items-baseline gap-2 text-sm">
                    <span className="font-mono tnum text-xs text-brand shrink-0">{i + 1}</span>
                    <span>
                      <span className="font-medium">{f.title}</span>
                      {f.detail && <span className="text-muted-foreground"> — {f.detail}</span>}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </CardBody>
    </Card>
  );
}
