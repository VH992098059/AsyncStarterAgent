import { useState, useEffect, useRef, type ReactNode } from "react";
import {
  IconBoard,
  IconSettings,
  IconSearch,
  IconMenu,
  IconSun,
  IconMoon,
  IconUser,
  IconLogout,
  IconChevronDown,
  IconTrigger,
} from "./Icons";

export type PageKey = "board" | "trigger" | "context" | "draft" | "delivery" | "settings";

interface NavItem {
  key: PageKey;
  label: string;
  icon: ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  { key: "board", label: "看板", icon: <IconBoard /> },
];

interface Toast {
  id: number;
  title: string;
  message?: string;
  type: "success" | "error" | "info";
}

let toastId = 0;
let toastListeners: ((toast: Toast) => void)[] = [];

export function showToast(title: string, message?: string, type: "success" | "error" | "info" = "info") {
  const toast: Toast = { id: ++toastId, title, message, type };
  toastListeners.forEach((fn) => fn(toast));
}

function ToastContainer() {
  const [toasts, setToasts] = useState<Toast[]>([]);

  useEffect(() => {
    const listener = (toast: Toast) => {
      setToasts((prev) => [...prev, toast]);
      setTimeout(() => {
        setToasts((prev) => prev.filter((t) => t.id !== toast.id));
      }, 2500);
    };
    toastListeners.push(listener);
    return () => {
      toastListeners = toastListeners.filter((fn) => fn !== listener);
    };
  }, []);

  return (
    <div className="fixed top-16 right-4 z-[100] flex flex-col gap-2 pointer-events-none">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`pointer-events-auto min-w-[220px] max-w-[320px] rounded-lg border px-3 py-2 shadow-lg backdrop-blur-xl toast-enter text-sm ${
            t.type === "success"
              ? "bg-emerald-500/15 border-emerald-500/25 text-emerald-400"
              : t.type === "error"
              ? "bg-red-500/15 border-red-500/25 text-red-400"
              : "toast-info"
          }`}
        >
          <div className="font-medium">{t.title}</div>
          {t.message && <div className="text-xs opacity-70 mt-0.5">{t.message}</div>}
        </div>
      ))}
    </div>
  );
}

interface LayoutProps {
  currentPage: PageKey;
  onNavigate: (page: PageKey) => void;
  chatOpen: boolean;
  onCloseChat?: () => void;
  onLogout?: () => void;
  username?: string;
  children: ReactNode;
  chatPanel: ReactNode;
}

function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 768);
  useEffect(() => {
    const handleResize = () => setIsMobile(window.innerWidth <= 768);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);
  return isMobile;
}

function UserMenu({ username, onLogout }: { username?: string; onLogout?: () => void }) {
  const [open, setOpen] = useState(false);
  const btnRef = useRef<HTMLButtonElement>(null);
  const [dropdownPos, setDropdownPos] = useState<{ top: number; left: number } | null>(null);

  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      const target = e.target as Node;
      if (btnRef.current && btnRef.current.contains(target)) return;
      const dropdown = document.getElementById("user-dropdown-popup");
      if (dropdown && dropdown.contains(target)) return;
      setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  useEffect(() => {
    if (open && btnRef.current) {
      const rect = btnRef.current.getBoundingClientRect();
      setDropdownPos({
        top: Math.max(8, rect.top - 60),
        left: rect.right + 8,
      });
    } else {
      setDropdownPos(null);
    }
  }, [open]);

  const displayName = username || "用户";
  const initial = displayName.charAt(0).toUpperCase();

  return (
    <div className="user-menu-wrapper">
      <button
        ref={btnRef}
        className={`user-avatar-btn ${open ? "open" : ""}`}
        onClick={() => setOpen(!open)}
        aria-label="用户菜单"
      >
        <div className="user-avatar">
          <span>{initial}</span>
        </div>
        <span className="user-name">{displayName}</span>
      </button>
      {open && dropdownPos && (
        <div
          id="user-dropdown-popup"
          className="user-dropdown"
          style={{
            position: "fixed",
            top: dropdownPos.top,
            left: dropdownPos.left,
            animation: "dropdownSlideIn 0.15s var(--ease-out-expo) forwards",
          }}
        >
          <div className="user-dropdown-info">
            <div className="user-dropdown-name">{displayName}</div>
            <div className="user-dropdown-desc">已登录</div>
          </div>
          <div className="user-dropdown-divider" />
          <button
            className="user-dropdown-item"
            onClick={() => {
              setOpen(false);
              onLogout?.();
            }}
          >
            <IconLogout size={16} />
            <span>退出登录</span>
          </button>
        </div>
      )}
    </div>
  );
}

