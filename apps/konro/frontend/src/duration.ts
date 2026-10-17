/** ISO 8601 time-only durations shared by display and editing. */
export function parseDuration(value = ""): number | null {
 const m = value.match(/^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$/);
 return m && m.slice(1).some(Boolean) ? Number(m[1] || 0)*3600 + Number(m[2] || 0)*60 + Number(m[3] || 0) : null;
}
export function formatDuration(iso?: string): string {
 if (!iso) return "";
 const seconds = parseDuration(iso);
 if (seconds === null) return iso;
 const h=Math.floor(seconds/3600), m=Math.floor(seconds%3600/60), s=seconds%60;
 return `${h ? `${h}時間` : ""}${m ? `${m}分` : ""}${s || !seconds ? `${s}秒` : ""}`;
}
export function durationMinutes(value = ""): string {
 const seconds=parseDuration(value);
 return seconds === null ? "" : String(seconds/60);
}
export function durationFromMinutes(value: string, old = ""): string {
 if (value === durationMinutes(old)) return old;
 if (!value.trim()) return "";
 const n=Number(value);
 if (!Number.isFinite(n) || n<0 || n>100000) throw new Error("時間は0〜100000分で入力してください");
 const seconds=Math.round(n*60), mins=Math.floor(seconds/60), rest=seconds%60;
 return `PT${mins ? `${mins}M` : ""}${rest || !mins ? `${rest}S` : ""}`;
}
