"use client";

import { useState, useCallback, useTransition, useOptimistic } from "react";
import { Row, Col, Button, Typography, Empty, Flex, Divider } from "antd";
import { PlusOutlined, ApiOutlined } from "@ant-design/icons";
import { ConnectorCard } from "./connector-card";
import { AddConnectorDialog } from "./add-connector-dialog";
import { type Connection, type ConnectionStatus } from "@/lib/api";
import { toggleConnectionAction, deleteConnectionAction } from "@/app/dashboard/connectors/actions";

const { Title, Text } = Typography;

interface Props {
  initialConnections: Connection[];
}

export function ConnectorGrid({ initialConnections }: Props) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [, startTransition] = useTransition();

  const [optimisticConnections, updateOptimistic] = useOptimistic(
    initialConnections,
    (state: Connection[], { id, status }: { id: string; status: ConnectionStatus }) =>
      state.map((c) => (c.id === id ? { ...c, status } : c))
  );

  const handleToggle = useCallback(
    (connection: Connection) => {
      const next: ConnectionStatus =
        connection.status === "CONNECTION_STATUS_ACTIVE"
          ? "CONNECTION_STATUS_DISABLED"
          : "CONNECTION_STATUS_ACTIVE";

      startTransition(async () => {
        updateOptimistic({ id: connection.id, status: next });
        await toggleConnectionAction(connection.id, connection.status);
      });
    },
    [updateOptimistic]
  );

  const handleDelete = useCallback((id: string) => {
    startTransition(async () => {
      await deleteConnectionAction(id);
    });
  }, []);

  return (
    <>
      {/* ── Page header ── */}
      <div style={{ background: "#fff", padding: "24px 32px", borderBottom: "1px solid #f0f0f0" }}>
        <Flex align="center" justify="space-between">
          <div>
            <Title level={4} style={{ margin: 0 }}>OTA Connectors</Title>
            <Text type="secondary" style={{ fontSize: 13 }}>
              Manage your org-level OTA credentials and activate them per property.
            </Text>
          </div>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            size="large"
            onClick={() => setDialogOpen(true)}
          >
            Add Connector
          </Button>
        </Flex>
      </div>

      {/* ── Content ── */}
      <div style={{ padding: "24px 32px" }}>
        {optimisticConnections.length === 0 ? (
          <Flex vertical align="center" justify="center" style={{ minHeight: 400 }}>
            <Empty
              image={<ApiOutlined style={{ fontSize: 56, color: "#cbd5e1" }} />}
              description={
                <Flex vertical align="center" gap={4}>
                  <Text strong style={{ fontSize: 15, color: "#475569" }}>No connectors yet</Text>
                  <Text type="secondary" style={{ fontSize: 13, maxWidth: 300, textAlign: "center" }}>
                    Add your first OTA connection to start syncing availability, rates, and reservations.
                  </Text>
                </Flex>
              }
            >
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setDialogOpen(true)}>
                Add your first connector
              </Button>
            </Empty>
          </Flex>
        ) : (
          <>
            <Text type="secondary" style={{ fontSize: 12, marginBottom: 16, display: "block" }}>
              {optimisticConnections.length} connector{optimisticConnections.length !== 1 ? "s" : ""} configured
            </Text>
            <Row gutter={[16, 16]}>
              {optimisticConnections.map((connection) => (
                <Col key={connection.id} xs={24} sm={12} lg={8} xl={6}>
                  <ConnectorCard
                    connection={connection}
                    isTogglingPending={false}
                    onToggle={() => handleToggle(connection)}
                    onDelete={() => handleDelete(connection.id)}
                  />
                </Col>
              ))}
            </Row>
          </>
        )}
      </div>

      <Divider style={{ margin: 0 }} />

      <AddConnectorDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </>
  );
}
