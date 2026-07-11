import { useEffect, useRef } from "react";

/**
 * Combo 语法：
 *   - "Escape" / "Enter" / "s" 等单键 → 监听 e.key（区分大小写，需大写键名用大写）
 *   - "mod+s" → macOS 上为 Cmd+s，其他平台为 Ctrl+s
 *   - "shift+/" → Shift+/（即 ?）
 *   - 多键用 + 连接，顺序不敏感
 *
 * handler 接收原始 KeyboardEvent，可调 e.preventDefault()。
 * deps 用于 handler 闭包刷新（与 useEffect 依赖一致）。
 */
export function useKeyboardShortcut(
  combo: string,
  handler: (event: KeyboardEvent) => void,
  deps: unknown[] = []
): void {
  const handlerRef = useRef(handler);
  useEffect(() => {
    handlerRef.current = handler;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    const parts = combo.toLowerCase().split("+").map((s) => s.trim());
    const key = parts[parts.length - 1];
    const needMod = parts.includes("mod");
    const needCtrl = parts.includes("ctrl");
    const needShift = parts.includes("shift");
    const needAlt = parts.includes("alt");
    const isMac = typeof navigator !== "undefined" && /mac/i.test(navigator.platform);

    const onKeyDown = (e: KeyboardEvent) => {
      const modOk = !needMod || (isMac ? e.metaKey : e.ctrlKey);
      const ctrlOk = !needCtrl || e.ctrlKey;
      const shiftOk = !needShift || e.shiftKey;
      const altOk = !needAlt || e.altKey;
      // e.key 大小写敏感：单字符键用小写比较，命名键（Escape/Enter）用原值比较
      const eKey = e.key.length === 1 ? e.key.toLowerCase() : e.key;
      const targetKey = key.length === 1 ? key.toLowerCase() : key;
      if (eKey === targetKey && modOk && ctrlOk && shiftOk && altOk) {
        handlerRef.current(e);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [combo]);
}
