"use client";

import React, { useEffect, useMemo, useState } from "react";
import { Typography, Card, Table, Flex, Button, App, InputNumber, Empty, Alert } from "antd";
import { TagsOutlined } from "@ant-design/icons";
import { usePropertyContext } from "../../../components/property-provider";
import { fetchRoomRatesData, saveRoomRatesAction } from "./actions";
import type { RoomType, Connection, ChannelRateRule, StoredBaseRate } from "@/lib/api";

const { Title, Text } = Typography;

const CHANNEL_LABEL: Record<string, string> = {
  CHANNEL_KIND_BOOKING_COM: "Booking.com",
  CHANNEL_KIND_AIRBNB: "Airbnb",
  CHANNEL_KIND_EXPEDIA: "Expedia",
  CHANNEL_KIND_AGODA: "Agoda",
  CHANNEL_KIND_DIRECT: "Direct",
  CHANNEL_KIND_UNSPECIFIED: "Channel",
};

function channelLabel(c: Connection): string {
  return c.name || CHANNEL_LABEL[c.kind] || "Channel";
}

// The Booking Engine (direct channel) is always a distribution target and is not
// an OTA connector, so it's added as a fixed first channel. Its per-channel
// pricing rule is stored under the nil-UUID channel id.
const BOOKING_ENGINE_CHANNEL: Connection = {
  id: "00000000-0000-0000-0000-000000000000",
  kind: "CHANNEL_KIND_DIRECT",
  name: "Booking Engine",
  status: "CONNECTION_STATUS_ACTIVE",
  createdAt: "",
  updatedAt: "",
};

const cellKey = (roomTypeId: string, channelId: string) => `${roomTypeId}::${channelId}`;

function activeRoomCount(rt: RoomType): number {
  return (rt.rooms || []).filter((r) => r.is_active ?? r.isActive ?? true).length;
}

function fmtMoney(amount: number, currency: string | null): string {
  try {
    return new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: currency || "USD",
      maximumFractionDigits: 0,
    }).format(amount);
  } catch {
    return `${Math.round(amount)} ${currency || ""}`.trim();
  }
}

