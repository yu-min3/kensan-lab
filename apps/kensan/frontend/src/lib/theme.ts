// 明暗の切り替え。既定は OS に合わせる（system）。選んだものはこのブラウザだけに覚える。
// 初回描画のちらつきを防ぐため、index.html の先頭でも同じ判定で .dark を付けている（キー名を揃える）。

export type Theme = "system" | "light" | "dark";

export const THEME_KEY = "kensan-theme";
const media = () => window.matchMedia("(prefers-color-scheme: dark)");

export function readTheme(): Theme {
  try {
    const v = localStorage.getItem(THEME_KEY);
    return v === "light" || v === "dark" ? v : "system";
  } catch {
    return "system";
  }
}

export function isDark(theme: Theme): boolean {
  return theme === "dark" || (theme === "system" && media().matches);
}

export function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", isDark(theme));
}

export function saveTheme(theme: Theme) {
  try {
    if (theme === "system") localStorage.removeItem(THEME_KEY);
    else localStorage.setItem(THEME_KEY, theme);
  } catch {
    // 保存できなくても今の画面には効かせる
  }
  applyTheme(theme);
}

// system のときだけ OS の切り替えに追従する。
export function watchSystem(theme: Theme): () => void {
  if (theme !== "system") return () => {};
  const m = media();
  const on = () => applyTheme("system");
  m.addEventListener("change", on);
  return () => m.removeEventListener("change", on);
}
