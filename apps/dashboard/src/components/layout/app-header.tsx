"use client";

import { Layout, Dropdown, Avatar, Typography, Flex } from "antd";
import { UserOutlined, LogoutOutlined, SettingOutlined } from "@ant-design/icons";
import type { MenuProps } from "antd";

const { Header } = Layout;
const { Text } = Typography;

const userMenuItems: MenuProps["items"] = [
  {
    key: "profile",
    icon: <UserOutlined />,
    label: "Profile",
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
    key: "logout",
    icon: <LogoutOutlined />,
    label: "Sign out",
    danger: true,
  },
];

export function AppHeader() {
  const handleMenuClick: MenuProps["onClick"] = ({ key }) => {
    if (key === "logout") {
      // TODO: Implement logout
      console.log("Logout clicked");
    } else if (key === "settings") {
      // TODO: Navigate to settings
      console.log("Settings clicked");
    } else if (key === "profile") {
      // TODO: Navigate to profile
      console.log("Profile clicked");
    }
  };

  return (
    <Header
      style={{
        background: "#fff",
        padding: "0 24px",
        borderBottom: "1px solid #f0f0f0",
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
              Admin User
            </Text>
            <Text type="secondary" style={{ fontSize: 11, lineHeight: 1.2 }}>
              admin@channel-manager.com
            </Text>
          </Flex>
          <Avatar icon={<UserOutlined />} style={{ backgroundColor: "#2563EB" }} />
        </Flex>
      </Dropdown>
    </Header>
  );
}
