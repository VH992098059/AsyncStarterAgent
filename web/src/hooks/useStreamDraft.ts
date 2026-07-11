import { useEffect, useRef } from "react";
import { streamDraft, type Mark } from "../api/client";

/**
 * useStreamDraft — streamDraft 的带重连封装。
 *
 * 背景：streamDraft 内部用 EventSource，浏览器对临时网络错误会自动重连，
 * 但当 readyState===CLOSED（连接彻底断开）时不会再重连，直接回调 onError。
 *
 * 本 hook 在 CLOSED 后手动重建 EventSource，连续 3 次 CLOSED 才彻底失败
 * 上报 onError。这样短暂网络抖动（如切 WiFi、DNS 短暂故障）不会立即让
 * 用户看到错误，提升 SSE 韧性。
 *
 * 参数：
 *  - runId：为 null 时不启动 stream
 *  - restartKey：变化时强制重启 stream（用于手动重试，如点击"重试"按钮）
 *  - onDelta/onComplete/onError：回调，用 ref 保持最新，不触发 effect 重跑
 *
 * 完成后（onComplete）不再重连，即使后续 CLOSED 也不动作。
 */
export function useStreamDraft(
  runId: string | null,
  restartKey: number,
  onDelta: (text: string) => void,
  onComplete: (marks: Mark[]) => void,
  onError: (err: Error) => void
): void {
  const cbRef = useRef({ onDelta, onComplete, onError });
  cbRef.current = { onDelta, onComplete, onError };

  useEffect(() => {
    if (!runId) return;

    const MAX_RETRIES = 3;
    const RECONNECT_DELAY_MS = 500;
    let closedCount = 0;
    let cancelled = false;
    let done = false;
    let cleanup: (() => void) | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    const start = () => {
      if (cancelled || done) return;
      cleanup = streamDraft(
        runId as string,
        (text) => {
          if (done || cancelled) return;
          cbRef.current.onDelta(text);
        },
        (marks) => {
          if (done || cancelled) return;
          done = true;
          cbRef.current.onComplete(marks);
        },
        (err) => {
          if (done || cancelled) return;
          closedCount++;
          if (closedCount >= MAX_RETRIES) {
            cbRef.current.onError(err);
          } else {
            // 短暂延迟后手动重建 EventSource
            reconnectTimer = setTimeout(() => {
              reconnectTimer = null;
              start();
            }, RECONNECT_DELAY_MS);
          }
        }
      );
    };

    start();

    return () => {
      cancelled = true;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      cleanup?.();
      cleanup = null;
    };
  }, [runId, restartKey]);
}
