import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Routine, type TodayView } from "../../lib/api";
import { useWorkspaceRefresh } from "../../hooks/useWorkspaceRefresh";
import { DiaryLine } from "../today/DiaryLine";
import { ActivityHistory } from "../today/ActivityHistory";
import { Card, CardBody, CardHead } from "../ui/card";
import { ErrorState, SkeletonRows } from "../ui/states";

export function HomeActivity() {
  const view = useQuery({ queryKey: ["today"], queryFn: api.today, refetchInterval: 60_000 });
  const refresh = useWorkspaceRefresh();
  const client = useQueryClient();
  const [pending, setPending] = useState<{ id: string; done: boolean } | null>(null);
  const change = useMutation({
    mutationFn: ({ routine, date, done }: { routine: Routine; date: string; done: boolean }) => api.routineState(routine, date, done),
    onMutate: async ({ routine, done }) => {
      await client.cancelQueries({ queryKey: ["today"] });
      const previous = client.getQueryData<TodayView>(["today"]);
      client.setQueryData<TodayView>(["today"], current => current && ({ ...current,
        routines: current.routines.map(item => item.id === routine.id ? { ...item, doneToday: done } : item),
      }));
      return { previous };
    },
    onError: (_, __, context) => { if (context?.previous) client.setQueryData(["today"], context.previous); },
    onSettled: async () => { await refresh(); setPending(null); },
  });
  if (view.isPending) return <SkeletonRows rows={3} />;
  if (view.isError) return <ErrorState error={view.error} onRetry={() => view.refetch()} />;
  const data = view.data;
  return (
    <div className="ds-section">
      <DiaryLine view={data} />
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 items-start">
        <Card>
          <CardHead title="習慣" />
          <CardBody className="ds-stack">
            {data.routines.length === 0 && <p className="text-sm text-muted-foreground">プロジェクトにルーティンを登録すると、ここに表示されます。</p>}
            {data.routines.map(routine => (
              <label key={routine.id} className="ds-inline items-start text-sm cursor-pointer">
                <input type="checkbox" checked={pending?.id === routine.id ? pending.done : routine.doneToday} disabled={change.isPending} onChange={event => { const done = event.target.checked; setPending({ id: routine.id, done }); change.mutate({ routine, date: data.date, done }); }} className="mt-1 accent-brand" />
                <span className="min-w-0 flex-1 break-words">{routine.text}<small className="block text-muted-foreground">{routine.project} · {routine.schedule}</small></span>
              </label>
            ))}
            {change.isError && <p role="alert" className="text-sm text-destructive">{change.error.message}</p>}
          </CardBody>
        </Card>
        <Card><CardHead title="最近の記録" sub="完了したこと・習慣の実績" /><CardBody><ActivityHistory view={data} /></CardBody></Card>
      </div>
    </div>
  );
}
