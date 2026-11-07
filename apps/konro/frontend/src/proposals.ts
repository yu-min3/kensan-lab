import type {Recipe} from './api';

export type Conditions = {
  idea: string; count: number; servings?: string; main: number; side: number;
  cuisine: string; avoid: string[]; use: string[]; fixed: string[]; omit: string[]; seed: number;
};
export type Proposal = {
  id: string; revision: number; state: string; mode: string; pending?: boolean;
  createdAt?: string; conditions: Conditions; interpretation: string; error?: string;
  job?: {id: string; state: string; reason?: string; created_at?: string; updated_at?: string};
  candidates: {recipe: Recipe; scale: string}[];
  menus: {id: string; picks: {file: string; reason: string}[]}[];
};
export const isActiveProposal = (state: string) => ['queued', 'running', 'waiting'].includes(state);
export async function proposalRequest<T>(path: string, method = 'GET', body?: unknown, operation: string = crypto.randomUUID()): Promise<T> {
  const response = await fetch('/api/v1/menu-proposals' + path, {
    method, headers: {'Content-Type': 'application/json', 'Idempotency-Key': operation},
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(20000),
  });
  if (!response.ok) throw new Error((await response.text()).trim());
  return response.json();
}
