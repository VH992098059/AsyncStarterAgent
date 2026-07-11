import { useCallback, useEffect, useMemo, useState } from "react";
import { login, register, saveAuth } from "../api/auth";
import { useRipple } from "../hooks/useRipple";

interface LoginProps {
  onAuthenticated: (username: string) => void;
}

type Tab = "login" | "register";

interface FieldErrors {
  username?: string;
  password?: string;
  confirmPassword?: string;
}

function friendlyError(message: string, tab: Tab): string {
  const m = message.toLowerCase();
  if (m.includes("invalid username or password")) return "用户名或密码错误，请检查后重试";
  if (m.includes("username already taken") || m.includes("username taken") || m.includes("已被注册")) {
    return "该用户名已被注册，请更换";
  }
  if (m.includes("username length") || m.includes("用户名长度")) return "用户名长度应为 3-64 个字符";
  if (m.includes("password length") || m.includes("密码至少")) return "密码至少需要 6 位";
  if (m.includes("invalid body") || m.includes("请求格式")) return "请求格式不正确，请刷新页面重试";
  if (m.includes("missing bearer") || m.includes("invalid token") || m.includes("登录凭证")) {
    return "登录状态已失效，请重新登录";
  }
  if (m.includes("no user in context") || m.includes("登录已过期")) return "登录已过期，请重新登录";
  if (m.includes("network")) return "网络异常，请检查网络后重试";
  return message;
}

function validateUsername(value: string): string | undefined {
  const v = value.trim();
  if (!v) return "请输入用户名";
  if (v.length < 3) return "用户名至少需要 3 个字符";
  if (v.length > 64) return "用户名不能超过 64 个字符";
  return undefined;
}

function validatePassword(value: string): string | undefined {
  if (!value) return "请输入密码";
  if (value.length < 6) return "密码至少需要 6 位";
  return undefined;
}

