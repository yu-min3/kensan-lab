import { describe, expect, it } from "vitest";
import { splitFeedSections } from "./FeedSections";

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
