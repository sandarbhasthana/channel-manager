"use client";

import { Layout, Dropdown, Typography, Flex } from "antd";
import { UserOutlined, LogoutOutlined, SettingOutlined } from "@ant-design/icons";
import type { MenuProps } from "antd";
import { useRouter } from "next/navigation";
import Avatar, { genConfig } from "react-nice-avatar";

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

export function AppHeader({ 
  userEmail = "admin@channel-manager.com",
  userName = "Admin User"
}: { 
  userEmail?: string;
  userName?: string;
}) {
  const router = useRouter();

  const handleMenuClick: MenuProps["onClick"] = ({ key }) => {
    if (key === "logout") {
      router.push("/api/auth/logout"); 
      console.log("Logout clicked");
    } else if (key === "settings") {
      router.push("/dashboard/settings");
    } else if (key === "profile") {
      router.push("/dashboard/settings"); 
    }
  };

  const avatarConfig = genConfig(userEmail);

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
