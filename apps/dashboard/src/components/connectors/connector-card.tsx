"use client";

import { Card, Switch, Tag, Popconfirm, Typography, Flex } from "antd";
import { DeleteOutlined } from "@ant-design/icons";
import { type Connection, type ConnectionStatus } from "@/lib/api";
import { OtaLogo, OTA_DISPLAY } from "./ota-logo";

const { Text } = Typography;

const STATUS_TAG: Record<ConnectionStatus, { label: string; color: string }> = {
  CONNECTION_STATUS_ACTIVE:      { label: "Active",   color: "success" },
  CONNECTION_STATUS_INACTIVE:    { label: "Inactive", color: "default" },
  CONNECTION_STATUS_DISABLED:    { label: "Disabled", color: "default" },
  CONNECTION_STATUS_ERROR:       { label: "Error",    color: "error"   },
  CONNECTION_STATUS_UNSPECIFIED: { label: "Unknown",  color: "default" },
};

interface ConnectorCardProps {
  connection: Connection;
  isTogglingPending: boolean;
  onToggle: () => void;
  onDelete: () => void;
}

export function ConnectorCard({ connection, isTogglingPending, onToggle, onDelete }: ConnectorCardProps) {
  const isActive = connection.status === "CONNECTION_STATUS_ACTIVE";
  const { label: statusLabel, color: statusColor } =
    STATUS_TAG[connection.status] ?? STATUS_TAG.CONNECTION_STATUS_UNSPECIFIED;
  const displayName = OTA_DISPLAY[connection.kind] ?? "Custom";

  return (
    <Card
      hoverable
      styles={{ body: { padding: 20 } }}
      style={{ opacity: isActive ? 1 : 0.72 }}
      extra={
        <Switch
          checked={isActive}
          loading={isTogglingPending}
          onChange={onToggle}
          size="default"
        />
      }
      actions={[
        <Popconfirm
          key="delete"
          title="Remove this connector?"
          description="This will remove the OTA connection. Property channels linked to it will also be deleted."
          onConfirm={onDelete}
          okText="Remove"
          okButtonProps={{ danger: true }}
          cancelText="Cancel"
        >
          <DeleteOutlined style={{ color: "#94a3b8", fontSize: 15 }} />
        </Popconfirm>,
      ]}
    >
      <Card.Meta
        avatar={<OtaLogo kind={connection.kind} size={44} />}
        title={
          <Text strong style={{ fontSize: 15 }}>
            {displayName}
          </Text>
        }
        description={
          <Flex vertical gap={6} style={{ marginTop: 2 }}>
            <Text
              type="secondary"
              style={{ fontSize: 12, maxWidth: 180, display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
            >
              {connection.name}
            </Text>
            <Tag color={statusColor} style={{ width: "fit-content" }}>
              {statusLabel}
            </Tag>
          </Flex>
        }
      />
    </Card>
  );
}
