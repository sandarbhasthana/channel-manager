"use client";

import { useState } from "react";
import { Layout, theme } from "antd";
import { AppSidebar } from "./app-sidebar";
import { AppHeader } from "./app-header";

const { Content } = Layout;

export function DashboardShell({ 
  children,
  userEmail = "admin@channel-manager.com",
  userName = "Admin User"
}: { 
  children: React.ReactNode;
  userEmail?: string;
  userName?: string;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const { token } = theme.useToken();

  return (
    <Layout hasSider style={{ minHeight: "100vh" }}>
      <AppSidebar collapsed={collapsed} onCollapse={setCollapsed} />
      <Layout style={{ background: token.colorBgLayout }}>
        <AppHeader userEmail={userEmail} userName={userName} />
        <Content style={{ background: token.colorBgLayout, overflowY: "auto" }}>
          {children}
        </Content>
      </Layout>
    </Layout>
  );
}
