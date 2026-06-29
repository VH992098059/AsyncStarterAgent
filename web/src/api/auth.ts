// 鉴权相关 API 客户端（决策 #2 扩 MVP）
// - register / login / logout / me 四个端点
// - 维护 localStorage 中的 token
// - 注册/登录成功后返回 token + 用户信息；调用方负责存储与跳转
import { apiFetch, setToken, clearToken } from "./client";

export interface AuthResponse {
  token: string;
  user_id: string;
  username: string;
  expires_in: number;
}

export interface MeResponse {
  user_id: string;
  username: string;
}

export interface LogoutResponse {
  revoked: boolean;
}

/** POST /api/v1/auth/register - 注册新用户 */
export async function register(username: string, password: string): Promise<AuthResponse> {
  return apiFetch<AuthResponse>("/api/v1/auth/register", {
    method: "POST",
    body: { username, password },
    auth: false,
  });
}

/** POST /api/v1/auth/login - 登录 */
export async function login(username: string, password: string): Promise<AuthResponse> {
  return apiFetch<AuthResponse>("/api/v1/auth/login", {
    method: "POST",
    body: { username, password },
    auth: false,
  });
}

/** POST /api/v1/auth/logout - 登出（撤销当前 token） */
export async function logout(): Promise<LogoutResponse> {
  const res = await apiFetch<LogoutResponse>("/api/v1/auth/logout", { method: "POST" });
  // 无论后端如何返回，本地都清掉
  clearToken();
  return res;
}

/** GET /api/v1/auth/me - 拉取当前用户（用于刷新页面后验证 token 仍有效） */
export async function getMe(): Promise<MeResponse> {
  return apiFetch<MeResponse>("/api/v1/auth/me");
}

// 把 token 存到 localStorage 的便捷包装
export function saveAuth(auth: AuthResponse): void {
  setToken(auth.token);
}
