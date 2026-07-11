import { useEffect, type RefObject } from "react";

/**
 * useClickOutside 监听 mousedown/touchstart 事件，当点击发生在 ref 元素之外时触发 handler。
 * 替换原先散落在 Layout UserMenu 与 KanbanBoard 的重复 mousedown 监听逻辑。
 *
 * @param ref 要监听的目标元素 ref
 * @param handler 点击外部时的回调
 * @param extraRefs 额外视为"内部"的元素 ref 列表（如弹出层 ref）
 */
export function useClickOutside(
  ref: RefObject<HTMLElement | null>,
  handler: (event: MouseEvent | TouchEvent) => void,
  extraRefs: RefObject<HTMLElement | null>[] = []
): void {
  useEffect(() => {
    const listener = (event: MouseEvent | TouchEvent) => {
      const target = event.target as Node | null;
      if (!target) return;
      if (ref.current && ref.current.contains(target)) return;
      for (const extra of extraRefs) {
        if (extra.current && extra.current.contains(target)) return;
      }
      handler(event);
    };
    document.addEventListener("mousedown", listener);
    document.addEventListener("touchstart", listener);
    return () => {
      document.removeEventListener("mousedown", listener);
      document.removeEventListener("touchstart", listener);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [handler]);
}
