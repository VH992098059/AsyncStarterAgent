import { useState, useEffect, useCallback } from "react";

/**
 * Route 联合类型。工作流页（context/draft/delivery）必须带 runId。
 * selectedTaskId / chatOpen 是看板内部状态，不进 URL。
 */
export type Route =
  | { page: "board" }
  | { page: "trigger" }
  | { page: "settings" }
  | { page: "context"; runId: string }
  | { page: "draft"; runId: string }
  | { page: "delivery"; runId: string };

/** 工作流页集合，用于类型守卫 */
const WORKFLOW_PAGES = new Set(["context", "draft", "delivery"]);

function isWorkflowPage(page: string): page is "context" | "draft" | "delivery" {
  return WORKFLOW_PAGES.has(page);
}

/** 把 location.hash 解析成 Route。非法路径回退到 board。 */
function parseHash(hash: string): Route {
  // hash 形如 "#/board" / "#/run/:runId/context" / "" （空视为 board）
  const path = hash.replace(/^#/, "");
  if (path === "" || path === "/" || path === "/board") return { page: "board" };
  if (path === "/trigger") return { page: "trigger" };
  if (path === "/settings") return { page: "settings" };
  const m = path.match(/^\/run\/([^/]+)\/(context|draft|delivery)$/);
  if (m) return { page: m[2] as "context" | "draft" | "delivery", runId: m[1] };
  return { page: "board" };
}

/** 把 Route 序列化成 location.hash 字符串。 */
function routeToHash(route: Route): string {
  switch (route.page) {
    case "board":
      return "#/board";
    case "trigger":
      return "#/trigger";
    case "settings":
      return "#/settings";
    default:
      return `#/run/${route.runId}/${route.page}`;
  }
}

export interface HashRouter {
  route: Route;
  /** 导航到新路由；会写 location.hash 触发 hashchange */
  navigate: (route: Route) => void;
  /** 浏览器后退 */
  back: () => void;
}

/**
 * useHashRouter 监听 hashchange，提供 route / navigate / back。
 * Tauri 环境兼容性最佳（无 server 路由，刷新不 404）。
 */
export function useHashRouter(): HashRouter {
  const [route, setRoute] = useState<Route>(() => parseHash(window.location.hash));

  useEffect(() => {
    const onHashChange = () => setRoute(parseHash(window.location.hash));
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const navigate = useCallback((next: Route) => {
    const hash = routeToHash(next);
    if (window.location.hash !== hash) {
      // 写 hash 会触发 hashchange → setRoute
      window.location.hash = hash;
    } else {
      // 同 hash 手动更新（避免静默忽略）
      setRoute(next);
    }
  }, []);

  const back = useCallback(() => window.history.back(), []);

  return { route, navigate, back };
}

/** 类型守卫：route 是否为工作流页（含 runId） */
export function isWorkflowRoute(route: Route): route is Extract<Route, { runId: string }> {
  return isWorkflowPage(route.page);
}
