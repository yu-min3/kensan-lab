import { expect, test } from "vitest";
import { markportURL } from "./markport";

test("workspace 相対パスを markport の path パラメータへ渡す", () => {
  expect(markportURL("notes/日本語 file.md"))
    .toBe("http://127.0.0.1:3000/?path=notes%2F%E6%97%A5%E6%9C%AC%E8%AA%9E%20file.md");
});