export function Login({ onAuthenticated }: LoginProps) {
  const [tab, setTab] = useState<Tab>("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const ripple = useRipple();

  // 主题状态，与 Layout 保持同步
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem("asa-theme");
    return saved ? saved === "dark" : document.documentElement.classList.contains("dark");
  });

  useEffect(() => {
    const handler = () => {
      setDarkMode(document.documentElement.classList.contains("dark"));
    };
    window.addEventListener("theme-change", handler);
    return () => window.removeEventListener("theme-change", handler);
  }, []);

  const toggleTheme = useCallback(() => {
    const next = !darkMode;
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("asa-theme", next ? "dark" : "light");
    setDarkMode(next);
    window.dispatchEvent(new Event("theme-change"));
  }, [darkMode]);

  const isValid = useMemo(() => {
    const errors: FieldErrors = {};
    errors.username = validateUsername(username);
    errors.password = validatePassword(password);
    if (tab === "register" && password !== confirmPassword) {
      errors.confirmPassword = "两次输入的密码不一致";
    }
    return !errors.username && !errors.password && !errors.confirmPassword;
  }, [username, password, confirmPassword, tab]);

  const handleTabChange = (next: Tab) => {
    setTab(next);
    setError(null);
    setFieldErrors({});
    setTouched({});
    setConfirmPassword("");
  };

  const validateAll = (): boolean => {
    const errors: FieldErrors = {};
    errors.username = validateUsername(username);
    errors.password = validatePassword(password);
    if (tab === "register") {
      if (password !== confirmPassword) {
        errors.confirmPassword = "两次输入的密码不一致";
      } else if (!confirmPassword) {
        errors.confirmPassword = "请再次输入密码";
      }
    }
    setFieldErrors(errors);
    setTouched({ username: true, password: true, confirmPassword: true });
    return !errors.username && !errors.password && !errors.confirmPassword;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validateAll()) return;
    setSubmitting(true);
    setError(null);
    try {
      const res = tab === "login"
        ? await login(username.trim(), password)
        : await register(username.trim(), password);
      saveAuth(res);
      onAuthenticated(res.username);
    } catch (err) {
      const raw = err instanceof Error ? err.message : String(err);
      setError(friendlyError(raw, tab));
    } finally {
      setSubmitting(false);
    }
  };

  const inputClass = (hasError: boolean) =>
    `w-full px-4 py-3 rounded-xl border text-sm focus:outline-none transition-colors ${
      darkMode
        ? "bg-zinc-900/50 text-zinc-200 placeholder:text-zinc-600"
        : "bg-white text-zinc-800 placeholder:text-zinc-400"
    } ${
      hasError
        ? "border-red-500/60 focus:border-red-500"
        : darkMode
          ? "border-zinc-800 focus:border-emerald-500/50"
          : "border-zinc-300 focus:border-emerald-500/50"
    }`;

  return (
    <div className="min-h-screen flex items-center justify-center px-4 py-12 relative">
      {/* Theme toggle */}
      <button
        type="button"
        onClick={toggleTheme}
        className={`absolute top-4 right-4 w-9 h-9 rounded-lg flex items-center justify-center transition-colors ${
          darkMode
            ? "bg-zinc-800 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-700"
            : "bg-zinc-100 text-zinc-500 hover:text-zinc-700 hover:bg-zinc-200"
        }`}
        aria-label="切换主题"
      >
        {darkMode ? (
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="5" />
            <line x1="12" y1="1" x2="12" y2="3" />
            <line x1="12" y1="21" x2="12" y2="23" />
            <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
            <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
            <line x1="1" y1="12" x2="3" y2="12" />
            <line x1="21" y1="12" x2="23" y2="12" />
            <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
            <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
          </svg>
        ) : (
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
          </svg>
        )}
      </button>

      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="flex items-center justify-center gap-2.5 mb-8">
          <div className="w-9 h-9 rounded-lg bg-emerald-500 flex items-center justify-center">
            <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round">
              <path d="M2 8h12M8 2v12" />
            </svg>
          </div>
          <span className={`font-semibold text-base tracking-tight ${darkMode ? "text-zinc-100" : "text-zinc-800"}`}>Async Starter</span>
        </div>

        <div className="glass-card card-lift rounded-2xl p-6 md:p-8 animate-fade-in-up">
          {/* Tabs */}
          <div className={`flex items-center gap-1 mb-6 p-1 rounded-xl ${darkMode ? "bg-zinc-900/50" : "bg-zinc-100"}`}>
            <button
              type="button"
              className={`flex-1 py-2 rounded-lg text-sm font-medium btn-press ripple-container transition-colors ${
                tab === "login"
                  ? darkMode ? "bg-zinc-800 text-zinc-100" : "bg-white text-zinc-800 shadow-sm"
                  : darkMode ? "text-zinc-500 hover:text-zinc-300" : "text-zinc-500 hover:text-zinc-700"
              }`}
              onClick={(e) => { ripple(e); handleTabChange("login"); }}
            >
              登录
            </button>
            <button
              type="button"
              className={`flex-1 py-2 rounded-lg text-sm font-medium btn-press ripple-container transition-colors ${
                tab === "register"
                  ? darkMode ? "bg-zinc-800 text-zinc-100" : "bg-white text-zinc-800 shadow-sm"
                  : darkMode ? "text-zinc-500 hover:text-zinc-300" : "text-zinc-500 hover:text-zinc-700"
              }`}
              onClick={(e) => { ripple(e); handleTabChange("register"); }}
            >
              注册
            </button>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4" noValidate>
            <div>
              <label className={`block text-xs font-medium uppercase tracking-wider mb-2 ${darkMode ? "text-zinc-400" : "text-zinc-500"}`}>
                用户名
              </label>
              <input
                type="text"
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value);
                  if (touched.username) {
                    setFieldErrors((prev) => ({ ...prev, username: validateUsername(e.target.value) }));
                  }
                }}
                onBlur={() => {
                  setTouched((prev) => ({ ...prev, username: true }));
                  setFieldErrors((prev) => ({ ...prev, username: validateUsername(username) }));
                }}
                placeholder={tab === "register" ? "3-64 个字符" : "用户名"}
                autoComplete="username"
                className={inputClass(Boolean(touched.username && fieldErrors.username))}
              />
              {touched.username && fieldErrors.username && (
                <p className="mt-1.5 text-xs text-red-400">{fieldErrors.username}</p>
              )}
            </div>
            <div>
              <label className={`block text-xs font-medium uppercase tracking-wider mb-2 ${darkMode ? "text-zinc-400" : "text-zinc-500"}`}>
                密码
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  if (touched.password) {
                    setFieldErrors((prev) => ({ ...prev, password: validatePassword(e.target.value) }));
                  }
                  if (tab === "register" && touched.confirmPassword) {
                    setFieldErrors((prev) => ({
                      ...prev,
                      confirmPassword:
                        e.target.value !== confirmPassword ? "两次输入的密码不一致" : undefined,
                    }));
                  }
                }}
                onBlur={() => {
                  setTouched((prev) => ({ ...prev, password: true }));
                  setFieldErrors((prev) => ({ ...prev, password: validatePassword(password) }));
                }}
                placeholder={tab === "register" ? "至少 6 位" : "密码"}
                autoComplete={tab === "register" ? "new-password" : "current-password"}
                className={inputClass(Boolean(touched.password && fieldErrors.password))}
              />
              {touched.password && fieldErrors.password && (
                <p className="mt-1.5 text-xs text-red-400">{fieldErrors.password}</p>
              )}
            </div>
            {tab === "register" && (
              <div>
                <label className={`block text-xs font-medium uppercase tracking-wider mb-2 ${darkMode ? "text-zinc-400" : "text-zinc-500"}`}>
                  再次输入密码
                </label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => {
                    setConfirmPassword(e.target.value);
                    if (touched.confirmPassword) {
                      setFieldErrors((prev) => ({
                        ...prev,
                        confirmPassword:
                          e.target.value !== password ? "两次输入的密码不一致" : undefined,
                      }));
                    }
                  }}
                  onBlur={() => {
                    setTouched((prev) => ({ ...prev, confirmPassword: true }));
                    setFieldErrors((prev) => ({
                      ...prev,
                      confirmPassword:
                        !confirmPassword
                          ? "请再次输入密码"
                          : confirmPassword !== password
                          ? "两次输入的密码不一致"
                          : undefined,
                    }));
                  }}
                  placeholder="再输入一次以确认"
                  autoComplete="new-password"
                  className={inputClass(Boolean(touched.confirmPassword && fieldErrors.confirmPassword))}
                />
                {touched.confirmPassword && fieldErrors.confirmPassword && (
                  <p className="mt-1.5 text-xs text-red-400">{fieldErrors.confirmPassword}</p>
                )}
              </div>
            )}

            {error && (
              <div className="px-3 py-2.5 rounded-lg bg-red-500/10 border border-red-500/20 text-sm text-red-400">
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={submitting || !isValid}
              className="w-full py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
              onClick={ripple}
            >
              {submitting ? (
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
              ) : null}
              {tab === "login" ? "登录" : "注册并登录"}
            </button>
          </form>

          <div className={`mt-6 pt-5 border-t text-xs text-center ${darkMode ? "border-zinc-800 text-zinc-500" : "border-zinc-200 text-zinc-500"}`}>
            {tab === "login" ? "还没有账号？" : "已有账号？"}
            <button
              type="button"
              className="text-emerald-500 hover:underline ml-1"
              onClick={() => handleTabChange(tab === "login" ? "register" : "login")}
            >
              {tab === "login" ? "立即注册" : "返回登录"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
