// Prompt Management API Service
import { API_CONFIG } from '../config'
import { httpClient } from '../client'

export interface EvalSummary {
  interaction_count?: number
  avg_rating?: number
  strengths?: string[]
  weaknesses?: string[]
}

export interface AIContext {
  id: string
  name: string
  situation: string
  version: string
  is_active: boolean
  is_default: boolean
  system_prompt: string
  allowed_tools: string[]
  max_turns: number
  temperature: number
  description: string | null
  created_at: string
  updated_at: string
  current_version_number: number | null
  active_version: number | null
  pending_candidate_count: number
}

export interface AIContextUpdateInput {
  system_prompt?: string
  allowed_tools?: string[]
  max_turns?: number
  temperature?: number
  changelog?: string
}

export interface AIContextVersion {
  id: string
  context_id: string
  version_number: number
  system_prompt: string
  allowed_tools: string[]
  max_turns: number
  temperature: number
  changelog: string | null
  created_at: string
  source: 'manual' | 'ai' | 'rollback'
  eval_summary: EvalSummary | null
  candidate_status: 'pending' | 'adopted' | 'rejected' | null
}

export interface VariableMetadata {
  name: string
  description: string
  example: string
  excludes_tools: string[]
}

export interface ToolMetadata {
  name: string
  description: string
  readonly: boolean
  category: string
}

export interface PromptMetadata {
  variables: VariableMetadata[]
  tools: ToolMetadata[]
}

const BASE = API_CONFIG.baseUrls.ai

// kensan-ai returns JSON directly (no {data: ...} envelope),
// but httpClient.request unwraps json.data if present, otherwise returns json as-is.

export async function fetchMetadata(): Promise<PromptMetadata> {
  return httpClient.get<PromptMetadata>(BASE, '/prompts/metadata')
}

export async function fetchContexts(situation?: string): Promise<AIContext[]> {
  const query = situation ? `?situation=${encodeURIComponent(situation)}` : ''
  return httpClient.get<AIContext[]>(BASE, `/prompts${query}`)
}

export async function fetchContext(id: string): Promise<AIContext> {
  return httpClient.get<AIContext>(BASE, `/prompts/${id}`)
}

export async function updateContext(id: string, data: AIContextUpdateInput): Promise<AIContext> {
  return httpClient.patch<AIContext>(BASE, `/prompts/${id}`, data)
}

export async function fetchVersions(contextId: string): Promise<AIContextVersion[]> {
  return httpClient.get<AIContextVersion[]>(BASE, `/prompts/${contextId}/versions`)
}

export async function fetchVersion(contextId: string, versionNumber: number): Promise<AIContextVersion> {
  return httpClient.get<AIContextVersion>(BASE, `/prompts/${contextId}/versions/${versionNumber}`)
}

export async function rollbackToVersion(contextId: string, versionNumber: number): Promise<AIContext> {
  return httpClient.post<AIContext>(BASE, `/prompts/${contextId}/rollback/${versionNumber}`)
}

export async function adoptVersion(contextId: string, versionNumber: number): Promise<{ status: string; message: string; context: AIContext }> {
  return httpClient.post(BASE, `/prompts/${contextId}/versions/${versionNumber}/adopt`)
}

export async function rejectVersion(contextId: string, versionNumber: number): Promise<{ status: string; message: string; context: AIContext }> {
  return httpClient.post(BASE, `/prompts/${contextId}/versions/${versionNumber}/reject`)
}

export async function runOptimization(force?: boolean): Promise<{
  period_start: string
  period_end: string
  contexts_evaluated: number
  candidates_created: number
  optimized_context_ids: string[]
  errors: string[]
}> {
  const query = force ? '?force=true' : ''
  return httpClient.post(BASE, `/prompts/run-optimization${query}`)
}
