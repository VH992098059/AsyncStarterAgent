import { useEffect, useState, useCallback } from "react";
import { Layout, type PageKey } from "./components/Layout";
import { KanbanBoard } from "./components/KanbanBoard";
import { ChatPanel } from "./components/ChatPanel";
import { TriggerConfig } from "./components/TriggerConfig";
import { ContextCollection } from "./components/ContextCollection";
import { DraftEditor } from "./components/DraftEditor";
import { Delivery } from "./components/Delivery";
import { Settings } from "./components/Settings";
import { Login } from "./components/Login";
import { streamDraft, type Draft, AUTH_LOGOUT_EVENT, getToken, clearToken, triggerAgent, type AgentRunItem } from "./api/client";
import { getMe, logout as apiLogout } from "./api/auth";

type AuthState = "loading" | "authenticated" | "unauthenticated";

const PAGE_MAP: Record<number, PageKey> = {
  1: "board",
  2: "trigger",
  3: "context",
  4: "draft",
  5: "delivery",
  6: "settings",
};

function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 768);
  useEffect(() => {
    const handleResize = () => setIsMobile(window.innerWidth <= 768);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);
  return isMobile;
}

export default function App() {
  const isMobile = useIsMobile();
  const [authState, setAuthState] = useState<AuthState>("loading");
  const [username, setUsername] = useState<string>("");

  const [currentPage, setCurrentPage] = useState<PageKey>("board");
  const [selectedTask, setSelectedTask] = useState<AgentRunItem | null>(null);
  const [chatOpen, setChatOpen] = useState(false);
  const [draft, setDraft] = useState<Draft>({ content: "", marks: [], completeness: 0 });
  const [runId, setRunId] = useState<string | null>(null);
  const [isStreaming, setIsStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
      setCurrentPage("board");
      setRunId(null);
      setSelectedTask(null);
      setChatOpen(false);
    };
    window.addEventListener(AUTH_LOGOUT_EVENT, onLogout);
    return () => window.removeEventListener(AUTH_LOGOUT_EVENT, onLogout);
  }, []);

  const handleAuthenticated = useCallback(() => {
    setAuthState("authenticated");
    setCurrentPage("board");
  }, []);

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
  }, []);

  const handleTrigger = useCallback(async (newRunId: string) => {
    setRunId(newRunId);
    setDraft({ content: "", marks: [], completeness: 0 });
    setIsStreaming(true);
    setError(null);
    setCurrentPage("context");
  }, []);

  const handleConfirmed = useCallback(() => {
    setCurrentPage("delivery");
  }, []);

  const handleAddTask = useCallback(async (text: string) => {
    const res = await triggerAgent(text);
    setRunId(res.run_id);
    setDraft({ content: "", marks: [], completeness: 0 });
    setSelectedTask(null);
    setChatOpen(false);
    setError(null);
  }, []);

  useEffect(() => {
    if (!runId) return;
    const cleanup = streamDraft(
      runId,
      (text) => {
        setDraft((d) => ({ ...d, content: d.content + text }));
      },
      (marks) => {
        setDraft((d) => ({ ...d, marks, completeness: 1 }));
        setIsStreaming(false);
      },
      (err) => {
        setError(`SSE 错误: ${err.message}`);
        setIsStreaming(false);
      }
    );
    return cleanup;
  }, [runId]);

  const navigate = (page: PageKey | number) => {
    if (typeof page === "number") {
      setCurrentPage(PAGE_MAP[page] || "board");
    } else {
      setCurrentPage(page);
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

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && chatOpen) {
        handleCloseChat();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
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
            runId={runId}
            onNavigate={navigate}
          />
        );
      case "draft":
        return (
          <DraftEditor
            draft={draft}
            runId={runId}
            onConfirmed={handleConfirmed}
            onNavigate={navigate}
          />
        );
      case "delivery":
        return (
          <Delivery
            draft={draft}
            runId={runId}
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
      chatPanel={chatPanel}
    >
      {renderPage()}
    </Layout>
  );
}
