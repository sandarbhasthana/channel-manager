"use client";

import { useState } from "react";
import { Layout } from "antd";
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

  return (
    <Layout hasSider style={{ minHeight: "100vh" }}>
      <AppSidebar collapsed={collapsed} onCollapse={setCollapsed} />
      <Layout>
        <AppHeader userEmail={userEmail} userName={userName} />
        <Content style={{ background: "#f8fafc", overflowY: "auto" }}>
          {children}
        </Content>
      </Layout>
    </Layout>
  );
}
