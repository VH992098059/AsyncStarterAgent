// 飞书 OAuth 授权 API 客户端（决策 #7 / F012）
// - getFeishuStatus  GET  /api/v1/auth/feishu/status   查询当前用户授权状态
// - startFeishuAuth  GET  /api/v1/auth/feishu/start    拉起授权（返回 authorize_url）
// - revokeFeishuAuth POST /api/v1/auth/feishu/revoke   撤销授权
//
// 注意：apiFetch<T> 已解析 {code,message,data} 信封并直接返回 data as T，
// 因此这里不能再 .json() 二次解析（与 auth.ts 风格一致）。
import { apiFetch } from "./client";

export interface FeishuAuthStatus {
  status: "authorized" | "not_authorized";
  name: string;
}

/** GET /api/v1/auth/feishu/status - 查询当前用户飞书授权状态 */
export async function getFeishuStatus(): Promise<FeishuAuthStatus> {
  return apiFetch<FeishuAuthStatus>("/api/v1/auth/feishu/status");
}

/** GET /api/v1/auth/feishu/start - 拉起飞书授权，返回跳转 URL */
export async function startFeishuAuth(): Promise<{ authorize_url: string }> {
  return apiFetch<{ authorize_url: string }>("/api/v1/auth/feishu/start");
}

/** POST /api/v1/auth/feishu/revoke - 撤销飞书授权 */
export async function revokeFeishuAuth(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/auth/feishu/revoke", {
    method: "POST",
  });
}

export interface FeishuAppConfig {
  app_id: string;
  app_secret_masked: string;
  configured: boolean;
}

/** GET /api/v1/feishu/app-config - 查询当前用户飞书应用凭证配置状态 */
export async function getFeishuAppConfig(): Promise<FeishuAppConfig> {
  return apiFetch<FeishuAppConfig>("/api/v1/feishu/app-config");
}

/** PUT /api/v1/feishu/app-config - 保存/更新飞书应用凭证。app_secret 留空或传掩码值保持原值不变 */
export async function updateFeishuAppConfig(appId: string, appSecret: string): Promise<FeishuAppConfig> {
  return apiFetch<FeishuAppConfig>("/api/v1/feishu/app-config", {
    method: "PUT",
    body: { app_id: appId, app_secret: appSecret },
  });
}

/** DELETE /api/v1/feishu/app-config - 删除飞书应用凭证（级联清除已有 OAuth 授权） */
export async function deleteFeishuAppConfig(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/feishu/app-config", { method: "DELETE" });
}
