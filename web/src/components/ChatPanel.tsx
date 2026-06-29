import { useState, useRef, useEffect } from "react";
import type { AgentRunItem } from "../api/client";
import { IconClose, IconBack, IconSend } from "./Icons";

interface ChatMessage {
  id: string;
  role: "agent" | "user";
  content: string;
}

interface ChatPanelProps {
  task: AgentRunItem | null;
  onClose: () => void;
  isMobile: boolean;
}

export function ChatPanel({ task, onClose, isMobile }: ChatPanelProps) {
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (task) {
      setMessages([
        {
          id: "welcome",
          role: "agent",
          content: `已为你加载任务「${task.task_type || "未命名任务"}」的相关上下文。你可以在这里查看进度、修改草稿或下达新指令。`,
        },
      ]);
      setInput("");
    }
  }, [task]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 120)}px`;
    }
  }, [input]);

  const handleSend = () => {
    if (!input.trim() || !task) return;

    const userMsg: ChatMessage = {
      id: `user-${Date.now()}`,
      role: "user",
      content: input.trim(),
    };

    setMessages((prev) => [...prev, userMsg]);
    setInput("");

    setTimeout(() => {
      const agentMsg: ChatMessage = {
        id: `agent-${Date.now()}`,
        role: "agent",
        content: "收到你的指令，我正在处理中。这是演示回复，后续将对接真实Agent工作流。",
      };
      setMessages((prev) => [...prev, agentMsg]);
    }, 800);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  if (!task) return null;

  return (
    <div className="chat-area chat-visible">
      <div className="chat-header">
        {isMobile && (
          <button className="chat-back-btn" onClick={onClose}>
            <IconBack />
          </button>
        )}
        <div className="chat-title">{task.task_type || "任务详情"}</div>
        <button className="chat-close-btn" onClick={onClose}>
          <IconClose />
        </button>
      </div>

      <div className="chat-messages">
        {messages.map((msg) => (
          <div key={msg.id} className={`chat-message ${msg.role}`}>
            <div className="chat-avatar">
              {msg.role === "agent" ? "A" : "U"}
            </div>
            <div className="chat-bubble">{msg.content}</div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      <div className="chat-input-area">
        <div className="chat-input-wrapper">
          <textarea
            ref={textareaRef}
            className="chat-input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="输入指令..."
            rows={1}
          />
          <button
            className="chat-send-btn"
            onClick={handleSend}
            disabled={!input.trim()}
          >
            <IconSend size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}
