import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { FeedSections, splitFeedItems, splitFeedSections } from "./FeedSections";

describe("splitFeedSections", () => {
  it("splits level-two sections and counts items", () => {
    const result = splitFeedSections(
      "## 要対応\n\n### A\n本文\n\n### B\n本文\n\n## 今日のニュース\n\n### C\n本文",
    );
    expect(result).toEqual([
      { title: "要対応", content: "### A\n本文\n\n### B\n本文", count: 2 },
      { title: "今日のニュース", content: "### C\n本文", count: 1 },
    ]);
  });

  it("keeps unstructured content readable", () => {
    expect(splitFeedSections("本文だけ")).toEqual([
      { title: "レポート", content: "本文だけ", count: 0 },
    ]);
  });
});

describe("splitFeedItems", () => {
  it("uses the source URL as a stable acknowledgement key", () => {
    const items = splitFeedItems(
      "### スカウト（未対応）\n本文\n\n- 更新日時: 2026-07-27T09:00:00+09:00\n- 出典: https://mail.google.com/mail/u/0/#inbox/thread-1\n\n### 別件\n本文",
    );
    expect(items[0]).toEqual({
      title: "スカウト（未対応）",
      content:
        "本文\n\n- 更新日時: 2026-07-27T09:00:00+09:00\n- 出典: https://mail.google.com/mail/u/0/#inbox/thread-1",
      key: "https://mail.google.com/mail/u/0/#inbox/thread-1",
      version: "2026-07-27T09:00:00+09:00",
    });
    expect(items[1].title).toBe("別件");
    expect(items[1].key).toBe("title:別件");
    expect(items[1].version).toMatch(/^content:[0-9a-f]{8}$/);
  });

  it("hides acknowledged inbox items and excludes them from the badge", () => {
    render(
      <FeedSections
        content={"## 要対応\n\n### 確認済み\n\n本文\n\n- 出典: https://example.com/thread/1"}
        acknowledgedVersions={
          new Map([
            [
              "https://example.com/thread/1",
              splitFeedItems(
                "### 確認済み\n\n本文\n\n- 出典: https://example.com/thread/1",
              )[0].version,
            ],
          ])
        }
      />,
    );
    expect(screen.getAllByText("0件").length).toBeGreaterThan(0);
    expect(screen.getByText("確認済み 1件")).toBeTruthy();
    expect(screen.queryByText("確認済み", { selector: "h5" })).toBeNull();
  });
});
