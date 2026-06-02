"use client";

import React, { useState } from "react";
import { Layout, Dropdown, Typography, Flex, theme, Segmented } from "antd";
import { UserOutlined, LogoutOutlined, SettingOutlined, LaptopOutlined, SunOutlined, MoonOutlined } from "@ant-design/icons";
import type { MenuProps } from "antd";
import { useRouter } from "next/navigation";
import Avatar, { genConfig } from "react-nice-avatar";
import { useTheme, ThemeMode } from "../antd-provider";

const { Header } = Layout;
const { Text } = Typography;

export function AppHeader({ 
  userEmail = "admin@channel-manager.com",
  userName = "Admin User"
}: { 
  userEmail?: string;
  userName?: string;
}) {
  const router = useRouter();
  const { mode, setMode } = useTheme();
  const { token } = theme.useToken();
  const [clickPos, setClickPos] = useState({ x: 0, y: 0 });

  const handleMenuClick: MenuProps["onClick"] = ({ key }) => {
    if (key === "logout") {
      router.push("/api/auth/logout"); 
    } else if (key === "settings") {
      router.push("/dashboard/settings");
    } else if (key === "profile") {
      router.push("/dashboard/settings"); 
    }
  };

  const handleThemeChange = (newMode: ThemeMode) => {
    // Check if View Transitions API is supported and user prefers motion
    // @ts-ignore - document.startViewTransition is relatively new
    if (!document.startViewTransition || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setMode(newMode);
      return;
    }

    const { x, y } = clickPos;
    const endRadius = Math.hypot(
      Math.max(x, innerWidth - x),
      Math.max(y, innerHeight - y)
    );

    // @ts-ignore
    const transition = document.startViewTransition(() => {
      setMode(newMode);
    });

    transition.ready.then(() => {
      document.documentElement.animate(
        [
          { clipPath: `circle(0px at ${x}px ${y}px)` },
          { clipPath: `circle(${endRadius}px at ${x}px ${y}px)` }
        ],
        {
          duration: 500,
          easing: "ease-in-out",
          pseudoElement: "::view-transition-new(root)"
        }
      );
    });
  };

  const userMenuItems: MenuProps["items"] = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: "My Profile",
    },
    {
      key: "settings",
      icon: <SettingOutlined />,
      label: "Settings",
    },
    {
      type: "divider",
    },
    {
      key: "theme_switcher",
      style: { padding: '8px 12px', cursor: 'default', background: 'transparent' },
      label: (
        <Flex justify="space-between" align="center" style={{ width: 200 }} onClick={e => e.stopPropagation()}>
          <Text style={{ fontSize: 13, fontWeight: 500 }}>Theme</Text>
          <div onMouseDown={(e) => setClickPos({ x: e.clientX, y: e.clientY })}>
            <Segmented
              value={mode}
              onChange={(value) => handleThemeChange(value as ThemeMode)}
              options={[
                { value: 'system', icon: <LaptopOutlined /> },
                { value: 'light', icon: <SunOutlined /> },
                { value: 'dark', icon: <MoonOutlined /> },
              ]}
              size="small"
              style={{ padding: 2 }}
            />
          </div>
        </Flex>
      ),
    },
    {
      type: "divider",
    },
    {
      key: "logout",
      icon: <LogoutOutlined style={{ color: token.colorError }} />,
      label: <span style={{ color: token.colorError }}>Logout</span>,
    },
  ];

  const avatarConfig = genConfig(userEmail);

  return (
    <Header
      style={{
        background: token.colorBgContainer,
        padding: "0 24px",
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        display: "flex",
        alignItems: "center",
        justifyContent: "flex-end",
        height: 64,
      }}
    >
      <Dropdown menu={{ items: userMenuItems, onClick: handleMenuClick }} placement="bottomRight" trigger={["click"]}>
        <Flex align="center" gap={10} style={{ cursor: "pointer", padding: "8px 12px", borderRadius: 8, transition: "background 0.2s" }}>
          <Flex vertical align="flex-end" style={{ marginRight: 4 }}>
            <Text strong style={{ fontSize: 13, lineHeight: 1.2 }}>
              {userName}
            </Text>
            <Text type="secondary" style={{ fontSize: 11, lineHeight: 1.2 }}>
              {userEmail}
            </Text>
          </Flex>
          <div style={{ width: 32, height: 32 }}>
            {/* @ts-ignore - React 19 type mismatch for this package */}
            <Avatar style={{ width: '100%', height: '100%' }} {...avatarConfig} />
          </div>
        </Flex>
      </Dropdown>
    </Header>
  );
}

