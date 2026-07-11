import { useCallback, useEffect, useRef } from "react";

export interface PollingOptions {
  /** 轮询间隔毫秒，默认 5000 */
  interval?: number;
  /** 是否启用轮询；false 时暂停。默认 true */
  enabled?: boolean;
  /** mount 时是否立即执行一次。默认 true */
  immediate?: boolean;
}

export interface PollingControls {
  /** 手动触发一次（不影响下一次 tick 时刻） */
  refresh: () => void;
  /** 暂停轮询（不取消已 in-flight 的请求） */
  pause: () => void;
  /** 恢复轮询 */
  resume: () => void;
}

/**
 * usePolling 周期性调用 fn，并在页面不可见时自动暂停。
 * fn 可以是 async；返回 Promise 被吞错（fn 内部应自行处理错误）。
 *
 * - enabled=false 或 document.hidden 时暂停
 * - 切回可见且 enabled=true 时立即 tick 一次再恢复周期
 * - pause/resume 是组件内状态，与 enabled 取并集
 */
export function usePolling(
  fn: () => void | Promise<void>,
  options: PollingOptions = {}
): PollingControls {
  const { interval = 5000, enabled = true, immediate = true } = options;
  const fnRef = useRef(fn);
  const pausedRef = useRef(false);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // 始终保持 fnRef 指向最新 fn，避免闭包陈旧
  useEffect(() => {
    fnRef.current = fn;
  }, [fn]);

  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const tick = useCallback(() => {
    void fnRef.current();
  }, []);

  const start = useCallback(() => {
    clearTimer();
    timerRef.current = setInterval(tick, interval);
  }, [clearTimer, tick, interval]);

  // visibility 变化：隐藏时暂停，可见且未手动暂停且 enabled 时恢复
  useEffect(() => {
    const onVisibility = () => {
      if (document.hidden) {
        clearTimer();
      } else if (enabled && !pausedRef.current) {
        tick(); // 恢复时立即拉一次，避免等满一个 interval
        start();
      }
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, [enabled, clearTimer, tick, start]);

  // enabled / interval 变化时重置
  useEffect(() => {
    if (enabled && !pausedRef.current && !document.hidden) {
      if (immediate) tick();
      start();
    } else {
      clearTimer();
    }
    return clearTimer;
  }, [enabled, interval, immediate, tick, start, clearTimer]);

  const refresh = useCallback(() => {
    tick();
  }, [tick]);

  const pause = useCallback(() => {
    pausedRef.current = true;
    clearTimer();
  }, [clearTimer]);

  const resume = useCallback(() => {
    pausedRef.current = false;
    if (enabled && !document.hidden) {
      tick();
      start();
    }
  }, [enabled, tick, start]);

  return { refresh, pause, resume };
}
