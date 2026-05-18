"use client";

import { useState, useCallback, useTransition } from "react";
import {
  Table, Tag, Button, Avatar, Typography, Flex, Popconfirm, Tabs,
  Divider,
} from "antd";
import {
  UserAddOutlined, DeleteOutlined, TeamOutlined,
  MailOutlined, CrownOutlined, UserOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import type { TeamMember, TeamInvitation } from "@/lib/team-api";
import { removeMemberAction, revokeInviteAction } from "@/app/dashboard/team/actions";
import { InviteModal } from "./invite-modal";

const { Title, Text } = Typography;

const ROLE_COLOR: Record<string, string> = {
  owner: "gold", admin: "blue", member: "default", viewer: "default",
};
const ROLE_ICON: Record<string, React.ReactNode> = {
  owner: <CrownOutlined />, admin: <CrownOutlined />,
  member: <UserOutlined />, viewer: <UserOutlined />,
};

interface Props {
  initialMembers: TeamMember[];
  initialInvitations: TeamInvitation[];
}

export function TeamManagement({ initialMembers, initialInvitations }: Props) {
  const [inviteOpen, setInviteOpen] = useState(false);
  const [, startTransition] = useTransition();

  const handleRemove = useCallback((membershipId: string) => {
    startTransition(async () => {
      await removeMemberAction(membershipId);
    });
  }, []);

  const handleRevoke = useCallback((invitationId: string) => {
    startTransition(async () => {
      await revokeInviteAction(invitationId);
    });
  }, []);

  const memberColumns: ColumnsType<TeamMember> = [
    {
      title: "Member",
      key: "member",
      render: (_, m) => (
        <Flex align="center" gap={12}>
          <Avatar src={m.avatarUrl} icon={<UserOutlined />} style={{ backgroundColor: "#2563EB" }} />
          <Flex vertical>
            <Text strong style={{ fontSize: 13 }}>{m.fullName || m.email}</Text>
            {m.fullName && <Text type="secondary" style={{ fontSize: 12 }}>{m.email}</Text>}
          </Flex>
        </Flex>
      ),
    },
    {
      title: "Role",
      dataIndex: "role",
      key: "role",
      width: 120,
      render: (role: string) => (
        <Tag icon={ROLE_ICON[role]} color={ROLE_COLOR[role] ?? "default"}>
          {role.charAt(0).toUpperCase() + role.slice(1)}
        </Tag>
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      width: 100,
      render: (status: string) => (
        <Tag color={status === "active" ? "success" : "warning"}>
          {status.charAt(0).toUpperCase() + status.slice(1)}
        </Tag>
      ),
    },
    {
      title: "",
      key: "actions",
      width: 60,
      render: (_, m) => (
        <Popconfirm
          title="Remove this member?"
          description="They will lose access to the organization."
          onConfirm={() => handleRemove(m.id)}
          okText="Remove" okButtonProps={{ danger: true }}
        >
          <Button type="text" size="small" icon={<DeleteOutlined />} danger />
        </Popconfirm>
      ),
    },
  ];

  const inviteColumns: ColumnsType<TeamInvitation> = [
    {
      title: "Email",
      dataIndex: "email",
      key: "email",
      render: (email: string) => (
        <Flex align="center" gap={8}>
          <MailOutlined style={{ color: "#94a3b8" }} />
          <Text>{email}</Text>
        </Flex>
      ),
    },
    {
      title: "Status",
      dataIndex: "state",
      key: "state",
      width: 100,
      render: (state: string) => (
        <Tag color={state === "pending" ? "processing" : "default"}>
          {state.charAt(0).toUpperCase() + state.slice(1)}
        </Tag>
      ),
    },
    {
      title: "Sent",
      dataIndex: "createdAt",
      key: "createdAt",
      width: 140,
      render: (d: string) => <Text type="secondary" style={{ fontSize: 12 }}>{new Date(d).toLocaleDateString()}</Text>,
    },
    {
      title: "",
      key: "actions",
      width: 80,
      render: (_, inv) => inv.state === "pending" ? (
        <Popconfirm title="Revoke this invitation?" onConfirm={() => handleRevoke(inv.id)} okText="Revoke" okButtonProps={{ danger: true }}>
          <Button size="small" danger>Revoke</Button>
        </Popconfirm>
      ) : null,
    },
  ];

  return (
    <>
      <div style={{ background: "#fff", padding: "24px 32px", borderBottom: "1px solid #f0f0f0" }}>
        <Flex align="center" justify="space-between">
          <div>
            <Title level={4} style={{ margin: 0 }}>Team</Title>
            <Text type="secondary" style={{ fontSize: 13 }}>Manage members and invite staff to your organization.</Text>
          </div>
          <Button type="primary" icon={<UserAddOutlined />} size="large" onClick={() => setInviteOpen(true)}>Invite Member</Button>
        </Flex>
      </div>
      <div style={{ padding: "24px 32px" }}>
        <Tabs defaultActiveKey="members" items={[
          { key: "members", label: <><TeamOutlined /> Members ({initialMembers.length})</>,
            children: <Table columns={memberColumns} dataSource={initialMembers} rowKey="id" pagination={false} /> },
          { key: "invitations", label: <><MailOutlined /> Invitations ({initialInvitations.length})</>,
            children: <Table columns={inviteColumns} dataSource={initialInvitations} rowKey="id" pagination={false} /> },
        ]} />
      </div>
      <Divider style={{ margin: 0 }} />
      <InviteModal open={inviteOpen} onClose={() => setInviteOpen(false)} />
    </>
  );
}
