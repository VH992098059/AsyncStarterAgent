import { useEffect, useState, useCallback } from "react";

export type ToastType = "success" | "error" | "info";

export interface Toast {
  id: number;
  title: string;
  message?: string;
  type: ToastType;
  exiting?: boolean;
}

/** 各类型自动消失时长（毫秒）。error 更长以便用户阅读。 */
const DURATION: Record<ToastType, number> = {
  success: 2500,
  error: 5000,
  info: 3000,
};

/** exit 动画时长，需与 index.css 的 .toast-exit 保持一致。 */
const EXIT_ANIMATION_MS = 200;

let toastId = 0;
let toastListeners: ((toasts: Toast[]) => void)[] = [];
let toastsState: Toast[] = [];

function emit() {
  const snapshot = toastsState;
  for (const fn of toastListeners) fn(snapshot);
}

/** showToast 派发一条 toast，返回其 id 供外部手动 dismiss。 */
export function showToast(
  title: string,
  message?: string,
  type: ToastType = "info"
): number {
  const id = ++toastId;
  const toast: Toast = { id, title, message, type };
  toastsState = [...toastsState, toast];
  emit();
  scheduleDismiss(id);
  return id;
}

/** dismissToast 先标记 exiting 触发退出动画，再延时真删。 */
export function dismissToast(id: number): void {
  const toast = toastsState.find((t) => t.id === id);
  if (!toast || toast.exiting) return;
  toastsState = toastsState.map((t) => (t.id === id ? { ...t, exiting: true } : t));
  emit();
  setTimeout(() => {
    toastsState = toastsState.filter((t) => t.id !== id);
    emit();
  }, EXIT_ANIMATION_MS);
}

function scheduleDismiss(id: number): void {
  const toast = toastsState.find((t) => t.id === id);
  if (!toast) return;
  setTimeout(() => dismissToast(id), DURATION[toast.type]);
}

/**
 * useToast 订阅全局 toast 列表，返回 {toasts, dismiss}。
 * ToastContainer 用此 hook 渲染；其它组件通常直接用 showToast 全局函数。
 */
export function useToast() {
  const [toasts, setToasts] = useState<Toast[]>(toastsState);

  useEffect(() => {
    const listener = (next: Toast[]) => setToasts(next);
    toastListeners.push(listener);
    return () => {
      toastListeners = toastListeners.filter((fn) => fn !== listener);
    };
  }, []);

  const dismiss = useCallback((id: number) => dismissToast(id), []);
  return { toasts, dismiss };
}
