import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MarkdownView } from "./MarkdownView";

describe("MarkdownView", () => {
  it("renders headings, lists, and safe external links", () => {
    render(
      <MarkdownView
        content={`## 今日のニュース

- [公式発表](https://example.com/news)
`}
      />,
    );
    expect(screen.getByRole("heading", { name: "今日のニュース" })).not.toBeNull();
    const link = screen.getByRole("link", { name: "公式発表" });
    expect(link.getAttribute("href")).toBe("https://example.com/news");
    expect(link.getAttribute("target")).toBe("_blank");
    expect(link.getAttribute("rel")).toBe("noopener noreferrer");
  });

  it("does not render raw HTML, unsafe links, or remote images", () => {
    const { container } = render(
      <MarkdownView
        content={`<script>alert("x")</script>

[危険](javascript:alert(1))

![tracking](https://example.com/pixel.png)
`}
      />,
    );
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector('a[href^="javascript:"]')).toBeNull();
  });
});
