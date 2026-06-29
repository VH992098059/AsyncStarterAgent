import axios, { AxiosError } from "axios";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:3080";

// ========== Types ==========

export interface Mark {
  id: string;
  hint: string;
  position: number;
  resolved: boolean;
}

export interface Draft {
  content: string;
  marks: Mark[];
  completeness: number;
}

export interface ApiResponse<T> {
  code: number;
  message: string;
  data?: T;
}

export interface TriggerResponse {
  run_id: string;
}

export interface DeliverResult {
  delivery_id: string;
  target_url: string;
  status: "success" | "failed";
}

export interface HealthResponse {
  status: string;
  env: string;
}

// ========== Auth + Axios Wrapper ==========

// localStorage 中 token 的 key。所有受保护请求都从这里取。
export const TOKEN_KEY = "asa_token";

// 自定义事件：401 触发。App 监听后清除本地状态 + 切到登录页。
export const AUTH_LOGOUT_EVENT = "asa:auth-logout";

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token);
  } catch {
    // localStorage 不可用时静默（极少见，仅 SSR / 隐私模式）
  }
}

export function clearToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {
    // ignore
  }
}

// 派发"被踢下线"事件。App 监听此事件做响应。
export function emitAuthLogout(): void {
  window.dispatchEvent(new CustomEvent(AUTH_LOGOUT_EVENT));
}

// apiFetch：所有受保护接口统一走这里。
//  - 自动注入 Authorization: Bearer <token>
//  - 401 → 清 token + 派发事件 + 抛 AuthError（让上层决定跳转文案）
//  - 其余非 2xx 抛出 Error，含后端 message 字段
export class AuthError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
    this.name = "AuthError";
  }
}

export interface ApiFetchOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  // 是否需要 Authorization（默认 true；auth 自身/login/register 显式 false）
  auth?: boolean;
  signal?: AbortSignal;
}

const axiosClient = axios.create({
  baseURL: API_BASE,
  headers: { "Content-Type": "application/json" },
});

export async function apiFetch<T>(path: string, opts: ApiFetchOptions = {}): Promise<T> {
  const { method = "GET", body, auth = true, signal } = opts;
  const headers: Record<string, string> = {};
  if (auth) {
    const tok = getToken();
    if (tok) headers["Authorization"] = `Bearer ${tok}`;
  }

  try {
    const res = await axiosClient.request<ApiResponse<T>>({
      url: path,
      method,
      headers,
      data: body,
      signal,
    });

    const json = res.data;
    // 204 / 空 body 容忍
    if (!json) return undefined as T;
    if (json.code !== 0) throw new Error(json.message);
    return json.data as T;
  } catch (err) {
    if (axios.isAxiosError(err)) {
      const axErr = err as AxiosError<ApiResponse<unknown>>;
      const status = axErr.response?.status;
      const message = axErr.response?.data?.message || axErr.message;

      if (status === 401) {
        // 黑名单命中 / token 过期 / 缺失，都清掉并通知 App。
        clearToken();
        emitAuthLogout();
        throw new AuthError(message || "未授权", 401);
      }

      throw new Error(message || `请求失败: ${status ?? "网络错误"}`);
    }
    throw err;
  }
}

// ========== API Functions ==========

/** POST /api/v1/trigger - 手动触发 Agent (FR-A04) */
export async function triggerAgent(text: string): Promise<TriggerResponse> {
  return apiFetch<TriggerResponse>("/api/v1/trigger", { method: "POST", body: { text } });
}

/** GET /api/v1/drafts/:id/stream - SSE 流式获取草稿 (FR-C05)
 *  EventSource 不支持自定义 Header，因此 token 走 query string 后端读取。
 *  后端 SSE 端点接受 ?token= 形式（这是 EventSource 的标准限制）。
 */
export function streamDraft(
  runId: string,
  onDelta: (text: string) => void,
  onComplete: (marks: Mark[]) => void,
  onError: (err: Error) => void
): () => void {
  const tok = getToken();
  const url = tok
    ? `${API_BASE}/api/v1/drafts/${runId}/stream?token=${encodeURIComponent(tok)}`
    : `${API_BASE}/api/v1/drafts/${runId}/stream`;
  const es = new EventSource(url);
  es.addEventListener("delta", (e) => {
    try {
      const data = JSON.parse((e as MessageEvent).data);
      onDelta(data.text);
    } catch (err) {
      onError(new Error(`parse delta: ${err instanceof Error ? err.message : String(err)}`));
    }
  });
  es.addEventListener("complete", (e) => {
    try {
      const data = JSON.parse((e as MessageEvent).data);
      onComplete(data.marks ?? []);
    } catch (err) {
      onError(new Error(`parse complete: ${err instanceof Error ? err.message : String(err)}`));
    }
    es.close();
  });
  es.addEventListener("error", () => {
    onError(new Error("SSE connection error"));
    es.close();
  });
  return () => es.close();
}

/** POST /api/v1/drafts/:id/deliver - 交付草稿 (FR-D01) */
export async function deliverDraft(
  draftId: string,
  targetType: "notion" | "obsidian"
): Promise<DeliverResult> {
  return apiFetch<DeliverResult>(`/api/v1/drafts/${draftId}/deliver`, {
    method: "POST",
    body: { target_type: targetType },
  });
}

