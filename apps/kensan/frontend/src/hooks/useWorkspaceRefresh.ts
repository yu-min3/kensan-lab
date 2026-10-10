import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";

// 同じファイルを読む画面は、一つの更新契約で揃える。
// 画面ごとに無効化対象を選ぶと、戻る操作でfreshな旧データが残る。
export function useWorkspaceRefresh() {
  const client = useQueryClient();
  return useCallback(
    () =>
      Promise.all(
        [
          "today",
          "portfolio",
          "board",
          "projects",
          "project",
          "project-metrics",
          "goals",
          "file",
        ].map((key) => client.invalidateQueries({ queryKey: [key] })),
      ),
    [client],
  );
}
