import { useEffect, useState } from "react";

/**
 * useIsMobile 监听窗口宽度，返回是否为移动端（<= 768px）。
 * 替换原先散落在 App.tsx 与 Layout.tsx 的重复实现。
 */
export function useIsMobile(breakpoint = 768): boolean {
  const [isMobile, setIsMobile] = useState(() =>
    typeof window !== "undefined" ? window.innerWidth <= breakpoint : false
  );

  useEffect(() => {
    const handleResize = () => setIsMobile(window.innerWidth <= breakpoint);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [breakpoint]);

  return isMobile;
}
