import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { applyTheme, readTheme, saveTheme, THEME_KEY } from "./theme";

function mockSystem(dark: boolean) {
  vi.stubGlobal("matchMedia", (q: string) => ({
    matches: dark && q.includes("dark"),
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

// jsdom と Node の localStorage がぶつかる環境があるので、Map で差し替える
function mockStorage() {
  const m = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, v),
    removeItem: (k: string) => void m.delete(k),
  });
}

describe("theme", () => {
  beforeEach(() => mockStorage());
  afterEach(() => {
    vi.unstubAllGlobals();
    document.documentElement.classList.remove("dark");
  });

  it("既定は system で、OS が暗ければ .dark を付ける", () => {
    mockSystem(true);
    expect(readTheme()).toBe("system");
    applyTheme(readTheme());
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("明を選ぶと OS が暗くても明るく、選択を覚える", () => {
    mockSystem(true);
    saveTheme("light");
    expect(localStorage.getItem(THEME_KEY)).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("自動に戻すと保存を消す", () => {
    mockSystem(false);
    saveTheme("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    saveTheme("system");
    expect(localStorage.getItem(THEME_KEY)).toBeNull();
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});
