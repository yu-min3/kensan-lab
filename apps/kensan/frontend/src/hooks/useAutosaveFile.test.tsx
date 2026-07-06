import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { splitFrontmatter, useAutosaveFile } from "./useAutosaveFile";
import { api, ApiError } from "../lib/api";

// api を丸ごと mock（fetch まで下りない）。ApiError は実物を使う。
vi.mock("../lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("../lib/api")>();
  return {
    ...mod,
    api: {
      file: vi.fn(),
      putFile: vi.fn(),
    },
  };
});

const mocked = api as unknown as {
  file: ReturnType<typeof vi.fn>;
  putFile: ReturnType<typeof vi.fn>;
};

const CONTENT = `---
type: daily
---

今日の本文
`;

function doc(mtime: string) {
  return { path: "daily/2026/07/07.md", size: 1, mtime, meta: {} };
}

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("splitFrontmatter", () => {
  it("frontmatter と本文を分離し、write で復元できる", () => {
    const { fm, body } = splitFrontmatter(CONTENT);
    expect(fm).toBe("---\ntype: daily\n---\n");
    expect(body).toBe("今日の本文\n");
  });

  it("frontmatter が無ければ全文が本文", () => {
    expect(splitFrontmatter("本文のみ")).toEqual({ fm: "", body: "本文のみ" });
  });
});

describe("useAutosaveFile", () => {
  beforeEach(() => {
    mocked.file.mockResolvedValue({ content: CONTENT, doc: doc("t1") });
    mocked.putFile.mockResolvedValue({ doc: doc("t2") });
  });
  afterEach(async () => {
    cleanup(); // unmount は仕様上 flush を発火しうる（409 テスト等）
    await new Promise((r) => setTimeout(r, 0)); // 迷子の mutate microtask を着地させてから
    vi.clearAllMocks();
  });

  it("読み込むと frontmatter を除いた本文が initialBody になり saved 状態", async () => {
    const { result } = renderHook(() => useAutosaveFile({ path: "daily/2026/07/07.md" }), { wrapper });
    await waitFor(() => expect(result.current.initialBody).toBe("今日の本文\n"));
    expect(result.current.saveState).toBe("saved");
  });

  it("onChange 後 debounce で保存され、frontmatter が前置されて PUT される", async () => {
    const { result } = renderHook(
      () => useAutosaveFile({ path: "daily/2026/07/07.md", debounceMs: 20 }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.initialBody).not.toBeNull());

    result.current.onChange("書き換えた本文\n");
    await waitFor(() => expect(mocked.putFile).toHaveBeenCalledTimes(1));

    const [path, full, baseMtime] = mocked.putFile.mock.calls[0];
    expect(path).toBe("daily/2026/07/07.md");
    expect(full).toBe("---\ntype: daily\n---\n書き換えた本文\n");
    expect(baseMtime).toBe("t1"); // 楽観ロックは読み込み時の mtime
    await waitFor(() => expect(result.current.saveState).toBe("saved"));
  });

  it("保存成功後は新しい mtime で次の保存を行う（楽観ロックの連鎖）", async () => {
    const { result } = renderHook(
      () => useAutosaveFile({ path: "daily/2026/07/07.md", debounceMs: 20 }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.initialBody).not.toBeNull());

    result.current.onChange("v1\n");
    await waitFor(() => expect(mocked.putFile).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(result.current.saveState).toBe("saved"));

    result.current.onChange("v2\n");
    await waitFor(() => expect(mocked.putFile).toHaveBeenCalledTimes(2));
    expect(mocked.putFile.mock.calls[1][2]).toBe("t2"); // 1 回目の応答の mtime
  });

  it("409 で conflict 状態になる（外部編集の顕在化）", async () => {
    mocked.putFile.mockRejectedValue(new ApiError(409, "conflict"));
    const { result } = renderHook(
      () => useAutosaveFile({ path: "daily/2026/07/07.md", debounceMs: 20 }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.initialBody).not.toBeNull());

    result.current.onChange("衝突する編集\n");
    await waitFor(() => expect(result.current.saveState).toBe("conflict"));
    expect(result.current.conflict).toBe(true);
  });

  it("unmount 時に未保存分が flush される（データ喪失防止）", async () => {
    const { result, unmount } = renderHook(
      () => useAutosaveFile({ path: "daily/2026/07/07.md", debounceMs: 100000 }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.initialBody).not.toBeNull());

    result.current.onChange("離脱直前の編集\n");
    await waitFor(() => expect(result.current.saveState).toBe("dirty")); // debounce が遠いので dirty のまま
    expect(mocked.putFile).not.toHaveBeenCalled();

    unmount(); // debounce タイマーは 100s 先 → flush だけが保存経路
    await waitFor(() => expect(mocked.putFile).toHaveBeenCalledTimes(1));
    expect(mocked.putFile.mock.calls.at(-1)?.[1]).toContain("離脱直前の編集");
  });

  it("変更が無ければ unmount しても保存しない", async () => {
    const { result, unmount } = renderHook(
      () => useAutosaveFile({ path: "daily/2026/07/07.md" }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.initialBody).not.toBeNull());
    unmount();
    expect(mocked.putFile).not.toHaveBeenCalled();
  });
});
