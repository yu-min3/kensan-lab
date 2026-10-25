export type SourcePerson={id?:string;name:string;url?:string};
export type SourceInfo={creator?:SourcePerson;publisher?:SourcePerson;rating?:{value:number;best:number;count?:number}};
export type RecipeMeta = {
  file: string;
  title: string;
  tags: string[] | null;
  servings?: string;
  prepTime?: string;
  cookTime?: string;
  source?: string;
  rating?: number;
  sourceInfo?:SourceInfo;
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

export { formatDuration } from "./duration";

export type ImportContent = { title: string; servings: string; ingredients: string[]; steps: string[] };
export type ImportDraft = {
  id: string; revision: number;
  source: { url?: string; raw: string; text: string; kind?:string;info?:SourceInfo };
  content: ImportContent; warnings: string[]; assessment?: string;
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

export type SearchSite = {id:string;name:string};
export type WebRecipe = {title:string;url:string;site:string;siteName:string;image?:string;description?:string;info?:SourceInfo;preferred?:boolean};
export type SearchResults = {results:WebRecipe[];warnings:string[]};
export async function searchSites():Promise<SearchSite[]> {
 const r=await fetch('/api/v1/discovery/sites');if(!r.ok)throw new Error('検索サイトを読み込めません');return r.json();
}
export async function searchWebRecipes(q:string,site:string,signal:AbortSignal,creator=""):Promise<SearchResults> {
 const params=new URLSearchParams({q,site,...(creator?{creator}: {})});
 const r=await fetch(`/api/v1/discovery/search?${params}`,{signal});
 if(!r.ok)throw new Error((await r.text()).trim()||'検索できませんでした');return r.json();
}

export async function saveRecipePhoto(file:string,version:string,photo:Blob):Promise<Recipe>{
 const res=await fetch(`/api/v1/recipe-photos/${encodeURIComponent(file)}`,{method:'PUT',headers:{'Content-Type':'image/jpeg','If-Match':`"${version}"`},body:photo});
 if(!res.ok)throw new RecipeWriteError((await res.text()).trim(),res.status);
 return res.json();
}

export type PlanItem={id:string;recipe:Recipe;scale:string;overrides?:Record<string,string>};
export type ShoppingItem={id:string;line:{raw:string;name:string;kind:string;min?:string;max?:string;unit?:string};text:string;status:'buy'|'home'|'bought'|'review';have?:string;note?:string;manual:boolean;sources:{itemId:string;file:string;version:string;title:string;row:number;raw:string;scaled:string}[]};
export type MealPlan={id:string;revision:number;name:string;items:PlanItem[];shopping:ShoppingItem[]};
export async function planRequest<T>(path='',method='GET',body?:unknown,operation=crypto.randomUUID()):Promise<T>{
 const r=await fetch('/api/v1/meal-plans'+path,{method,headers:{'Content-Type':'application/json','Idempotency-Key':operation},body:body===undefined?undefined:JSON.stringify(body)});
 if(!r.ok)throw new RecipeWriteError((await r.text()).trim()||'買い物リストを取得できません',r.status);return r.json();
}
