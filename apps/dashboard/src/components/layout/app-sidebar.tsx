"use client";

import { usePathname, useRouter } from "next/navigation";
import { Layout, Menu } from "antd";
import type { MenuProps } from "antd";
import {
  AppstoreOutlined,
  ApiOutlined,
  HomeOutlined,
  TeamOutlined,
  SettingOutlined,
} from "@ant-design/icons";

const { Sider } = Layout;

const menuItems: MenuProps["items"] = [
  { key: "/dashboard", icon: <AppstoreOutlined />, label: "Dashboard" },
  { key: "/dashboard/connectors", icon: <ApiOutlined />, label: "Connectors" },
  { key: "/dashboard/properties", icon: <HomeOutlined />, label: "Properties" },
  { key: "/dashboard/team", icon: <TeamOutlined />, label: "Team" },
  { key: "/dashboard/settings", icon: <SettingOutlined />, label: "Settings" },
];

function CMLogo({ collapsed }: { collapsed: boolean }) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: collapsed ? "center" : "flex-start",
        gap: collapsed ? 0 : 12,
        padding: collapsed ? "20px 0 16px" : "20px 20px 16px",
        borderBottom: "1px solid rgba(255,255,255,0.08)",
        marginBottom: 8,
        transition: "all 0.2s ease",
      }}
    >
      <svg width="34" height="34" viewBox="0 0 56 56" fill="none" style={{ flexShrink: 0 }}>
        <circle cx="28" cy="28" r="27" stroke="#334155" strokeWidth="2" />
        <path d="M28 4 A24 24 0 0 1 52 28" stroke="#2563EB" strokeWidth="5" strokeLinecap="round" fill="none" />
        <path d="M52 28 A24 24 0 0 1 28 52" stroke="#22C55E" strokeWidth="5" strokeLinecap="round" fill="none" />
        <path d="M28 52 A24 24 0 0 1 4 28" stroke="#F59E0B" strokeWidth="5" strokeLinecap="round" fill="none" />
        <path d="M4 28 A24 24 0 0 1 28 4" stroke="#EF4444" strokeWidth="5" strokeLinecap="round" fill="none" />
        <circle cx="28" cy="28" r="16" fill="#0F172A" />
        <text x="28" y="33" textAnchor="middle" fontSize="11" fontWeight="700" fill="white" fontFamily="Inter, sans-serif">CM</text>
      </svg>
      {!collapsed && (
        <span style={{ color: "white", fontWeight: 600, fontSize: 13, lineHeight: 1.35, whiteSpace: "nowrap" }}>
          Channel<br />Manager
        </span>
      )}
    </div>
  );
}

interface AppSidebarProps {
  collapsed: boolean;
  onCollapse: (v: boolean) => void;
}

export function AppSidebar({ collapsed, onCollapse }: AppSidebarProps) {
  const pathname = usePathname();
  const router = useRouter();

  const selectedKey =
    (menuItems as { key: string }[])
      .map((item) => item.key)
      .filter((key) => pathname === key || pathname.startsWith(key + "/"))
      .sort((a, b) => b.length - a.length)[0] ?? "/dashboard";

  return (
    <Sider
      collapsible
      collapsed={collapsed}
      onCollapse={onCollapse}
      width={230}
      theme="dark"
      style={{ height: "100vh", position: "sticky", top: 0 }}
    >
      <CMLogo collapsed={collapsed} />
      <Menu
        theme="dark"
        mode="inline"
        selectedKeys={[selectedKey]}
        items={menuItems}
        onClick={({ key }) => router.push(key)}
        style={{ border: "none" }}
      />
    </Sider>
  );
}
