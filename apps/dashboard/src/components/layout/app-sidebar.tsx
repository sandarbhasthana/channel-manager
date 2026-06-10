"use client";

import React, { useState, useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { ConfigProvider, Layout, Menu, Badge, Avatar, Button, Input, Tooltip, theme as antTheme, type MenuProps } from "antd";
import {
  AppstoreOutlined,
  CalendarOutlined,
  BookOutlined,
  TagsOutlined,
  HomeOutlined,
  FolderOutlined,
  TeamOutlined,
  ApiOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import { useTheme } from "../antd-provider";

import "./app-sidebar.css";

const getTimeAgo = (timestamp: number) => {
  const seconds = Math.floor((Date.now() - timestamp) / 1000);
  if (seconds < 15) return "Just Now";
  if (seconds < 60) return `${seconds} sec ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  return `${hours} hr ago`;
};

const { Sider } = Layout;
const ACCENT = "#2563eb";

const themeFor = (dark: boolean) => ({
  algorithm: dark ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
  token: {
    colorPrimary: ACCENT,
    borderRadius: 10,
    fontFamily: '"Geist", system-ui, -apple-system, sans-serif',
    colorBgContainer: dark ? "#0b1322" : "#ffffff",
    colorBorderSecondary: dark ? "#18253f" : "#eef1f6",
  },
  components: {
    Layout: {
      siderBg: dark ? "#0b1322" : "#ffffff",
    },
    Menu: {
      itemBg: "transparent",
      subMenuItemBg: "transparent",
      itemHeight: 40,
      itemMarginInline: 14,
      itemMarginBlock: 3,
      itemBorderRadius: 10,
      iconSize: 17,
      collapsedIconSize: 18,
      itemColor: dark ? "#8d9ab6" : "#46556e",
      itemHoverColor: dark ? "#e8edf7" : "#0f1c33",
      itemHoverBg: dark ? "rgba(255,255,255,0.045)" : "#f1f4f9",
      itemSelectedColor: dark ? "#7caaf9" : ACCENT,
      itemSelectedBg: dark ? "rgba(37,99,235,0.17)" : "#e9f0fe",
      groupTitleColor: dark ? "#5b6987" : "#9aa6bc",
      groupTitleFontSize: 10.5,
      activeBarBorderWidth: 0,
    },
    Input: {
      colorBgContainer: dark ? "#101a2e" : "#f4f6fa",
      colorBorder: dark ? "#1e2c49" : "#e7ebf2",
      colorTextPlaceholder: dark ? "#5b6987" : "#9aa6bc",
    },
    Button: {
      colorBgContainer: "transparent",
    },
  },
});

const groupLabel = (txt: string) => (
  <span style={{ letterSpacing: "1.1px", fontWeight: 600, textTransform: "uppercase" }}>{txt}</span>
);

const CollapseIcon = ({ collapsed }: { collapsed: boolean }) => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true" focusable="false" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
    <path d="M21.25 6.72v10.56a2.97 2.97 0 0 1-2.97 2.97H5.72a2.97 2.97 0 0 1-2.97-2.97V6.72a2.97 2.97 0 0 1 2.97-2.97h12.56a2.97 2.97 0 0 1 2.97 2.97"></path>
    <path 
      d="M6.25 7.25v9.5" 
      style={{
        transitionProperty: "transform, translate, scale, rotate",
        transitionDuration: "0.25s",
        transitionTimingFunction: "cubic-bezier(0.4, 0, 0.2, 1)",
        translate: collapsed ? "10.5px 0" : "1px 0"
      }}
    ></path>
  </svg>
);

const Divider = ({ isDark }: { isDark: boolean }) => (
  <div style={{ height: 1, backgroundColor: isDark ? '#18253f' : '#eef1f6', margin: '4px 0px' }} />
);

const navItems = (collapsed: boolean, isDark: boolean): MenuProps['items'] => [
  {
    key: "grp-overview",
    type: "group",
    label: collapsed ? <Divider isDark={isDark} /> : groupLabel("Overview"),
    children: [
      { key: "/dashboard", icon: <AppstoreOutlined />, label: "Dashboard" },
      {
        key: "/dashboard/bookings",
        icon: <BookOutlined />,
        label: (
          <span style={{ display: "flex", alignItems: "center", justifyContent: "space-between", width: "100%" }}>
            Bookings
            <Badge count={8} size="small" style={{ background: ACCENT, boxShadow: "none", fontWeight: 600 }} />
          </span>
        ),
      },
    ],
  },
  {
    key: "grp-inventory",
    type: "group",
    label: collapsed ? <Divider isDark={isDark} /> : groupLabel("Inventory"),
    children: [
      { key: "/dashboard/inventory", icon: <CalendarOutlined />, label: "Calendar" },
      { key: "/dashboard/room-rates", icon: <TagsOutlined />, label: "Rooms & Rates" },
      { key: "/dashboard/properties", icon: <HomeOutlined />, label: "Properties" },
    ],
  },
  {
    key: "grp-distribution",
    type: "group",
    label: collapsed ? <Divider isDark={isDark} /> : groupLabel("Distribution"),
    children: [
      {
        key: "/dashboard/channels",
        icon: <ApiOutlined />,
        label: (
          <span style={{ display: "flex", alignItems: "center", justifyContent: "space-between", width: "100%" }}>
            Channels
            <Badge dot status="success" />
          </span>
        ),
      },
    ],
  },
  {
    key: "grp-organization",
    type: "group",
    label: collapsed ? <Divider isDark={isDark} /> : groupLabel("Organization"),
    children: [
      { key: "/dashboard/groups", icon: <FolderOutlined />, label: "Groups" },
      { key: "/dashboard/team", icon: <TeamOutlined />, label: "Team" },
    ],
  },
];

const flattenKeys = (items: any[] | undefined): string[] => {
  if (!items) return [];
  return items.flatMap((item) => {
    if (item.children) {
      return flattenKeys(item.children);
    }
    return [item.key];
  }).filter(Boolean);
};

interface AppSidebarProps {
  collapsed: boolean;
  onCollapse: (v: boolean) => void;
}

export function AppSidebar({ collapsed, onCollapse }: AppSidebarProps) {
  const pathname = usePathname();
  const router = useRouter();
  const { mode } = useTheme();
  const isDark = mode === "dark";

  const [lastSync, setLastSync] = useState(Date.now() - 4 * 60 * 1000);
  const [syncStatus, setSyncStatus] = useState<'success' | 'warning' | 'error'>('success');
  const [isSyncing, setIsSyncing] = useState(false);
  const [timeText, setTimeText] = useState("4 min ago");

  useEffect(() => {
    setTimeText(getTimeAgo(lastSync));
    const timer = setInterval(() => {
      setTimeText(getTimeAgo(lastSync));
    }, 10000);
    return () => clearInterval(timer);
  }, [lastSync]);

  const handleSync = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isSyncing) return;
    setIsSyncing(true);
    
    setTimeout(() => {
      setIsSyncing(false);
      setLastSync(Date.now());
      
      const rand = Math.random();
      if (rand < 0.7) setSyncStatus('success');
      else if (rand < 0.9) setSyncStatus('warning');
      else setSyncStatus('error');
    }, 1500);
  };

  const allKeys = flattenKeys(navItems(collapsed, isDark));
  const selectedKey =
    allKeys
      .filter((key) => pathname === key || pathname.startsWith(key + "/"))
      .sort((a, b) => b.length - a.length)[0] ?? "/dashboard";

  const border = isDark ? "1px solid #18253f" : "1px solid #eef1f6";
  const themeClass = isDark ? "dark-theme" : "light-theme";

  return (
    <ConfigProvider theme={themeFor(isDark)}>
      <Sider
        width={252}
        collapsedWidth={76}
        collapsed={collapsed}
        trigger={null}
        style={{
          height: "100vh",
          position: "sticky",
          top: 0,
          borderRight: border,
          display: "flex",
          flexDirection: "column",
        }}
        className={themeClass}
      >
        <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
          {/* brand */}
          <div className="sb-brand" style={collapsed ? { justifyContent: "center", padding: "20px 0 16px" } : undefined}>
            <div className="sb-mark"></div>
            {!collapsed && (
              <div style={{ minWidth: 0 }}>
                <div className="sb-brand-name">Channel Manager</div>
              </div>
            )}
          </div>

          {/* search */}

          {/* nav */}
          <div style={{ flex: 1, overflowY: "auto", overflowX: "hidden", paddingTop: 2 }}>
            <Menu
              mode="inline"
              inlineCollapsed={collapsed}
              selectedKeys={[selectedKey]}
              onClick={(e) => router.push(e.key)}
              items={navItems(collapsed, isDark)}
              style={{ border: "none", background: "transparent" }}
            />
          </div>

          {/* sync status */}
          {collapsed ? (
            <Tooltip placement="right" title={syncStatus === 'success' ? `All channels synced · ${timeText}` : syncStatus === 'warning' ? 'Sync warnings' : 'Sync error'}>
              <div 
                style={{ 
                  display: "flex", 
                  justifyContent: "center", 
                  padding: "16px 0", 
                  cursor: "pointer",
                  color: isDark ? "#5b6987" : "#64748b",
                  transition: "color 0.2s"
                }}
                onClick={handleSync}
                onMouseEnter={(e) => e.currentTarget.style.color = isDark ? "#e8edf7" : "#0f1c33"}
                onMouseLeave={(e) => e.currentTarget.style.color = isDark ? "#5b6987" : "#64748b"}
              >
                <div style={{ position: "relative", display: "flex" }}>
                  <SyncOutlined 
                    spin={isSyncing} 
                    style={{ fontSize: 16 }} 
                  />
                  <span 
                    style={{ 
                      position: "absolute", 
                      top: -3, 
                      right: -4, 
                      width: 10, 
                      height: 10, 
                      borderRadius: "50%",
                      backgroundColor: syncStatus === 'success' ? '#34d399' : syncStatus === 'warning' ? '#fbbf24' : '#f87171',
                      border: `2px solid ${isDark ? "#0b1322" : "#ffffff"}`,
                    }} 
                  />
                </div>
              </div>
            </Tooltip>
          ) : (
            <div className={`sync-card ${syncStatus} ${isSyncing ? 'syncing' : ''}`} onClick={handleSync}>
              <span className="sync-dot"></span>
              <div style={{ minWidth: 0 }}>
                <div className="sync-t">
                  {syncStatus === 'success' ? 'All channels synced' : syncStatus === 'warning' ? 'Sync warnings' : 'Sync error'}
                </div>
                <div className="sync-s">
                  {isSyncing ? 'Syncing...' : `Last Push ${timeText}`}
                </div>
              </div>
              <div className="sync-icon-btn">
                <SyncOutlined spin={isSyncing} />
              </div>
            </div>
          )}

          {/* footer: collapse */}
          <div style={{ borderTop: border, padding: "12px 0 12px 22px", display: "flex" }}>
            <Button
              type="text"
              onClick={() => onCollapse(!collapsed)}
              style={{ 
                color: isDark ? "#5b6987" : "#64748b",
                width: 32,
                height: 32,
                padding: 0,
                display: "flex",
                justifyContent: "center",
                alignItems: "center",
              }}
            >
              <CollapseIcon collapsed={collapsed} />
            </Button>
          </div>
        </div>
      </Sider>
    </ConfigProvider>
  );
}
