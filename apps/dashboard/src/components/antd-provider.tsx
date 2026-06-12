"use client";

import { createContext, useContext, useState, useEffect } from "react";
import { ConfigProvider, theme as antdTheme, App } from "antd";

export type ThemeMode = "light" | "dark" | "system";

interface ThemeContextType {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
}

const ThemeContext = createContext<ThemeContextType>({
  mode: "system",
  setMode: () => {},
});

export function useTheme() {
  return useContext(ThemeContext);
}

export function AntdProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>("system");
  const [systemIsDark, setSystemIsDark] = useState(false);

  useEffect(() => {
    const saved = localStorage.getItem("theme_mode") as ThemeMode;
    if (saved) {
      setModeState(saved);
    }

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    setSystemIsDark(mediaQuery.matches);
    
    const handler = (e: MediaQueryListEvent) => setSystemIsDark(e.matches);
    mediaQuery.addEventListener("change", handler);
    return () => mediaQuery.removeEventListener("change", handler);
  }, []);

  const setMode = (newMode: ThemeMode) => {
    setModeState(newMode);
    localStorage.setItem("theme_mode", newMode);
  };

  const isDark = mode === "dark" || (mode === "system" && systemIsDark);

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  }, [isDark]);

  // Dark theme maps exactly to reference variables
  const darkTokens = {
    colorPrimary: "#2563eb",
    colorBgBase: "#090e1a",
    colorBgContainer: "#101a2e",
    colorBgElevated: "#15213a",
    colorBgLayout: "#090e1a",
    colorTextBase: "#e8edf7",
    colorTextSecondary: "#8d9ab6",
    colorTextTertiary: "#5b6987",
    colorBorder: "#1e2c49",
    colorBorderSecondary: "#18253f",
    colorSuccess: "#34d399",
    colorWarning: "#fbbf24",
    colorError: "#f87171",
    colorInfo: "#38bdf8",
  };

  // Light theme designed from scratch for good contrast
  const lightTokens = {
    colorPrimary: "#2563eb",
    colorBgBase: "#f8fafc",
    colorBgContainer: "#ffffff",
    colorBgElevated: "#ffffff",
    colorBgLayout: "#f8fafc",
    colorTextBase: "#0f172a",
    colorTextSecondary: "#475569",
    colorTextTertiary: "#64748b",
    colorBorder: "#e2e8f0",
    colorBorderSecondary: "#f1f5f9",
    colorSuccess: "#10b981",
    colorWarning: "#f59e0b",
    colorError: "#ef4444",
    colorInfo: "#0ea5e9",
  };

  const currentTokens = isDark ? darkTokens : lightTokens;

  return (
    <ThemeContext.Provider value={{ mode, setMode }}>
      <ConfigProvider
        theme={{
          algorithm: isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
          token: {
            ...currentTokens,
            borderRadius: 12,
            fontFamily: "'Geist', 'Inter', system-ui, sans-serif",
          },
          components: {
            Layout: {
              siderBg: isDark ? "#0b1322" : "#ffffff",
              headerBg: isDark ? "rgba(9,14,26,0.7)" : "rgba(255,255,255,0.7)",
              bodyBg: currentTokens.colorBgLayout,
              triggerBg: isDark ? "#0b1322" : "#ffffff",
              triggerColor: currentTokens.colorTextSecondary,
            },
            Menu: {
              darkItemBg: "transparent",
              darkSubMenuItemBg: "transparent",
              darkItemSelectedBg: "rgba(37, 99, 235, 0.25)",
              darkItemColor: "rgba(255,255,255,0.6)",
              darkItemSelectedColor: "#ffffff",
              darkItemHoverColor: "#ffffff",
              darkItemHoverBg: "rgba(255,255,255,0.06)",
              itemBg: "transparent",
              subMenuItemBg: "transparent",
              itemSelectedBg: "rgba(37, 99, 235, 0.1)",
            },
            Card: {
              paddingLG: 22,
              colorBorderSecondary: currentTokens.colorBorder,
            },
            Table: {
              colorBgContainer: currentTokens.colorBgContainer,
              headerBg: currentTokens.colorBgContainer,
              borderColor: currentTokens.colorBorderSecondary,
              rowHoverBg: isDark ? "#15213a" : "#f1f5f9",
            },
          },
        }}
      >
        <App>
          <div style={{ background: currentTokens.colorBgBase, minHeight: "100vh", color: currentTokens.colorTextBase }}>
            {children}
          </div>
        </App>
      </ConfigProvider>
    </ThemeContext.Provider>
  );
}
