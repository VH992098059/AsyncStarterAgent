import { useToast } from "../hooks/useToast";

/**
 * ToastContainer 订阅 useToast 全局 store，渲染所有 toast。
 * - error 类型默认显示关闭按钮
 * - exiting=true 时切换到 .toast-exit 类触发退出动画
 *
 * 位置：fixed top-16 right-4，避开顶栏按钮与搜索框（project_memory 约定）。
 * pointer-events-none 容器 + pointer-events-auto 单条，避免遮挡看板交互。
 */
export function ToastContainer() {
  const { toasts, dismiss } = useToast();

  if (toasts.length === 0) return null;

  return (
    <div className="fixed top-16 right-4 z-[100] flex flex-col gap-2 pointer-events-none">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`pointer-events-auto min-w-[220px] max-w-[320px] rounded-lg border px-3 py-2 shadow-lg backdrop-blur-xl text-sm ${
            t.exiting ? "toast-exit" : "toast-enter"
          } ${
            t.type === "success"
              ? "bg-emerald-500/15 border-emerald-500/25 text-emerald-400"
              : t.type === "error"
              ? "bg-red-500/15 border-red-500/25 text-red-400"
              : "toast-info"
          }`}
        >
          <div className="flex items-start gap-2">
            <div className="flex-1 min-w-0">
              <div className="font-medium">{t.title}</div>
              {t.message && <div className="text-xs opacity-70 mt-0.5 break-words">{t.message}</div>}
            </div>
            {t.type === "error" && (
              <button
                className="shrink-0 opacity-60 hover:opacity-100 transition-opacity"
                onClick={() => dismiss(t.id)}
                aria-label="关闭"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
