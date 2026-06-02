"use client";

import { usePathname, useRouter } from "next/navigation";
import { Layout, Menu, theme, Switch, Flex } from "antd";
import type { MenuProps } from "antd";
import {
  AppstoreOutlined,
  CalendarOutlined,
  BookOutlined,
  TagOutlined,
  HomeOutlined,
  FolderOutlined,
  TeamOutlined,
  SunOutlined,
  MoonOutlined,
} from "@ant-design/icons";
import { useTheme } from "../antd-provider";

const { Sider } = Layout;

const menuItems: MenuProps["items"] = [
  { key: "/dashboard", icon: <AppstoreOutlined />, label: "Dashboard" },
  { key: "/dashboard/inventory", icon: <CalendarOutlined />, label: "Inventory" },
  { key: "/dashboard/bookings", icon: <BookOutlined />, label: "Bookings" },
  { key: "/dashboard/room-rates", icon: <TagOutlined />, label: "Rooms & Rates" },
  { key: "/dashboard/properties", icon: <HomeOutlined />, label: "Properties" },
  { key: "/dashboard/groups", icon: <FolderOutlined />, label: "Groups" },
  { key: "/dashboard/team", icon: <TeamOutlined />, label: "Team" },
];

function CMLogo({ collapsed }: { collapsed: boolean }) {
  const { token } = theme.useToken();
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: collapsed ? "center" : "flex-start",
        gap: collapsed ? 0 : 12,
        padding: "0 20px",
        height: 64, // Exact height of the header
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        borderRight: `1px solid ${token.colorBorderSecondary}`,
        boxSizing: "border-box",
        transition: "all 0.2s ease",
        flexShrink: 0,
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
        <span style={{ color: token.colorTextBase, fontWeight: 600, fontSize: 13, lineHeight: 1.35, whiteSpace: "nowrap" }}>
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
  const { token } = theme.useToken();
  const { mode } = useTheme();
  const isDark = mode === "dark";

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
      style={{ 
        height: "100vh", 
        position: "sticky", 
        top: 0,
        borderRight: `1px solid ${token.colorBorderSecondary}`,
      }}
    >
      <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
        <CMLogo collapsed={collapsed} />
        
        <div style={{ flex: 1, overflowY: "auto", overflowX: "hidden", paddingTop: 8 }}>
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            items={menuItems}
            onClick={({ key }) => router.push(key)}
            style={{ border: "none", background: "transparent" }}
          />
        </div>
      </div>
    </Sider>
  );
}

