"use client";

import React, { useEffect, useState } from "react";
import {
  Typography, Card, Table, Button, Space, Tag, Modal, Form, Input,
  InputNumber, Switch, App, Popconfirm,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  fetchPromoCodes,
  savePromoCodeAction,
  deletePromoCodeAction,
} from "./actions";
import type { PromoCode, PromoCodeInput } from "@/lib/api";

const { Text } = Typography;

export function CouponsManager() {
  const { message } = App.useApp();
  const [codes, setCodes] = useState<PromoCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<PromoCode | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      setCodes(await fetchPromoCodes());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const openNew = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ discountPct: 10, isActive: true, maxUses: 0 });
    setModalOpen(true);
  };

  const openEdit = (c: PromoCode) => {
    setEditing(c);
    form.setFieldsValue({
      code: c.code,
      description: c.description,
      discountPct: c.discountPct,
      maxUses: c.maxUses ?? 0,
      propertyId: c.propertyId ?? "",
      isActive: c.isActive,
    });
    setModalOpen(true);
  };

  const submit = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      const input: PromoCodeInput = {
        id: editing?.id,
        code: values.code,
        description: values.description || undefined,
        discountPct: Number(values.discountPct),
        maxUses: Number(values.maxUses) || 0,
        propertyId: values.propertyId || undefined,
        isActive: values.isActive,
      };
      await savePromoCodeAction(input);
      message.success(editing ? "Coupon updated" : "Coupon created");
      setModalOpen(false);
      load();
    } catch (err) {
      if ((err as { errorFields?: unknown }).errorFields) return; // form validation
      message.error((err as Error).message || "Failed to save coupon");
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await deletePromoCodeAction(id);
      message.success("Coupon deleted");
      load();
    } catch (err) {
      message.error((err as Error).message || "Failed to delete");
    }
  };

  const columns = [
    { title: "CODE", dataIndex: "code", key: "code", render: (v: string) => <Text strong style={{ fontSize: 13 }}>{v}</Text> },
    { title: "DISCOUNT", dataIndex: "discountPct", key: "discount", render: (v: number) => <Text style={{ fontSize: 13 }}>{v}%</Text> },
    {
      title: "USES",
      key: "uses",
      render: (c: PromoCode) => (
        <Text style={{ fontSize: 13 }}>
          {c.uses ?? 0}{c.maxUses && c.maxUses > 0 ? ` / ${c.maxUses}` : " / ∞"}
        </Text>
      ),
    },
    {
      title: "SCOPE",
      key: "scope",
      render: (c: PromoCode) => (
        <Text style={{ fontSize: 13 }}>{c.propertyId ? "Property" : "Org-wide"}</Text>
      ),
    },
    {
      title: "STATUS",
      dataIndex: "isActive",
      key: "status",
      render: (a: boolean) => <Tag color={a ? "green" : "default"}>{a ? "ACTIVE" : "INACTIVE"}</Tag>,
    },
    {
      title: "",
      key: "actions",
      align: "right" as const,
      render: (c: PromoCode) => (
        <Space>
          <Button size="small" type="text" onClick={() => openEdit(c)}>Edit</Button>
          <Popconfirm title="Delete this coupon?" onConfirm={() => remove(c.id)} okText="Delete" okButtonProps={{ danger: true }}>
            <Button size="small" type="text" danger>Delete</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card
      variant="outlined"
      style={{ borderRadius: 16, marginTop: 22 }}
      styles={{ body: { padding: 0 } }}
      title={
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%" }}>
          <span>Coupons</span>
          <Button type="primary" icon={<PlusOutlined />} onClick={openNew} style={{ borderRadius: 10 }}>
            New coupon
          </Button>
        </div>
      }
    >
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={codes}
        pagination={{ pageSize: 8, hideOnSinglePage: true }}
        locale={{ emptyText: "No coupons yet" }}
      />

      <Modal
        title={editing ? "Edit coupon" : "New coupon"}
        open={modalOpen}
        onOk={submit}
        confirmLoading={saving}
        onCancel={() => setModalOpen(false)}
        okText={editing ? "Save" : "Create"}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 12 }}>
          <Form.Item name="code" label="Code" rules={[{ required: true, message: "Code is required" }]}>
            <Input placeholder="SUMMER25" disabled={!!editing} />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input placeholder="Summer campaign" />
          </Form.Item>
          <Form.Item
            name="discountPct"
            label="Discount %"
            rules={[{ required: true, message: "Discount is required" }]}
          >
            <InputNumber min={0.01} max={100} step={1} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="maxUses" label="Max uses (0 = unlimited)">
            <InputNumber min={0} step={1} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="propertyId" label="Property ID (blank = org-wide)">
            <Input placeholder="Leave blank for all properties" />
          </Form.Item>
          <Form.Item name="isActive" label="Active" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
