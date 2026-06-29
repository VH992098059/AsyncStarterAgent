import { useCallback } from "react";

export function useRipple() {
  return useCallback((e: React.MouseEvent<HTMLElement>) => {
    const target = e.currentTarget;
    if (!target.classList.contains("ripple-container")) return;

    const rect = target.getBoundingClientRect();
    const size = Math.max(rect.width, rect.height);
    const x = e.clientX - rect.left - size / 2;
    const y = e.clientY - rect.top - size / 2;

    const ripple = document.createElement("span");
    ripple.className = "ripple";
    ripple.style.width = ripple.style.height = size + "px";
    ripple.style.left = x + "px";
    ripple.style.top = y + "px";

    target.appendChild(ripple);
    setTimeout(() => ripple.remove(), 400);
  }, []);
}
