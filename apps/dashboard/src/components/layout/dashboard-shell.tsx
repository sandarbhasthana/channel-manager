"use client";

import { useState } from "react";
import { Layout } from "antd";
import { AppSidebar } from "./app-sidebar";
import { AppHeader } from "./app-header";

const { Content } = Layout;

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <Layout hasSider style={{ minHeight: "100vh" }}>
      <AppSidebar collapsed={collapsed} onCollapse={setCollapsed} />
      <Layout>
        <AppHeader />
        <Content style={{ background: "#f8fafc", overflowY: "auto" }}>
          {children}
        </Content>
      </Layout>
    </Layout>
  );
}
