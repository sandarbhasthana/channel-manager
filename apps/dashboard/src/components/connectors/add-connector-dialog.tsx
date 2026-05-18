"use client";

import { useState, useCallback, useTransition, useMemo } from "react";
import { Modal, Form, Input, Button, Card, Flex, Typography, Alert } from "antd";
import { ArrowLeftOutlined } from "@ant-design/icons";
import { OtaLogo, OTA_DISPLAY } from "./ota-logo";
import { type ChannelKind } from "@/lib/api";
import { createConnectionAction } from "@/app/dashboard/connectors/actions";

const { Text } = Typography;

const ALL_KINDS: ChannelKind[] = [
  "CHANNEL_KIND_AIRBNB",
  "CHANNEL_KIND_BOOKING_COM",
  "CHANNEL_KIND_EXPEDIA",
  "CHANNEL_KIND_AGODA",
  "CHANNEL_KIND_DIRECT",
  "CHANNEL_KIND_UNSPECIFIED",
];

interface Props {
  open: boolean;
  onClose: () => void;
}

export function AddConnectorDialog({ open, onClose }: Props) {
  const [step, setStep] = useState<"pick" | "configure">("pick");
  const [selectedKind, setSelectedKind] = useState<ChannelKind | null>(null);
  const [apiError, setApiError] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();
  const [form] = Form.useForm();

  const handleClose = useCallback(() => {
    setStep("pick");
    setSelectedKind(null);
    setApiError(null);
    form.resetFields();
    onClose();
  }, [onClose, form]);

  const handlePick = useCallback((kind: ChannelKind) => {
    setSelectedKind(kind);
    setStep("configure");
  }, []);

  const handleFinish = useCallback(
    (values: { name: string; apiKey?: string; apiSecret?: string }) => {
      const fd = new FormData();
      fd.set("kind", selectedKind!);
      fd.set("name", values.name);
      if (values.apiKey) fd.set("apiKey", values.apiKey);
      if (values.apiSecret) fd.set("apiSecret", values.apiSecret);

      startTransition(async () => {
        const result = await createConnectionAction(fd);
        if (result?.error) {
          setApiError(result.error);
        } else {
          handleClose();
        }
      });
    },
    [selectedKind, handleClose]
  );

  const title = useMemo(
    () => step === "pick"
      ? "Choose a connector"
      : `Configure ${selectedKind ? OTA_DISPLAY[selectedKind] : ""}`,
    [step, selectedKind]
  );

  return (
    <Modal
      open={open}
      onCancel={handleClose}
      title={title}
      footer={null}
      width={480}
      destroyOnHidden
    >
      {step === "pick" ? (
        <Flex wrap gap={12} style={{ marginTop: 8 }}>
          {ALL_KINDS.map((kind) => (
            <Card
              key={kind}
              hoverable
              onClick={() => handlePick(kind)}
              style={{ width: "calc(33.33% - 8px)", cursor: "pointer", textAlign: "center" }}
              styles={{ body: { padding: "16px 8px" } }}
            >
              <Flex vertical align="center" gap={8}>
                <OtaLogo kind={kind} size={40} />
                <Text style={{ fontSize: 11, fontWeight: 500 }}>
                  {kind === "CHANNEL_KIND_UNSPECIFIED" ? "Custom" : OTA_DISPLAY[kind]}
                </Text>
              </Flex>
            </Card>
          ))}
        </Flex>
      ) : (
        <Form form={form} layout="vertical" onFinish={handleFinish} style={{ marginTop: 8 }}>
          {apiError && <Alert message={apiError} type="error" showIcon style={{ marginBottom: 16 }} />}

          <Flex align="center" gap={10} style={{ marginBottom: 20 }}>
            <OtaLogo kind={selectedKind!} size={36} />
            <Text strong>{OTA_DISPLAY[selectedKind!]}</Text>
          </Flex>

          <Form.Item name="name" label="Account name" rules={[{ required: true, message: "Please enter an account name" }]}>
            <Input placeholder={`e.g. Main ${selectedKind ? OTA_DISPLAY[selectedKind] : ""} Account`} />
          </Form.Item>

          <Form.Item name="apiKey" label="API Key">
            <Input.Password placeholder="Paste your API key" autoComplete="off" />
          </Form.Item>

          <Form.Item name="apiSecret" label={<>API Secret <Text type="secondary" style={{ fontSize: 12 }}>(optional)</Text></>}>
            <Input.Password placeholder="Paste your API secret" autoComplete="new-password" />
          </Form.Item>

          <Flex gap={8}>
            <Button icon={<ArrowLeftOutlined />} onClick={() => setStep("pick")} style={{ flex: 1 }}>
              Back
            </Button>
            <Button type="primary" htmlType="submit" loading={isPending} style={{ flex: 2 }}>
              Add connector
            </Button>
          </Flex>
        </Form>
      )}
    </Modal>
  );
}
