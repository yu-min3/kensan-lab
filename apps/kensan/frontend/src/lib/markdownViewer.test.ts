import { describe, expect, it } from "vitest";
import { resolveMarkdownPath, viewerURL } from "./markdownViewer";

describe("resolveMarkdownPath", () => {
  it("resolves sibling markdown", () => {
    expect(resolveMarkdownPath("projects/job-search/docs/index.md", "detail.md"))
      .toBe("projects/job-search/docs/detail.md");
  });

  it("resolves parent markdown", () => {
    expect(resolveMarkdownPath("projects/job-search/docs/index.md", "../README.md"))
      .toBe("projects/job-search/README.md");
  });

  it("keeps workspace-root paths at root", () => {
    expect(resolveMarkdownPath("projects/job-search/docs/index.md", "notes/2026/example.md"))
      .toBe("notes/2026/example.md");
  });

  it("rejects external, anchors, non-markdown, and root escape", () => {
    expect(resolveMarkdownPath("notes/a.md", "https://example.com/a.md")).toBeNull();
    expect(resolveMarkdownPath("notes/a.md", "#section")).toBeNull();
    expect(resolveMarkdownPath("notes/a.md", "image.png")).toBeNull();
    expect(resolveMarkdownPath("notes/a.md", "../../escape.md")).toBeNull();
  });
});

it("builds an encoded viewer URL", () => {
  expect(viewerURL("notes/日本語 file.md")).toBe("/view?path=notes%2F%E6%97%A5%E6%9C%AC%E8%AA%9E%20file.md");
});