/** GET /health - 健康检查（公开端点，不需要 auth） */
export async function checkHealth(): Promise<HealthResponse> {
  try {
    const res = await axiosClient.get<ApiResponse<HealthResponse>>("/health");
    return res.data.data!;
  } catch (err) {
    if (axios.isAxiosError(err)) {
      throw new Error(`健康检查失败: ${err.response?.status ?? err.message}`);
    }
    throw err;
  }
}

// ========== Lists (决策 #2 扩 MVP) ==========

export interface KeywordItem {
  pattern: string;
  task_type: string;
}

/** GET /api/v1/keywords - 关键词规则（来自 trigger.Matcher） */
export async function listKeywords(): Promise<KeywordItem[]> {
  return apiFetch<KeywordItem[]>("/api/v1/keywords");
}

export interface DataSourceItem {
  id: string;
  type: string;
  name: string;
  status: string;
  last_sync_at?: string | null;
}

/** GET /api/v1/datasources - 当前用户的数据源列表 */
export async function listDataSources(): Promise<DataSourceItem[]> {
  return apiFetch<DataSourceItem[]>("/api/v1/datasources");
}

export interface AgentRunItem {
  id: string;
  task_type: string;
  status: string;
  current_stage: string;
  trigger_type: string;
  trigger_source: string;
  error_message: string;
  created_at: string;
  completed_at?: string | null;
}

export interface AgentRunStats {
  total: Record<string, number>;
  today: Record<string, number>;
}

export interface AgentRunsResponse {
  runs: AgentRunItem[];
  stats: AgentRunStats;
}

/** GET /api/v1/agent-runs?limit=N - 近期运行列表 + 状态统计 */
export async function listAgentRuns(limit = 20): Promise<AgentRunsResponse> {
  return apiFetch<AgentRunsResponse>(`/api/v1/agent-runs?limit=${limit}`);
}

// ========== Settings（模型/Embedding/第三方配置） ==========

export interface UserSettings {
  llm_api_key: string;
  llm_base_url: string;
  llm_model: string;
  llm_temperature: number;
  llm_max_tokens: number;
  embed_api_key: string;
  embed_base_url: string;
  embed_model: string;
  notion_api_key: string;
  notion_parent_page: string;
  obsidian_vault_path: string;
}

export interface ModelPreset {
  id: string;
  name: string;
  base_url: string;
  llm_model: string;
  embed_model: string;
  cloud: boolean;
}

export const MODEL_PRESETS: ModelPreset[] = [
  { id: "openai", name: "OpenAI (GPT-4o-mini)", base_url: "https://api.openai.com/v1", llm_model: "gpt-4o-mini", embed_model: "text-embedding-3-small", cloud: true },
  { id: "openai-gpt4o", name: "OpenAI (GPT-4o)", base_url: "https://api.openai.com/v1", llm_model: "gpt-4o", embed_model: "text-embedding-3-small", cloud: true },
  { id: "anthropic", name: "Anthropic Claude (via OpenAI-compatible)", base_url: "https://api.anthropic.com/v1", llm_model: "claude-sonnet-4-20250514", embed_model: "", cloud: true },
  { id: "deepseek", name: "DeepSeek", base_url: "https://api.deepseek.com/v1", llm_model: "deepseek-chat", embed_model: "", cloud: true },
  { id: "moonshot", name: "Moonshot (Kimi)", base_url: "https://api.moonshot.cn/v1", llm_model: "moonshot-v1-8k", embed_model: "", cloud: true },
  { id: "qwen", name: "通义千问 (DashScope)", base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1", llm_model: "qwen-plus", embed_model: "text-embedding-v2", cloud: true },
  { id: "zhipu", name: "智谱 GLM", base_url: "https://open.bigmodel.cn/api/paas/v4", llm_model: "glm-4-flash", embed_model: "embedding-2", cloud: true },
  { id: "siliconflow", name: "SiliconFlow", base_url: "https://api.siliconflow.cn/v1", llm_model: "deepseek-ai/DeepSeek-V3", embed_model: "BAAI/bge-m3", cloud: true },
  { id: "ollama", name: "Ollama 本地", base_url: "http://localhost:11434/v1", llm_model: "qwen2.5:7b", embed_model: "nomic-embed-text", cloud: false },
  { id: "lmstudio", name: "LM Studio 本地", base_url: "http://localhost:1234/v1", llm_model: "local-model", embed_model: "", cloud: false },
  { id: "custom", name: "自定义（手动填写）", base_url: "", llm_model: "", embed_model: "", cloud: false },
];

export async function getSettings(): Promise<UserSettings> {
  return apiFetch<UserSettings>("/api/v1/settings");
}

export async function updateSettings(body: Partial<UserSettings>): Promise<UserSettings> {
  return apiFetch<UserSettings>("/api/v1/settings", { method: "PUT", body });
}

export async function testLLMConnection(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/settings/test-llm", { method: "POST" });
}

export function isMaskedKey(key: string): boolean {
  if (!key) return true;
  return /\*{4}/.test(key);
}