export function Layout({ currentPage, onNavigate, chatOpen, onCloseChat, onLogout, username, children, chatPanel }: LayoutProps) {
  const isMobile = useIsMobile();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem("asa-theme");
    return saved ? saved === "dark" : true;
  });

  useEffect(() => {
    document.documentElement.classList.toggle("dark", darkMode);
    localStorage.setItem("asa-theme", darkMode ? "dark" : "light");
  }, [darkMode]);

  useEffect(() => {
    setSidebarOpen(false);
  }, [currentPage]);

  const handleNavClick = (page: PageKey) => {
    onNavigate(page);
    if (page !== "board" && onCloseChat) onCloseChat();
    if (isMobile) setSidebarOpen(false);
  };

  const showMobileNav = isMobile && currentPage !== "board";

  return (
    <div className={`app-container ${chatOpen ? "chat-open" : ""}`}>
      <ToastContainer />
      <div className="top-bar">
        <div className="top-bar-left">
          <button
            className="icon-btn hamburger-btn"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label="菜单"
          >
            <IconMenu />
          </button>

          <div className="top-bar-logo">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#10b981" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
            <span className="hidden sm:inline">Async Starter Agent</span>
          </div>
        </div>

        <div className="top-bar-search">
          <div className="top-bar-search-inner">
            <div className="top-bar-search-icon">
              <IconSearch />
            </div>
            <input
              type="text"
              placeholder="输入指令快速触发 Agent..."
            />
          </div>
        </div>

        <div className="top-bar-actions">
          <button className="icon-btn" onClick={() => setDarkMode(!darkMode)} aria-label="切换主题">
            {darkMode ? <IconSun /> : <IconMoon />}
          </button>
        </div>
      </div>

      {sidebarOpen && isMobile && (
        <div className="sidebar-overlay show" onClick={() => setSidebarOpen(false)} />
      )}

      <div className="main-content">
        <nav className={`sidebar ${sidebarOpen ? "show" : ""}`}>
          <div className="sidebar-nav">
            {NAV_ITEMS.map((item) => (
              <button
                key={item.key}
                className={`sidebar-item ${currentPage === item.key ? "active" : ""}`}
                onClick={() => handleNavClick(item.key)}
                title={item.label}
              >
                {item.icon}
                <span>{item.label}</span>
              </button>
            ))}
          </div>

          <div className="sidebar-bottom">
            <button
              className={`sidebar-item ${currentPage === "trigger" ? "active" : ""}`}
              onClick={() => handleNavClick("trigger")}
              title="触发规则配置"
            >
              <IconTrigger size={20} />
              <span>规则</span>
            </button>
            <button
              className={`sidebar-item ${currentPage === "settings" ? "active" : ""}`}
              onClick={() => handleNavClick("settings")}
              title="设置"
            >
              <IconSettings size={20} />
              <span>设置</span>
            </button>
            <div className="sidebar-divider" />
            <UserMenu username={username} onLogout={onLogout} />
          </div>
        </nav>

        {currentPage === "board" ? (
          <>
            <div className="kanban-area">{children}</div>
            {chatPanel}
          </>
        ) : (
          <div className="page-content">{children}</div>
        )}
      </div>

      {showMobileNav && (
        <div className="mobile-bottom-nav">
          {NAV_ITEMS.map((item) => (
            <button
              key={item.key}
              className={`mobile-bottom-nav-item ${currentPage === item.key ? "active" : ""}`}
              onClick={() => handleNavClick(item.key)}
            >
              {item.icon}
              <span>{item.label}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

export type { LayoutProps };
