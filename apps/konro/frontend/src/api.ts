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
