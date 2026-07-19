// Cooking-session state, persisted to localStorage so closing the browser
// mid-cook (or an auth 401 → reload) never loses progress. Kitchen device is
// a single phone — no server-side session (see app-plan.md).

export type Timer = {
  id: string;
  recipe: string; // recipe file
  label: string; // "20分" etc.
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

export function formatRemaining(ms: number): string {
  const total = Math.max(0, Math.ceil(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  return `${m}:${String(s).padStart(2, "0")}`;
}
