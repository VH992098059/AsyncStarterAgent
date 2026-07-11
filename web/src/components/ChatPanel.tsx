import { useMemo, useState, useEffect } from "react";
import {
  AssistantRuntimeProvider,
  useLocalRuntime,
  ThreadPrimitive,
  MessagePrimitive,
  MessagePartPrimitive,
  ComposerPrimitive,
} from "@assistant-ui/react";
import { createChatAdapter, loadThreadMessages } from "../adapters/chatAdapter";
import type { AgentRunItem } from "../api/client";
import { IconClose, IconBack, IconSend, IconStop } from "./Icons";

interface ChatPanelProps {
  task: AgentRunItem | null;
  onClose: () => void;
  isMobile: boolean;
}

/**
 * ChatPanel 使用 assistant-ui 组件：
 *  - 使用 AssistantRuntimeProvider 提供运行时上下文
 *  - 使用 useLocalRuntime + ChatModelAdapter 连接后端
 *  - 使用 ThreadPrimitive 和 ComposerPrimitive 构建 UI
 *  - 保持原有暗色主题和翠绿色风格
 */
export function ChatPanel({ task, onClose, isMobile }: ChatPanelProps) {
  const [initialMessages, setInitialMessages] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  // 加载历史消息
  useEffect(() => {
    if (!task) {
      setInitialMessages([]);
      setLoading(false);
      setLoadError(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setLoadError(null);

    loadThreadMessages(task.id)
      .then((msgs) => {
        if (cancelled) return;
        setInitialMessages(msgs);
      })
      .catch((err) => {
        if (cancelled) return;
        setLoadError(`加载历史消息失败: ${err instanceof Error ? err.message : String(err)}`);
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [task?.id]);

  if (!task) return null;

  return (
    <div className="chat-area chat-visible">
      {/* Header */}
      <div className="chat-header">
        {isMobile && (
          <button className="chat-back-btn" onClick={onClose}>
            <IconBack />
          </button>
        )}
        <div className="chat-title">{task.task_type || "任务详情"}</div>
        <button className="chat-close-btn" onClick={onClose} aria-label="关闭">
          <IconClose />
        </button>
      </div>

      {loading && (
        <div className="chat-loading">
          <div className="chat-loading-dots">
            <span />
            <span />
            <span />
          </div>
          <span className="chat-loading-text">加载历史消息...</span>
        </div>
      )}

      {!loading && loadError && (
        <div className="chat-empty">
          <div className="chat-empty-text">{loadError}</div>
        </div>
      )}

      {/* key=task.id：任务切换时强制重新挂载，避免 useLocalRuntime 复用旧线程状态 */}
      {!loading && !loadError && (
        <ChatThread key={task.id} runId={task.id} initialMessages={initialMessages} />
      )}
    </div>
  );
}

interface ChatThreadProps {
  runId: string;
  initialMessages: any[];
}

/**
 * ChatThread 独立持有 adapter + runtime。
 * 通过外层 key={task.id} 保证每个任务拿到全新的 useLocalRuntime 实例，
 * 且挂载时 initialMessages 已是加载完成的历史消息（useLocalRuntime 内部
 * 用 useState 懒初始化创建线程仓库，仅在首次挂载时读取 initialMessages）。
 */
function ChatThread({ runId, initialMessages }: ChatThreadProps) {
  const adapter = useMemo(() => createChatAdapter(runId), [runId]);
  const runtime = useLocalRuntime(adapter, { initialMessages });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      {/* Thread (消息列表) */}
      <ThreadPrimitive.Root className="chat-thread">
        <ThreadPrimitive.Viewport className="chat-messages">
          <ThreadPrimitive.Messages
            components={{
              UserMessage: UserMessage,
              AssistantMessage: AssistantMessage,
            }}
          />
          <ThreadPrimitive.Empty>
            <div className="chat-empty">
              <div className="chat-empty-title">开始对话</div>
              <div className="chat-empty-text">
                向 Agent 发送指令，它会基于当前任务上下文回复你。
              </div>
            </div>
          </ThreadPrimitive.Empty>
        </ThreadPrimitive.Viewport>
      </ThreadPrimitive.Root>

      {/* Composer (输入框) */}
      <div className="chat-input-area">
        <ComposerPrimitive.Root className="chat-input-wrapper">
          <ComposerPrimitive.Input
            className="chat-input"
            placeholder="输入指令..."
            rows={1}
          />
          <ComposerPrimitive.Send className="chat-send-btn" aria-label="发送">
            <IconSend size={16} />
          </ComposerPrimitive.Send>
        </ComposerPrimitive.Root>
      </div>
    </AssistantRuntimeProvider>
  );
}

/**
 * 用户消息文本组件
 */
function UserText() {
  return (
    <p className="whitespace-pre-wrap">
      <MessagePartPrimitive.Text />
    </p>
  );
}

/**
 * 助手消息文本组件
 */
function AssistantText() {
  return (
    <p className="whitespace-pre-wrap">
      <MessagePartPrimitive.Text />
    </p>
  );
}

/**
 * 助手推理内容组件（DeepSeek thinking）- 可折叠
 */
function AssistantReasoning() {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="chat-reasoning">
      <button
        className="chat-reasoning-toggle"
        onClick={() => setExpanded(!expanded)}
        type="button"
      >
        <span className="chat-reasoning-label">
          {expanded ? "收起" : "展开"}思考过程
        </span>
        <span className={`chat-reasoning-arrow ${expanded ? "expanded" : ""}`}>
          ▼
        </span>
      </button>
      {expanded && (
        <div className="chat-reasoning-content">
          <MessagePartPrimitive.Text />
        </div>
      )}
    </div>
  );
}

/**
 * 用户消息组件
 */
function UserMessage() {
  return (
    <div className="chat-message user">
      <div className="chat-avatar">U</div>
      <div className="chat-bubble-wrap">
        <div className="chat-bubble">
          <MessagePrimitive.Parts>
            {({ part }) => {
              if (part.type === "text") return <UserText />;
              return null;
            }}
          </MessagePrimitive.Parts>
        </div>
      </div>
    </div>
  );
}

/**
 * 助手消息组件（支持推理内容 + 加载动画）
 * 使用 MessagePrimitive.Parts render function 模式，避免弃用的 useMessage()。
 * assistant-ui 会在 streaming 且无内容时注入 synthetic empty text part
 * （type: "text", text: "", status: "running"），借此显示加载动画。
 */
function AssistantMessage() {
  return (
    <div className="chat-message agent">
      <div className="chat-avatar">A</div>
      <div className="chat-bubble-wrap">
        <div className="chat-bubble">
          <MessagePrimitive.Parts>
            {({ part }) => {
              if (part.type === "text") {
                // synthetic empty text part: streaming 但还没收到内容
                if (part.text === "" && part.status?.type === "running") {
                  return <LoadingDots />;
                }
                return <AssistantText />;
              }
              if (part.type === "reasoning") return <AssistantReasoning />;
              return null;
            }}
          </MessagePrimitive.Parts>
        </div>
      </div>
    </div>
  );
}

/**
 * 加载动画三点组件
 */
function LoadingDots() {
  return (
    <div className="chat-bubble-loading">
      <span />
      <span />
      <span />
    </div>
  );
}
