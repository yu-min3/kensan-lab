export type RecipeMeta = {
  file: string;
  title: string;
  tags: string[] | null;
  servings?: string;
  prepTime?: string;
  cookTime?: string;
  source?: string;
  rating?: number;
  images?: string[];
};

export type Recipe = RecipeMeta & {
  version: string;
  ingredients: string[] | null;
  steps: string[] | null;
  notes?: string;
};

export async function fetchRecipes(): Promise<RecipeMeta[]> {
  const res = await fetch("/api/v1/recipes");
  if (!res.ok) throw new Error(`recipes: ${res.status}`);
  return (await res.json()) ?? [];
}

export async function fetchRecipe(file: string): Promise<Recipe> {
  const res = await fetch(`/api/v1/recipes/${encodeURIComponent(file)}`);
  if (!res.ok) throw new Error(`recipe ${file}: ${res.status}`);
  return res.json();
}

export type ImportReport = {
  recipes: number;
  images: number;
  emptySections: number;
  skipped?: number;
  warnings: string[] | null;
  unknownProps: string[] | null;
};

/** Recipe Keeper のエクスポート zip をそのまま body で送る（multipart ではない） */
export async function importRecipeZip(file: File): Promise<ImportReport> {
  const res = await fetch("/api/v1/import", { method: "POST", body: file });
  if (!res.ok) throw new Error(`${res.status} ${(await res.text()).trim()}`);
  return res.json();
}

/** "PT1H20M" → "1時間20分" */
export function formatDuration(iso?: string): string {
  if (!iso) return "";
  const m = iso.match(/^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$/);
  if (!m) return iso;
  const [, h, min, s] = m;
  let out = "";
  if (h) out += `${h}時間`;
  if (min) out += `${min}分`;
  if (s && !h && !min) out += `${s}秒`;
  return out;
}

export type ImportContent = { title: string; servings: string; ingredients: string[]; steps: string[] };
export type ImportDraft = {
  id: string; revision: number;
  source: { url?: string; raw: string; text: string };
  content: ImportContent; warnings: string[];
  pending: boolean; detached: boolean; agent_available: boolean; saved_file?: string;
  job?: { id: string; state: string; reason?: string };
};
export async function draftRequest(path: string, method = "GET", body?: unknown): Promise<ImportDraft> {
  const res = await fetch(`/api/v1/import/drafts${path}`, {
    method, headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) throw new Error((await res.text()).trim());
  return res.json();
}

export type RecipeEdit = Pick<Recipe, "title" | "servings" | "tags" | "prepTime" | "cookTime" | "source" | "rating" | "ingredients" | "steps" | "notes">;
export class RecipeWriteError extends Error { constructor(message: string, public status: number) { super(message); } }
export async function saveRecipe(file: string | null, version: string, input: RecipeEdit, createID: string): Promise<Recipe> {
  const res = await fetch(file ? `/api/v1/recipes/${encodeURIComponent(file)}` : "/api/v1/recipes", {
    method: file ? "PUT" : "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": createID },
    body: JSON.stringify(file ? { ...input, version } : input),
  });
  if (!res.ok) throw new RecipeWriteError((await res.text()).trim(), res.status);
  return res.json();
}
