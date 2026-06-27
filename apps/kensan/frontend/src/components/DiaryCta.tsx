import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { BookOpen } from "lucide-react";
import { api, ApiError, todayISO, dailyPath, dailySkeleton } from "../lib/api";
import { Card, CardBody } from "./ui/card";
import { Button } from "./ui/button";

const WEEKDAYS = ["日", "月", "火", "水", "木", "金", "土"];

// ダッシュボードから今日の日記を起票する（R6）。既にあれば開く、無ければ
// 規約の骨組みで作ってから開く。直近の日記ボードは廃止した。
export function DiaryCta() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const today = todayISO();
  const d = new Date();

  const open = useMutation({
    mutationFn: async () => {
      try {
        await api.daily(today); // 既にあれば 200
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) {
          await api.createFile(dailyPath(today), dailySkeleton(today));
        } else {
          throw e;
        }
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["daily", today] });
      navigate("/daily");
    },
  });

  return (
    <Card>
      <CardBody className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <p className="h-serif text-base font-semibold">
            {d.getMonth() + 1}月{d.getDate()}日（{WEEKDAYS[d.getDay()]}）の日記
          </p>
          <p className="text-xs text-muted-foreground mt-0.5">その日の出来事・感想・学びを書き留める。</p>
        </div>
        <Button variant="primary" loading={open.isPending} onClick={() => open.mutate()}>
          <BookOpen size={16} />
          今日の日記を書く
        </Button>
      </CardBody>
    </Card>
  );
}
