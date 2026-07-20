// Cooking-session state, persisted to localStorage so closing the browser
// mid-cook (or an auth 401 → reload) never loses progress. Kitchen device is
// a single phone — no server-side session (see app-plan.md).

export type Timer = {
  id: string;
  recipe: string; // recipe file
  label: string; // "20分" etc.
  note?: string; // step context, e.g. "塩(少々)を…" (absent in old sessions)
  endsAt: number; // epoch ms
  acknowledged: boolean; // user dismissed the fired alarm
};

export type Session = {
  startedAt: number;
  files: string[];
  active: number;
  steps: Record<string, boolean[]>;
  ingredients: Record<string, boolean[]>;
  timers: Timer[];
};

const KEY = "konro-session-v1";

export function loadSession(): Session | null {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

export function saveSession(s: Session | null) {
  if (s === null) localStorage.removeItem(KEY);
  else localStorage.setItem(KEY, JSON.stringify(s));
}

export function newSession(files: string[]): Session {
  return {
    startedAt: Date.now(),
    files,
    active: 0,
    steps: {},
    ingredients: {},
    timers: [],
  };
}

/** Extract timer candidates like 20分 / 1時間 / 30秒 from a step text.
 * Full-width digits (２０分) are common in Japanese recipes — normalize first. */
export function timerCandidates(step: string): { label: string; seconds: number }[] {
  const normalized = step.replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0));
  const out: { label: string; seconds: number }[] = [];
  const seen = new Set<string>();
  for (const m of normalized.matchAll(/(\d+)\s*(時間|分|秒)/g)) {
    const n = parseInt(m[1], 10);
    const unit = m[2];
    const seconds = unit === "時間" ? n * 3600 : unit === "分" ? n * 60 : n;
    const label = `${n}${unit}`;
    if (seconds > 0 && seconds <= 12 * 3600 && !seen.has(label)) {
      seen.add(label);
      out.push({ label, seconds });
    }
  }
  return out;
}

/** Ingredient lines that are group labels (単独A/B, 【ソース】, ■南蛮酢),
 * rendered as section dividers instead of checkable items. Conservative:
 * anything carrying a quantity stays a normal ingredient. */
export function isIngredientGroup(line: string): boolean {
  const t = line.trim();
  if (/^[A-ZＡ-Ｚa-z]$/.test(t)) return true;
  if (/^[【\[（(].*[】\]）)]$/.test(t)) return true;
  if (/^[■●▼◆☆★◎○]/.test(t) && !/[0-9０-９]/.test(t) && !/適量|少々|お好み/.test(t)) return true;
  return false;
}

/** Loose search normalization: lowercase, hiragana→katakana, full→half width. */
export function normalizeQuery(s: string): string {
  return s
    .toLowerCase()
    .replace(/[ぁ-ん]/g, (c) => String.fromCharCode(c.charCodeAt(0) + 0x60))
    .replace(/[Ａ-Ｚａ-ｚ０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0));
}

/** Strip decorations (【作り置き】, ☆) so burner tabs spend their ~5 visible
 * chars on the dish name. */
export function tabName(title: string): string {
  const t = title.replace(/【[^】]*】/g, "").replace(/[☆★♪〜~]/g, "").trim();
  return t || title;
}

export function formatRemaining(ms: number): string {
  const total = Math.max(0, Math.ceil(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  return `${m}:${String(s).padStart(2, "0")}`;
}
