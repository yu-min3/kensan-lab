// ノートの閲覧・検索はローカルの markport（read-only ビューア）に任せる（2026-10-09〜）。
// markport は Yu の Mac 上で workspace を root に 127.0.0.1 で動くため、スマホからは開けない。
const MARKPORT_BASE = (import.meta.env.VITE_MARKPORT_URL ?? "http://127.0.0.1:3000").replace(/\/+$/, "");

export function markportURL(path: string): string {
  return `${MARKPORT_BASE}/?path=${encodeURIComponent(path)}`;
}
