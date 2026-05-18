"use client";

import { useCallback, useTransition, useMemo } from "react";
import { Modal, Form, Input, Select, Button, Alert, Flex } from "antd";
import { MailOutlined } from "@ant-design/icons";
import { sendInviteAction } from "@/app/dashboard/team/actions";
import { useState } from "react";

interface Props {
  open: boolean;
  onClose: () => void;
}

const ROLES = [
  { value: "admin", label: "Admin — Full access" },
  { value: "member", label: "Member — Standard access" },
  { value: "viewer", label: "Viewer — Read-only access" },
];

export function InviteModal({ open, onClose }: Props) {
  const [form] = Form.useForm();
  const [apiError, setApiError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const handleClose = useCallback(() => {
    form.resetFields();
    setApiError(null);
    onClose();
  }, [form, onClose]);

  const handleFinish = useCallback(
    (values: { email: string; role: string }) => {
      setApiError(null);
      startTransition(async () => {
        const result = await sendInviteAction(values.email, values.role);
        if (result?.error) {
          setApiError(result.error);
        } else {
          handleClose();
        }
      });
    },
    [handleClose]
  );

  const title = useMemo(() => "Invite Team Member", []);

  return (
    <Modal
      open={open}
      onCancel={handleClose}
      title={title}
      footer={null}
      width={440}
      destroyOnHidden
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={handleFinish}
        initialValues={{ role: "member" }}
        style={{ marginTop: 16 }}
      >
        {apiError && (
          <Alert message={apiError} type="error" showIcon style={{ marginBottom: 16 }} />
        )}

        <Form.Item
          name="email"
          label="Email address"
          rules={[
            { required: true, message: "Please enter an email address" },
            { type: "email", message: "Please enter a valid email" },
          ]}
        >
          <Input
            prefix={<MailOutlined style={{ color: "#94a3b8" }} />}
            placeholder="colleague@company.com"
            size="large"
          />
        </Form.Item>

        <Form.Item name="role" label="Role">
          <Select options={ROLES} size="large" />
        </Form.Item>

        <Flex gap={8} style={{ marginTop: 8 }}>
          <Button onClick={handleClose} style={{ flex: 1 }}>
            Cancel
          </Button>
          <Button
            type="primary"
            htmlType="submit"
            loading={isPending}
            style={{ flex: 2 }}
          >
            Send Invitation
          </Button>
        </Flex>
      </Form>
    </Modal>
  );
}
