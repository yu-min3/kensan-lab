// Use the same .dark selector as Whetstone; follow OS changes while open.
export function followSystemTheme() {
  const preference = window.matchMedia("(prefers-color-scheme: dark)");
  const apply = () => {
    document.documentElement.classList.toggle("dark", preference.matches);
    const color = getComputedStyle(document.documentElement).getPropertyValue("--background").trim();
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", `hsl(${color})`);
  };
  apply();
  preference.addEventListener("change", apply);
  return () => preference.removeEventListener("change", apply);
}
