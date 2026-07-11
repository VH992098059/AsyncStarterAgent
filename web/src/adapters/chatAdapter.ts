import type { ChatModelAdapter, ChatModelRunOptions, ChatModelRunResult } from "@assistant-ui/react";
import { getToken } from "../api/client";
import { listMessages, type ChatMessage } from "../api/client";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:3080";

/**
 * 从 assistant-ui 的 content 数组中提取文本
 */
function extractTextFromContent(content: any[]): string {
  if (!Array.isArray(content)) return "";
  const textPart = content.find((part) => part.type === "text");
  return textPart?.text ?? "";
}

/**
 * 创建 ChatModelAdapter 实例
 * 使用 async generator 实现流式更新
 * @param runId - Agent run ID
 */
export function createChatAdapter(runId: string): ChatModelAdapter {
  return {
    async *run({
      messages,
      abortSignal,
    }: ChatModelRunOptions): AsyncGenerator<ChatModelRunResult, void> {
      // 1. 获取最后一条用户消息
      const lastUserMessage = messages.filter((m) => m.role === "user").pop();
      if (!lastUserMessage) {
        throw new Error("No user message found");
      }

      // 2. 提取用户文本
      const userText = extractTextFromContent(lastUserMessage.content as any[]);

      // 3. 发起流式请求
      const tok = getToken();
      const res = await fetch(`${API_BASE}/api/v1/agent-runs/${runId}/chat`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(tok ? { Authorization: `Bearer ${tok}` } : {}),
        },
        body: JSON.stringify({ message: userText }),
        signal: abortSignal,
      });

      if (!res.ok) {
        const text = await res.text();
        let msg = `请求失败: ${res.status}`;
        try {
          const json = JSON.parse(text);
          msg = json.message || json.data?.message || msg;
        } catch {
          // 非 JSON 响应
        }
        throw new Error(msg);
      }

      if (!res.body) {
        throw new Error("响应体为空");
      }

      // 4. 解析 SSE 流
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      let fullText = "";
      let fullReasoning = "";

      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });

          // SSE 事件以 \n\n 分隔
          let sepIdx: number;
          while ((sepIdx = buffer.indexOf("\n\n")) >= 0) {
            const rawEvent = buffer.slice(0, sepIdx);
            buffer = buffer.slice(sepIdx + 2);

            // 提取 data: 行
            let dataLine = "";
            for (const line of rawEvent.split("\n")) {
              if (line.startsWith("data: ")) {
                dataLine += line.slice(6);
              }
            }
            if (!dataLine) continue;

            try {
              const data = JSON.parse(dataLine);
              switch (data.type) {
                case "chat_delta":
                  fullText += data.text ?? "";
                  // yield 流式更新（包含已累积的推理内容）
                  yield {
                    content: [
                      ...(fullReasoning ? [{ type: "reasoning" as const, text: fullReasoning }] : []),
                      { type: "text" as const, text: fullText },
                    ],
                  };
                  break;
                case "chat_reasoning":
                  // 累积推理内容，避免每个 chunk 产生独立 part
                  fullReasoning += data.text ?? "";
                  yield {
                    content: [
                      { type: "reasoning" as const, text: fullReasoning },
                      { type: "text" as const, text: fullText },
                    ],
                  };
                  break;
                case "chat_complete":
                  // 流结束，返回最终结果
                  yield {
                    content: [{ type: "text", text: fullText }],
                    status: { type: "complete", reason: "stop" },
                  };
                  return;
                case "chat_error":
                  throw new Error(data.message ?? "chat error");
              }
            } catch (err) {
              if (err instanceof SyntaxError) {
                // JSON 解析错误，忽略
                continue;
              }
              throw err;
            }
          }
        }
      } finally {
        reader.releaseLock();
      }

      // 5. 如果流正常结束但没有 chat_complete 事件，返回最终结果
      yield {
        content: [{ type: "text", text: fullText }],
        status: { type: "complete", reason: "stop" },
      };
    },
  };
}

/**
 * 加载历史消息并转换为 assistant-ui 格式
 */
export async function loadThreadMessages(runId: string): Promise<any[]> {
  const messages = await listMessages(runId);
  // status 字段只允许出现在 assistant 消息上（assistant-ui 会对 user
  // 消息带 status 直接抛错），user 消息不设置该字段。
  return messages.map((msg) => ({
    id: msg.id,
    role: msg.role,
    content: [
      ...(msg.reasoning_content ? [{ type: "reasoning" as const, text: msg.reasoning_content }] : []),
      { type: "text" as const, text: msg.content },
    ],
    createdAt: new Date(msg.created_at),
    ...(msg.role === "assistant" && {
      status:
        msg.status === "done"
          ? { type: "complete" as const }
          : msg.status === "error"
            ? { type: "incomplete" as const, reason: "error" as const }
            : msg.status === "streaming"
              ? { type: "running" as const }
              : { type: "complete" as const },
    }),
  }));
}
