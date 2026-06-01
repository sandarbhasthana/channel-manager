"use client";

import { useState, useEffect } from "react";
import {
  Typography,
  Row,
  Col,
  Card,
  Empty,
  Tag,
  Flex,
  Segmented,
  Table,
  Dropdown,
  Button,
  Modal,
  Form,
  Input,
  Select,
} from "antd";
import {
  HomeOutlined,
  GlobalOutlined,
  DollarOutlined,
  AppstoreOutlined,
  UnorderedListOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
} from "@ant-design/icons";
import { type Property, type RoomType } from "@/lib/api";
import { fetchRoomTypesAction } from "./actions";

const { Title, Text } = Typography;

interface Props {
  properties: Property[];
}

export function PropertiesGrid({ properties: initialProperties }: Props) {
  const [properties, setProperties] = useState<Property[]>(initialProperties);
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [removeModalOpen, setRemoveModalOpen] = useState(false);
  const [roomsModalOpen, setRoomsModalOpen] = useState(false);
  const [roomsLoading, setRoomsLoading] = useState(false);
  const [roomsData, setRoomsData] = useState<RoomType[]>([]);
  const [selectedProperty, setSelectedProperty] = useState<Property | null>(null);
  
  const [editForm] = Form.useForm();
  const [modal, contextHolder] = Modal.useModal();

  // Sync state if initialProperties changes
  useEffect(() => {
    setProperties(initialProperties);
  }, [initialProperties]);

  const handleAction = async (action: "edit" | "remove" | "rooms", property: Property) => {
    setSelectedProperty(property);
    if (action === "edit") {
      editForm.setFieldsValue({
        name: property.name,
        externalId: property.externalId || property.external_id,
        currency: property.currency || "USD",
        timezone: property.timezone || "UTC",
      });
      setEditModalOpen(true);
    } else if (action === "remove") {
      setRemoveModalOpen(true);
    } else if (action === "rooms") {
      setRoomsModalOpen(true);
      setRoomsLoading(true);
      try {
        const data = await fetchRoomTypesAction(property.id);
        setRoomsData(data);
      } finally {
        setRoomsLoading(false);
      }
    }
  };

  const handleEditSave = () => {
    editForm.validateFields().then((values) => {
      // Optimistically update local state
      setProperties((prev) =>
        prev.map((p) =>
          p.id === selectedProperty?.id
            ? {
                ...p,
                name: values.name,
                currency: values.currency,
                timezone: values.timezone,
              }
            : p
        )
      );
      
      modal.success({
        title: "Property Updated",
        content: `Property "${values.name}" has been updated successfully!`,
      });
      setEditModalOpen(false);
      setSelectedProperty(null);
    });
  };

  const handleRemoveConfirm = () => {
    if (!selectedProperty) return;
    
    // Optimistically update local state
    setProperties((prev) => prev.filter((p) => p.id !== selectedProperty.id));
    
    modal.success({
      title: "Property Removed",
      content: `Property "${selectedProperty.name}" has been removed from Channel Manager!`,
    });
    setRemoveModalOpen(false);
    setSelectedProperty(null);
  };

  return (
    <>
      {contextHolder}
      {/* ── Page header ── */}
      <div style={{ background: "#fff", padding: "24px 32px", borderBottom: "1px solid #f0f0f0" }}>
        <Flex align="center" justify="space-between">
          <div>
            <Title level={4} style={{ margin: 0 }}>Properties</Title>
            <Text type="secondary" style={{ fontSize: 13 }}>
              View and manage properties synced from your PMS connections.
            </Text>
          </div>
        </Flex>
      </div>

      {/* ── Content ── */}
      <div style={{ padding: "24px 32px" }}>
        {properties.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<Text type="secondary">No properties synced yet.</Text>}
          />
        ) : (
          <>
            {/* ── Controls Row ── */}
            <Flex justify="space-between" align="center" style={{ marginBottom: 16 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {properties.length} propert{properties.length !== 1 ? "ies" : "y"} synced
              </Text>
              <Segmented
                options={[
                  { value: "grid", icon: <AppstoreOutlined /> },
                  { value: "list", icon: <UnorderedListOutlined /> },
                ]}
                value={viewMode}
                onChange={(val) => setViewMode(val as "grid" | "list")}
              />
            </Flex>

            {/* ── Views ── */}
            {viewMode === "grid" ? (
              <Row gutter={[24, 24]}>
                {properties.map((prop) => (
                  <Col xs={24} sm={12} lg={8} key={prop.id}>
                    <Card
                      title={
                        <Flex align="center" gap={12}>
                          <HomeOutlined style={{ color: "#1677ff", fontSize: 20 }} />
                          <span>{prop.name}</span>
                        </Flex>
                      }
                      extra={
                        <Dropdown
                          menu={{
                            items: [
                              {
                                key: "rooms",
                                label: "View Rooms",
                                icon: <AppstoreOutlined />,
                                onClick: () => handleAction("rooms", prop),
                              },
                              {
                                key: "edit",
                                label: "Edit",
                                icon: <EditOutlined />,
                                onClick: () => handleAction("edit", prop),
                              },
                              {
                                key: "remove",
                                label: "Remove",
                                icon: <DeleteOutlined />,
                                danger: true,
                                onClick: () => handleAction("remove", prop),
                              },
                            ],
                          }}
                          trigger={["click"]}
                        >
                          <Button type="text" shape="circle" icon={<MoreOutlined />} />
                        </Dropdown>
                      }
                      style={{
                        height: "100%",
                      }}
                    >
                      <Flex vertical gap={12}>
                        <Flex justify="space-between">
                          <Text type="secondary">ID</Text>
                          <Text style={{ fontFamily: "monospace" }}>{prop.externalId || prop.external_id}</Text>
                        </Flex>
                        <Flex justify="space-between">
                          <Text type="secondary">Currency</Text>
                          <Flex align="center" gap={4}>
                            <DollarOutlined style={{ color: "rgba(0, 0, 0, 0.45)" }} />
                            <Text>{prop.currency || "N/A"}</Text>
                          </Flex>
                        </Flex>
                        <Flex justify="space-between">
                          <Text type="secondary">Timezone</Text>
                          <Flex align="center" gap={4}>
                            <GlobalOutlined style={{ color: "rgba(0, 0, 0, 0.45)" }} />
                            <Text>{prop.timezone || "N/A"}</Text>
                          </Flex>
                        </Flex>
                        <Flex justify="space-between">
                          <Text type="secondary">Status</Text>
                          <Tag color="success">Active</Tag>
                        </Flex>
                      </Flex>
                    </Card>
                  </Col>
                ))}
              </Row>
            ) : (
              <Table
                dataSource={properties}
                rowKey="id"
                pagination={{ pageSize: 10 }}
                columns={[
                  {
                    title: "Property Name",
                    dataIndex: "name",
                    key: "name",
                    render: (text) => (
                      <Flex align="center" gap={8}>
                        <HomeOutlined style={{ color: "#1677ff" }} />
                        <span style={{ fontWeight: 500 }}>{text}</span>
                      </Flex>
                    ),
                  },
                  {
                    title: "PMS External ID",
                    dataIndex: "externalId",
                    key: "externalId",
                    render: (_, record) => (
                      <span style={{ fontFamily: "monospace" }}>
                        {record.externalId || record.external_id}
                      </span>
                    ),
                  },
                  {
                    title: "Currency",
                    dataIndex: "currency",
                    key: "currency",
                    render: (text) => text || "N/A",
                  },
                  {
                    title: "Timezone",
                    dataIndex: "timezone",
                    key: "timezone",
                    render: (text) => text || "N/A",
                  },
                  {
                    title: "Status",
                    key: "status",
                    render: () => <Tag color="success">Active</Tag>,
                  },
                  {
                    title: "Action",
                    key: "action",
                    align: "right" as const,
                    render: (_, record) => (
                      <Dropdown
                        menu={{
                          items: [
                            {
                              key: "rooms",
                              label: "View Rooms",
                              icon: <AppstoreOutlined />,
                              onClick: () => handleAction("rooms", record),
                            },
                            {
                              key: "edit",
                              label: "Edit",
                              icon: <EditOutlined />,
                              onClick: () => handleAction("edit", record),
                            },
                            {
                              key: "remove",
                              label: "Remove",
                              icon: <DeleteOutlined />,
                              danger: true,
                              onClick: () => handleAction("remove", record),
                            },
                          ],
                        }}
                        trigger={["click"]}
                      >
                        <Button type="text" shape="circle" icon={<MoreOutlined />} />
                      </Dropdown>
                    ),
                  },
                ]}
              />
            )}
          </>
        )}
      </div>

      {/* ── Edit Modal ── */}
      <Modal
        title="Edit Property"
        open={editModalOpen}
        onCancel={() => {
          setEditModalOpen(false);
          setSelectedProperty(null);
        }}
        okText="Save"
        cancelText="Cancel"
        onOk={handleEditSave}
        destroyOnHidden
      >
        <Form
          form={editForm}
          layout="vertical"
          style={{ marginTop: 16 }}
        >
          <Form.Item
            name="name"
            label="Property Name"
            rules={[{ required: true, message: "Please enter property name" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="externalId"
            label="PMS External ID"
          >
            <Input disabled />
          </Form.Item>
          <Form.Item
            name="currency"
            label="Currency"
            rules={[{ required: true, message: "Please select currency" }]}
          >
            <Select
              options={[
                { value: "USD", label: "USD - US Dollar" },
                { value: "EUR", label: "EUR - Euro" },
                { value: "INR", label: "INR - Indian Rupee" },
                { value: "GBP", label: "GBP - British Pound" },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="timezone"
            label="Timezone"
            rules={[{ required: true, message: "Please select timezone" }]}
          >
            <Select
              options={[
                { value: "UTC", label: "UTC" },
                { value: "America/New_York", label: "America/New_York" },
                { value: "Europe/London", label: "Europe/London" },
                { value: "Asia/Kolkata", label: "Asia/Kolkata" },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* ── Remove Modal ── */}
      <Modal
        title="Remove Property"
        open={removeModalOpen}
        onCancel={() => {
          setRemoveModalOpen(false);
          setSelectedProperty(null);
        }}
        okText="Remove"
        okButtonProps={{ danger: true }}
        cancelText="Cancel"
        onOk={handleRemoveConfirm}
      >
        <div style={{ padding: "16px 0" }}>
          <Text>
            Are you sure you want to remove <strong>{selectedProperty?.name}</strong> from Channel Manager?
          </Text>
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: 13 }}>
              This will un-sync the property, but will not delete any actual records in your source PMS.
            </Text>
          </div>
        </div>
      </Modal>
      {/* ── Rooms Modal ── */}
      <Modal
        title={`Rooms: ${selectedProperty?.name}`}
        open={roomsModalOpen}
        onCancel={() => setRoomsModalOpen(false)}
        footer={null}
        width={800}
      >
        <Table
          dataSource={roomsData}
          rowKey="id"
          loading={roomsLoading}
          pagination={false}
          columns={[
            {
              title: "Name",
              dataIndex: "name",
              key: "name",
            },
            {
              title: "Code",
              dataIndex: "code",
              key: "code",
            },
            {
              title: "Max Occupancy",
              key: "max_occupancy",
              render: (_, record) => record.maxOccupancy || record.max_occupancy || "-",
            },
            {
              title: "External ID",
              key: "external_id",
              render: (_, record) => <Text style={{ fontFamily: "monospace" }}>{record.externalId || record.external_id || "-"}</Text>,
            },
          ]}
          expandable={{
            expandedRowRender: (record) => (
              <Table
                dataSource={record.rooms || []}
                rowKey="id"
                pagination={false}
                size="small"
                columns={[
                  { title: "Room Name / Number", dataIndex: "name", key: "name" },
                  { 
                    title: "External ID", 
                    key: "external_id", 
                    render: (_, r) => <Text style={{ fontFamily: "monospace", fontSize: 12 }}>{r.externalId || r.external_id || "-"}</Text> 
                  },
                  { 
                    title: "Status", 
                    key: "status", 
                    render: (_, r) => (r.isActive || r.is_active ? <Tag color="green">Active</Tag> : <Tag color="default">Inactive</Tag>) 
                  }
                ]}
              />
            ),
            rowExpandable: (record) => !!record.rooms && record.rooms.length > 0,
          }}
        />
      </Modal>
    </>
  );
}
