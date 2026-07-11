import { useEffect, useState, useCallback } from "react";
import { Layout, showToast, type PageKey } from "./components/Layout";
import { KanbanBoard } from "./components/KanbanBoard";
import { ChatPanel } from "./components/ChatPanel";
import { TriggerConfig } from "./components/TriggerConfig";
import { ContextCollection } from "./components/ContextCollection";
import { DraftEditor } from "./components/DraftEditor";
import { Delivery } from "./components/Delivery";
import { Settings } from "./components/Settings";
import { Login } from "./components/Login";
import { getDraft, type Draft, AUTH_LOGOUT_EVENT, getToken, clearToken, triggerAgent, type AgentRunItem } from "./api/client";
import { getMe, logout as apiLogout } from "./api/auth";
import { useIsMobile } from "./hooks/useIsMobile";
import { useKeyboardShortcut } from "./hooks/useKeyboardShortcut";
import { useHashRouter, isWorkflowRoute } from "./hooks/useHashRouter";
import { useStreamDraft } from "./hooks/useStreamDraft";

type AuthState = "loading" | "authenticated" | "unauthenticated";

const PAGE_MAP: Record<number, PageKey> = {
  1: "board",
  2: "trigger",
  3: "context",
  4: "draft",
  5: "delivery",
  6: "settings",
};

