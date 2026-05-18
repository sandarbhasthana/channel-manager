"use client";

import { ConfigProvider, theme as antdTheme } from "antd";

const { defaultAlgorithm } = antdTheme;

export function AntdProvider({ children }: { children: React.ReactNode }) {
  return (
    <ConfigProvider
      theme={{
        algorithm: defaultAlgorithm,
        token: {
          colorPrimary: "#2563EB",
          colorSuccess: "#10B981",
          colorError: "#EF4444",
          colorWarning: "#F59E0B",
          borderRadius: 10,
          fontFamily: "'Inter', ui-sans-serif, system-ui, sans-serif",
          colorBgContainer: "#ffffff",
          colorBgLayout: "#f8fafc",
        },
        components: {
          Layout: {
            siderBg: "#0F172A",
            triggerBg: "#0F172A",
            bodyBg: "#f8fafc",
          },
          Menu: {
            darkItemBg: "transparent",
            darkSubMenuItemBg: "transparent",
            darkItemSelectedBg: "rgba(37, 99, 235, 0.25)",
            darkItemColor: "rgba(255,255,255,0.6)",
            darkItemSelectedColor: "#ffffff",
            darkItemHoverColor: "#ffffff",
            darkItemHoverBg: "rgba(255,255,255,0.06)",
          },
          Card: {
            paddingLG: 20,
          },
          Switch: {
            colorPrimary: "#10B981",
            colorPrimaryHover: "#059669",
          },
        },
      }}
    >
      {children}
    </ConfigProvider>
  );
}
