"use client";

import React, { useEffect, useState } from "react";
import {
  Typography, Card, Table, Flex, Space, Switch, Tag, Empty, App, Segmented, InputNumber, Button,
} from "antd";
import { GlobalOutlined } from "@ant-design/icons";
import { usePropertyContext } from "../../../components/property-provider";
import {
  fetchDirectReservations,
  fetchBookingEngineSettings,
  saveBookingEngineSettingsAction,
} from "./actions";
import { CouponsManager } from "./CouponsManager";
import type { DirectReservation } from "@/lib/api";

const { Title, Text } = Typography;

type CalDate = { year: number; month: number; day: number } | undefined;

function fmtDate(d: CalDate): string {
  if (!d) return "-";
  const mm = String(d.month).padStart(2, "0");
  const dd = String(d.day).padStart(2, "0");
  return `${mm}/${dd}/${d.year}`;
}

function fmtMoney(total?: { amountMinor: string; currency: string }): string {
  if (!total) return "-";
  const minor = Number(total.amountMinor || "0");
  if (Number.isNaN(minor)) return "-";
  return `${(minor / 100).toFixed(2)} ${total.currency}`;
}

function fmtBookedAt(iso: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString("en-US", { month: "2-digit", day: "2-digit", year: "numeric" });
}

export default function BookingEnginePage() {
  const { message } = App.useApp();
  const { activeProperty, loading: ctxLoading } = usePropertyContext();

  const [reservations, setReservations] = useState<DirectReservation[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [route, setRoute] = useState<"pms" | "cm">("pms");
  const [routePercent, setRoutePercent] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (ctxLoading || !activeProperty) return;
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const [rows, settings] = await Promise.all([
          fetchDirectReservations(activeProperty.id),
          fetchBookingEngineSettings(activeProperty.id),
        ]);
        if (cancelled) return;
        setReservations(rows);
        if (settings) {
          setEnabled(settings.directChannelEnabled);
          setRoute(settings.bookingRoute === "cm" ? "cm" : "pms");
          setRoutePercent(settings.bookingRoutePercent ?? 0);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => {
      cancelled = true;
    };
  }, [activeProperty, ctxLoading]);

  const save = async () => {
    if (!activeProperty) return;
    setSaving(true);
    try {
      const settings = await saveBookingEngineSettingsAction({
        propertyId: activeProperty.id,
        directChannelEnabled: enabled,
        bookingRoute: route,
        bookingRoutePercent: route === "cm" ? routePercent : 0,
      });
      setEnabled(settings.directChannelEnabled);
      setRoute(settings.bookingRoute === "cm" ? "cm" : "pms");
      setRoutePercent(settings.bookingRoutePercent ?? 0);
      message.success("Booking engine settings saved");
    } catch (err) {
      message.error((err as Error).message || "Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  const columns = [
    {
      title: "GUEST",
      dataIndex: "guestName",
      key: "guest",
      render: (v: string) => <Text style={{ fontSize: 13 }}>{v || "Unknown Guest"}</Text>,
    },
    {
      title: "CONFIRMATION",
      dataIndex: "confirmationCode",
      key: "confirmation",
      render: (v: string) => <Text style={{ fontSize: 13 }}>{v || "-"}</Text>,
    },
    {
      title: "CHECK-IN",
      key: "checkin",
      render: (r: DirectReservation) => <Text style={{ fontSize: 13 }}>{fmtDate(r.checkIn)}</Text>,
    },
    {
      title: "CHECK-OUT",
      key: "checkout",
      render: (r: DirectReservation) => <Text style={{ fontSize: 13 }}>{fmtDate(r.checkOut)}</Text>,
    },
    {
      title: "TOTAL",
      key: "total",
      render: (r: DirectReservation) => <Text style={{ fontSize: 13 }}>{fmtMoney(r.total)}</Text>,
    },
    {
      title: "STATUS",
      dataIndex: "status",
      key: "status",
      render: (v: string) => <Text style={{ fontSize: 13 }}>{(v || "").toUpperCase()}</Text>,
    },
    {
      title: "BOOKED",
      dataIndex: "bookedAt",
      key: "bookedAt",
      render: (v: string) => <Text style={{ fontSize: 13 }}>{fmtBookedAt(v)}</Text>,
    },
  ];

  return (
    <div style={{ padding: "30px 38px", margin: "0 auto", animation: "fade-up .32s ease both" }}>
      <Flex justify="space-between" align="flex-end" style={{ marginBottom: 26 }}>
        <div>
          <Title level={2} style={{ margin: 0, fontSize: 25, letterSpacing: "-0.5px" }}>
            Booking Engine
          </Title>
          <Text type="secondary" style={{ fontSize: 14, marginTop: 4, display: "block" }}>
            {activeProperty ? `Direct-channel bookings for ${activeProperty.name}` : "Loading..."}
          </Text>
        </div>
        <GlobalOutlined style={{ fontSize: 18, color: "#8b5cf6" }} />
      </Flex>

      <Card
        variant="outlined"
        style={{ borderRadius: 16, marginBottom: 18 }}
        styles={{ body: { padding: "18px 20px" } }}
        title="Settings"
      >
        <Space orientation="vertical" size="large" style={{ width: "100%" }}>
          <Flex justify="space-between" align="center">
            <div>
              <Text style={{ fontSize: 14, fontWeight: 600 }}>Direct channel</Text>
              <div>
                <Text type="secondary" style={{ fontSize: 13 }}>
                  When off, the storefront stops taking new direct bookings for this property.
                </Text>
              </div>
            </div>
            <Switch checked={enabled} disabled={!activeProperty} onChange={setEnabled} />
          </Flex>

          <Flex justify="space-between" align="center">
            <div>
              <Text style={{ fontSize: 14, fontWeight: 600 }}>Booking route</Text>
              <div>
                <Text type="secondary" style={{ fontSize: 13 }}>
                  Where the booking engine sends stay actions during the cutover.
                </Text>
              </div>
            </div>
            <Segmented
              value={route}
              onChange={(v) => setRoute(v as "pms" | "cm")}
              options={[
                { label: "PMS (direct)", value: "pms" },
                { label: "Channel Manager", value: "cm" },
              ]}
            />
          </Flex>

          {route === "cm" && (
            <Flex justify="space-between" align="center">
              <div>
                <Text style={{ fontSize: 14, fontWeight: 600 }}>Canary ramp</Text>
                <div>
                  <Text type="secondary" style={{ fontSize: 13 }}>
                    Percentage of new bookings routed through the Channel Manager (0–100).
                  </Text>
                </div>
              </div>
              <InputNumber
                min={0}
                max={100}
                value={routePercent}
                onChange={(v) => setRoutePercent(Math.max(0, Math.min(100, Number(v) || 0)))}
                style={{ width: 100 }}
              />
            </Flex>
          )}

          <Flex justify="flex-end">
            <Button type="primary" loading={saving} disabled={!activeProperty} onClick={save} style={{ borderRadius: 10 }}>
              Save settings
            </Button>
          </Flex>
        </Space>
      </Card>

      {!enabled && (
        <Card
          variant="outlined"
          style={{ borderRadius: 12, marginBottom: 18, background: "rgba(250, 204, 21, 0.08)" }}
          styles={{ body: { padding: "12px 16px" } }}
        >
          <Text style={{ fontSize: 13 }}>
            The direct channel is off. The storefront will not take new bookings for this property.
            Existing reservations below are unaffected.
          </Text>
        </Card>
      )}

      <Card variant="outlined" style={{ borderRadius: 16 }} styles={{ body: { padding: 0 } }}>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={reservations}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
          locale={{
            emptyText: <Empty description="No direct bookings yet" image={Empty.PRESENTED_IMAGE_SIMPLE} />,
          }}
        />
      </Card>

      <CouponsManager />
    </div>
  );
}