export default function App() {
  const isMobile = useIsMobile();
  const { route, navigate: navigateRoute } = useHashRouter();
  const currentPage: PageKey = route.page;
  const routeRunId = isWorkflowRoute(route) ? route.runId : null;

  const [authState, setAuthState] = useState<AuthState>("loading");
  const [username, setUsername] = useState<string>("");

  const [selectedTask, setSelectedTask] = useState<AgentRunItem | null>(null);
  const [chatOpen, setChatOpen] = useState(false);
  const [draft, setDraft] = useState<Draft>({ content: "", marks: [], completeness: 0 });
  const [runId, setRunId] = useState<string | null>(null);
  const [isStreaming, setIsStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  useEffect(() => {
    const tok = getToken();
    if (!tok) {
      setAuthState("unauthenticated");
      return;
    }
    getMe()
      .then((me) => {
        setAuthState("authenticated");
        setUsername(me.username);
      })
      .catch(() => {
        clearToken();
        setAuthState("unauthenticated");
      });
  }, []);

  useEffect(() => {
    const onLogout = () => {
      setAuthState("unauthenticated");
      navigateRoute({ page: "board" });
      setRunId(null);
      setSelectedTask(null);
      setChatOpen(false);
    };
    window.addEventListener(AUTH_LOGOUT_EVENT, onLogout);
    return () => window.removeEventListener(AUTH_LOGOUT_EVENT, onLogout);
  }, [navigateRoute]);

  const handleAuthenticated = useCallback(() => {
    setAuthState("authenticated");
    navigateRoute({ page: "board" });
  }, [navigateRoute]);

  const handleLogout = useCallback(async () => {
    try {
      await apiLogout();
    } catch {
    }
    clearToken();
    setAuthState("unauthenticated");
    setRunId(null);
    setSelectedTask(null);
    setChatOpen(false);
    navigateRoute({ page: "board" });
  }, [navigateRoute]);

  const handleTrigger = useCallback(async (newRunId: string) => {
    setRunId(newRunId);
    setDraft({ content: "", marks: [], completeness: 0 });
    setIsStreaming(true);
    setError(null);
    navigateRoute({ page: "context", runId: newRunId });
  }, [navigateRoute]);

  const handleConfirmed = useCallback(() => {
    if (runId) navigateRoute({ page: "delivery", runId });
  }, [runId, navigateRoute]);

  const handleAddTask = useCallback(async (text: string) => {
    const res = await triggerAgent(text);
    setRunId(res.run_id);
    setDraft({ content: "", marks: [], completeness: 0 });
    setSelectedTask(null);
    setChatOpen(false);
    setError(null);
    // 不跳 context，留在看板；SSE 在后台流式生成草稿，用户从看板点进去可查看
    navigateRoute({ page: "board" });
  }, [navigateRoute]);

  // 从看板卡片菜单"查看草稿"跳转到该 run 的草稿页
  const handleViewDraft = useCallback((runId: string) => {
    navigateRoute({ page: "draft", runId });
  }, [navigateRoute]);

  // SSE 流式拉取草稿：useStreamDraft 封装了 3 次重连逻辑，
  // 短暂网络抖动会自动重连，连续 3 次 CLOSED 才彻底失败。
  useStreamDraft(
    runId,
    retryKey,
    (text) => {
      setDraft((d) => ({ ...d, content: d.content + text }));
    },
    (marks) => {
      setDraft((d) => ({ ...d, marks, completeness: 1 }));
      setIsStreaming(false);
    },
    (err) => {
      setError(err.message);
      showToast("SSE 错误", err.message, "error");
      setIsStreaming(false);
    }
  );

  // 进入工作流页（routeRunId 变化）时恢复 draft：刷新或直接 URL 进入时
  // getDraft 拉取已落库内容；若未完成则启动 SSE 续传。
  useEffect(() => {
    if (!routeRunId) return;
    if (routeRunId === runId) return; // 已在跑 SSE，无需恢复
    getDraft(routeRunId)
      .then((d) => {
        if (d.completeness >= 1) {
          // 已完成：直接显示，不启动 SSE
          setDraft({ content: d.markdown, marks: d.marks, completeness: d.completeness });
          setIsStreaming(false);
        } else {
          // 未完成：重置 draft 并启动 SSE 续传
          setDraft({ content: "", marks: [], completeness: 0 });
          setRunId(routeRunId);
          setIsStreaming(true);
          setError(null);
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : String(err));
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeRunId]);

  const handleRetrySSE = useCallback(() => {
    setError(null);
    setRetryKey((k) => k + 1);
  }, []);

  // navigate 兼容子组件传 PageKey | number（沿用 PAGE_MAP），转调 router.navigate
  const navigate = (page: PageKey | number) => {
    const key = typeof page === "number" ? PAGE_MAP[page] || "board" : page;
    if (key === "context" || key === "draft" || key === "delivery") {
      if (runId) navigateRoute({ page: key, runId });
    } else {
      navigateRoute({ page: key });
    }
    window.scrollTo(0, 0);
  };

  const handleSelectTask = (task: AgentRunItem) => {
    setSelectedTask(task);
    setChatOpen(true);
  };

  const handleCloseChat = useCallback(() => {
    setChatOpen(false);
    setSelectedTask(null);
  }, []);

  useKeyboardShortcut("Escape", () => {
    if (chatOpen) handleCloseChat();
  }, [chatOpen, handleCloseChat]);

  if (authState === "loading") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#0a0a0a]">
        <svg className="animate-spin h-6 w-6 text-emerald-500" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      </div>
    );
  }

  if (authState === "unauthenticated") {
    return <Login onAuthenticated={handleAuthenticated} />;
  }

  const renderPage = () => {
    switch (currentPage) {
      case "board":
        return (
          <KanbanBoard
            onSelectTask={handleSelectTask}
            onDeselectTask={handleCloseChat}
            selectedTaskId={selectedTask?.id ?? null}
            onAddTask={handleAddTask}
            onViewDraft={handleViewDraft}
          />
        );
      case "trigger":
        return (
          <TriggerConfig
            onTrigger={handleTrigger}
            onError={setError}
            onNavigate={navigate}
          />
        );
      case "context":
        return (
          <ContextCollection
            draft={draft}
            runId={routeRunId ?? runId}
            onNavigate={navigate}
            error={error}
            onRetry={handleRetrySSE}
          />
        );
      case "draft":
        return (
          <DraftEditor
            draft={draft}
            runId={routeRunId ?? runId}
            onConfirmed={handleConfirmed}
            onNavigate={navigate}
          />
        );
      case "delivery":
        return (
          <Delivery
            draft={draft}
            runId={routeRunId ?? runId}
            onNavigate={navigate}
          />
        );
      case "settings":
        return <Settings />;
      default:
        return null;
    }
  };

  const chatPanel = (
    <ChatPanel
      task={selectedTask}
      onClose={handleCloseChat}
      isMobile={isMobile}
    />
  );

  return (
    <Layout
      currentPage={currentPage}
      onNavigate={navigate}
      chatOpen={chatOpen}
      onCloseChat={handleCloseChat}
      onLogout={handleLogout}
      username={username}
      onSearch={handleAddTask}
      chatPanel={chatPanel}
    >
      {renderPage()}
    </Layout>
  );
}
