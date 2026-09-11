import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { BookOpen } from "lucide-react";
import { api, ApiError, dailyPath, dailySkeleton } from "../lib/api";
import { Button } from "./ui/button";

// 今日画面のJST日付を受け取り、日記作成と遷移で同じ日付を使う。
export function DiaryButton({ date }: { date: string }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const open = useMutation({
    mutationFn: async () => {
      try {
        await api.daily(date);
      } catch (e) {
        if (!(e instanceof ApiError) || e.status !== 404) throw e;
        try {
          await api.createFile(dailyPath(date), dailySkeleton(date));
        } catch (createError) {
          // 別タブが先に作った場合は既存日記を開く。上書きはしない。
          if (!(createError instanceof ApiError) || createError.status !== 409)
            throw createError;
        }
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["file", dailyPath(date)] });
      qc.invalidateQueries({ queryKey: ["dailyList"] });
      navigate(`/daily?date=${date}`);
    },
  });
  return (
    <div className="ds-stack">
      <Button
        variant="primary"
        loading={open.isPending}
        onClick={() => open.mutate()}
      >
        <BookOpen size={16} />
        今日の日記を書く
      </Button>
      {open.isError && (
        <p role="alert" className="text-xs text-destructive">
          {open.error.message}
        </p>
      )}
    </div>
  );
}