export default function RoomRatesPage() {
  const { message } = App.useApp();
  const { activeProperty, loading: ctxLoading } = usePropertyContext();

  const [roomTypes, setRoomTypes] = useState<RoomType[]>([]);
  const [channels, setChannels] = useState<Connection[]>([]);
  const [adjust, setAdjust] = useState<Record<string, number>>({});
  const [baseRates, setBaseRates] = useState<Record<string, { price: number | null; currency: string | null }>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    if (ctxLoading || !activeProperty) return;
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const data = await fetchRoomRatesData(activeProperty.id);
        if (cancelled) return;
        setRoomTypes(data.roomTypes);
        // The Booking Engine (direct channel) is always present; OTA connectors
        // follow (disabled ones can't receive rates, so they're excluded).
        setChannels([
          BOOKING_ENGINE_CHANNEL,
          ...data.channels.filter((c) => c.status !== "CONNECTION_STATUS_DISABLED"),
        ]);
        const map: Record<string, number> = {};
        for (const r of data.rules) map[cellKey(r.roomTypeId, r.channelId)] = r.adjustPct;
        setAdjust(map);
        const bmap: Record<string, { price: number | null; currency: string | null }> = {};
        for (const b of data.baseRates) bmap[b.roomTypeId] = { price: b.pricePerNight, currency: b.currency };
        setBaseRates(bmap);
        setDirty(false);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => {
      cancelled = true;
    };
  }, [activeProperty, ctxLoading]);

  const setCell = (roomTypeId: string, channelId: string, value: number | null) => {
    setAdjust((prev) => ({ ...prev, [cellKey(roomTypeId, channelId)]: Number(value) || 0 }));
    setDirty(true);
  };

  const setBaseCell = (roomTypeId: string, value: number | null) => {
    setBaseRates((prev) => ({
      ...prev,
      [roomTypeId]: {
        price: value == null ? null : Number(value),
        currency: prev[roomTypeId]?.currency ?? activeProperty?.currency ?? "USD",
      },
    }));
    setDirty(true);
  };

  const save = async () => {
    if (!activeProperty) return;
    setSaving(true);
    try {
      const rules: ChannelRateRule[] = [];
      for (const rt of roomTypes) {
        for (const ch of channels) {
          const v = adjust[cellKey(rt.id, ch.id)];
          if (v !== undefined) rules.push({ roomTypeId: rt.id, channelId: ch.id, adjustPct: v });
        }
      }
      const bases: StoredBaseRate[] = [];
      for (const rt of roomTypes) {
        const b = baseRates[rt.id];
        if (b && b.price != null) {
          bases.push({
            roomTypeId: rt.id,
            amount: b.price,
            currency: b.currency ?? activeProperty.currency ?? "USD",
          });
        }
      }
      await saveRoomRatesAction(activeProperty.id, bases, rules);
      message.success("Pricing saved");
      setDirty(false);
    } catch (err) {
      message.error((err as Error).message || "Failed to save pricing");
    } finally {
      setSaving(false);
    }
  };

  const columns = useMemo(() => {
    const cols: any[] = [
      {
        title: "ROOM TYPE",
        key: "roomType",
        fixed: "left" as const,
        width: 260,
        render: (rt: RoomType) => (
          <div>
            <Text strong style={{ fontSize: 13 }}>{rt.name}</Text>
            <div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {rt.code} · sleeps {rt.max_occupancy ?? rt.maxOccupancy ?? "-"} · {activeRoomCount(rt)} room
                {activeRoomCount(rt) !== 1 ? "s" : ""}
              </Text>
            </div>
          </div>
        ),
      },
      {
        title: "BASE RATE",
        key: "base",
        width: 150,
        align: "right" as const,
        render: (rt: RoomType) => {
          const b = baseRates[rt.id];
          return (
            <InputNumber
              value={b?.price ?? undefined}
              onChange={(v) => setBaseCell(rt.id, v as number | null)}
              min={0}
              step={1}
              placeholder="Set base"
              style={{ width: 120 }}
            />
          );
        },
      },
    ];
    for (const ch of channels) {
      cols.push({
        title: channelLabel(ch),
        key: ch.id,
        width: 150,
        align: "center" as const,
        render: (rt: RoomType) => {
          const adj = adjust[cellKey(rt.id, ch.id)] ?? 0;
          const b = baseRates[rt.id];
          const finalRate = b && b.price != null ? b.price * (1 + adj / 100) : null;
          return (
            <div>
              <InputNumber
                value={adj}
                onChange={(v) => setCell(rt.id, ch.id, v as number | null)}
                min={-100}
                max={500}
                step={1}
                formatter={(v) => `${v}%`}
                parser={(v) => (v ? Number(String(v).replace("%", "")) : 0)}
                style={{ width: 96 }}
              />
              <div style={{ marginTop: 4, minHeight: 15 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {finalRate != null ? fmtMoney(finalRate, b?.currency ?? null) : "—"}
                </Text>
              </div>
            </div>
          );
        },
      });
    }
    return cols;
  }, [channels, adjust, baseRates]);

  return (
    <div style={{ padding: "30px 38px", animation: "fade-up .32s ease both" }}>
      <Flex justify="space-between" align="flex-end" style={{ marginBottom: 20 }}>
        <div>
          <Title level={2} style={{ margin: 0, fontSize: 25, letterSpacing: "-0.5px" }}>
            <TagsOutlined style={{ marginRight: 10 }} />
            Rooms &amp; Rates
          </Title>
          <Text type="secondary" style={{ fontSize: 14, marginTop: 4, display: "block" }}>
            {activeProperty ? `Per-channel pricing for ${activeProperty.name}` : "Loading…"}
          </Text>
        </div>
        <Button
          type="primary"
          loading={saving}
          disabled={!dirty || !activeProperty}
          onClick={save}
          style={{ borderRadius: 10 }}
        >
          Save pricing
        </Button>
      </Flex>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 18, borderRadius: 12 }}
        title="How per-channel pricing works"
        description="The rooms below are the ones your PMS makes available for this property. BASE RATE is seeded from your PMS when a live quote is available and is editable here (stored in CM) — so you can set it even without a live PMS feed. Then set a per-channel adjustment (percent); the final rate each channel receives, shown under each input, is the base × (1 + adjustment). Leave 0% to distribute at the base rate."
      />

      <Card variant="outlined" style={{ borderRadius: 16 }} styles={{ body: { padding: 0 } }}>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={roomTypes}
          pagination={false}
          scroll={{ x: "max-content" }}
          locale={{
            emptyText: (
              <Empty
                description={
                  channels.length === 0
                    ? "No channels connected yet — add OTA connectors first."
                    : "No rooms available from the PMS for this property."
                }
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            ),
          }}
        />
      </Card>
    </div>
  );
}
