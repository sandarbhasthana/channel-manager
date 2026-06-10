"use client";

import React, { useState, useEffect } from "react";
import { Typography, Button, Space, Card, Row, Col, Progress, Table, Badge, Flex, Tag, Avatar, Spin, Select, App, Dropdown } from "antd";
import { 
  DownloadOutlined, 
  DashboardOutlined, 
  DollarOutlined, 
  LineChartOutlined, 
  StarOutlined, 
  StarFilled,
  RightOutlined,
  MoreOutlined
} from "@ant-design/icons";
import dynamic from 'next/dynamic';
import { useRouter } from "next/navigation";
import { useTheme } from "../../components/antd-provider";
import { METRICS, REVENUE, CHANNEL_PERF, MOCK_BOOKINGS, CHANNELS } from "../../lib/mock-data";
import { fetchBookingsForProperty, deleteBookingAction } from "./actions";
import { usePropertyContext } from "../../components/property-provider";
// Dynamically import the Area chart because Ant Design Charts uses canvas which isn't SSR compatible
const Area = dynamic(() => import('@ant-design/plots').then((mod) => mod.Area), { ssr: false });

const { Title, Text } = Typography;

export default function DashboardRoot() {
  const router = useRouter();
  const [range, setRange] = useState("12M");
  const { mode } = useTheme();
  const { message } = App.useApp();
  const [bookings, setBookings] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  const { properties, activeProperty, setActiveProperty, defaultPropertyId, setDefaultProperty, initialBookings, loading: ctxLoading } = usePropertyContext();
  const isDark = mode === "dark";

  const mapBookings = (data: any[]) => {
    return data.map((r: any, i: number) => {
      const bId = r.bookingId || r.booking_id || "";
      const gName = r.guestName || r.guest_name || "Unknown Guest";
      const room = r.roomName || r.room_name || r.roomType || r.room_type || "Unknown Room";
      
      const formatDate = (d: string) => {
        if (!d) return "-";
        const date = new Date(d);
        if (isNaN(date.getTime())) return d;
        return date.toLocaleDateString("en-US", { month: "2-digit", day: "2-digit", year: "numeric" });
      };

      return {
        uid: `${bId}-${i}`,
        id: bId.substring(0, 8).toUpperCase(),
        guest: gName,
        room: room,
        checkin: formatDate(r.checkin),
        checkout: formatDate(r.checkout),
        adults: r.adults || 2,
        children: r.children || 0,
        status: (r.status || "CONFIRMED").toUpperCase(),
        payment: (r.paymentStatus || r.payment_status || "UNPAID").toUpperCase(),
      };
    });
  };

  useEffect(() => {
    if (ctxLoading) return;
    
    if (initialBookings && initialBookings.length > 0) {
      setBookings(mapBookings(initialBookings).slice(0, 8));
    } else {
      setBookings(MOCK_BOOKINGS.map((b, i) => ({ 
        uid: `${b.id}-${i}`,
        id: b.id,
        guest: b.guest,
        room: b.rt,
        checkin: b.ci,
        checkout: b.co,
        adults: 2,
        children: 0,
        status: b.status.toUpperCase(),
        payment: "PAID",
      })));
    }
    setLoading(false);
  }, [ctxLoading, initialBookings]);

  // When active property changes, refetch bookings (only if not initial load)
  const [lastPropId, setLastPropId] = useState<string | null>(null);

  useEffect(() => {
    if (ctxLoading || !activeProperty) return;
    if (!lastPropId) {
      setLastPropId(activeProperty.id);
      return; // Skip first mount because initialBookings handles it
    }
    if (activeProperty.id === lastPropId) return;

    setLastPropId(activeProperty.id);
    const loadNewBookings = async () => {
      setLoading(true);
      try {
        const newBookings = await fetchBookingsForProperty(activeProperty.id);
        if (newBookings && newBookings.length > 0) {
          const mapped = mapBookings(newBookings);
          setBookings(mapped.slice(0, 8));
        } else {
          setBookings([]);
        }
      } catch (err) {
        console.error(err);
        setBookings([]);
      } finally {
        setLoading(false);
      }
    };
    loadNewBookings();
  }, [activeProperty, ctxLoading, lastPropId]);

  const getIcon = (id: string) => {
    switch (id) {
      case "occ": return <DashboardOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      case "adr": return <DollarOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      case "revpar": return <LineChartOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      case "rev": return <StarOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      default: return <DashboardOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
    }
  };

  const handleDelete = async (bookingId: string) => {
    if (!activeProperty) return;
    try {
      const originalBooking = bookings.find(b => b.id === bookingId || b.uid.startsWith(bookingId));
      const targetId = originalBooking ? originalBooking.uid.substring(0, originalBooking.uid.lastIndexOf('-')) : bookingId;
      
      await deleteBookingAction(activeProperty.id, targetId);
      message.success(`Booking ${targetId} successfully deleted`);
      
      setBookings(prev => prev.map(b => 
        (b.id === bookingId || b.uid.startsWith(bookingId))
          ? { ...b, status: "CANCELLED" }
          : b
      ));
    } catch (error) {
      message.error("Failed to delete booking");
      console.error(error);
    }
  };

  const columns = [
    {
      title: "GUEST",
      key: "guest",
      render: (record: any) => <Text style={{ fontSize: 13 }}>{record.guest}</Text>
    },
    {
      title: "ROOM",
      dataIndex: "room",
      key: "room",
      render: (text: string) => <Text style={{ fontSize: 13 }}>{text}</Text>
    },
    {
      title: "CHECK-IN",
      dataIndex: "checkin",
      key: "checkin",
      render: (text: string) => <Text style={{ fontSize: 13 }}>{text}</Text>
    },
    {
      title: "CHECK-OUT",
      dataIndex: "checkout",
      key: "checkout",
      render: (text: string) => <Text style={{ fontSize: 13 }}>{text}</Text>
    },
    {
      title: "ADULTS",
      dataIndex: "adults",
      key: "adults",
      align: "center" as const,
      render: (val: number) => <Text style={{ fontSize: 13 }}>{val}</Text>
    },
    {
      title: "CHILDREN",
      dataIndex: "children",
      key: "children",
      align: "center" as const,
      render: (val: number) => <Text style={{ fontSize: 13 }}>{val}</Text>
    },
    {
      title: "STATUS",
      dataIndex: "status",
      key: "status",
      render: (status: string) => <Text style={{ fontSize: 13 }}>{status}</Text>
    },
    {
      title: "PAYMENT",
      dataIndex: "payment",
      key: "payment",
      render: (payment: string) => <Text style={{ fontSize: 13 }}>{payment}</Text>
    },
    {
      title: "ACTIONS",
      key: "actions",
      align: "center" as const,
      render: (text: any, record: any) => (
        <Dropdown
          trigger={["click"]}
          placement="bottomRight"
          align={{ offset: [0, -8] }}
          menu={{
            items: [
              { key: "report", label: "Generate Report", style: { padding: "0 12px", height: 28, lineHeight: "28px", fontSize: 13 } },
              { key: "invoice", label: "Generate Invoice", style: { padding: "0 12px", height: 28, lineHeight: "28px", fontSize: 13 } },
              { 
                key: "delete", 
                danger: true,
                label: "Delete Reservation",
                style: { padding: "0 12px", height: 28, lineHeight: "28px", fontSize: 13 },
                onClick: () => handleDelete(record.uid)
              }
            ]
          }}
        >
          <Button type="text" size="small" style={{ width: 24, height: 24, padding: 0 }} icon={<MoreOutlined style={{ color: "#8b5cf6", transform: "rotate(90deg)" }} />} />
        </Dropdown>
      )
    }
  ];

  return (
    <div style={{ padding: "30px 38px", margin: "0 auto", animation: "fade-up .32s ease both" }}>
      {/* Header section */}
      <Flex justify="space-between" align="flex-end" style={{ marginBottom: 26 }}>
        <div>
          <Title level={2} style={{ margin: 0, fontSize: 25, letterSpacing: "-0.5px" }}>Dashboard</Title>
          <Text type="secondary" style={{ fontSize: 14, marginTop: 4, display: "block" }}>
            {activeProperty ? `${activeProperty.name} · ${activeProperty.currency || 'USD'}` : 'Loading property...'}
          </Text>
        </div>
        <Space size="middle">
          <Space.Compact>
            <Select 
              value={activeProperty?.id} 
              onChange={(id) => {
                const prop = properties.find((p: any) => p.id === id);
                if (prop) setActiveProperty(prop);
              }}
              options={properties.map((p: any) => ({ 
                value: p.id, 
                label: (
                  <Flex justify="space-between" align="center">
                    {p.name}
                    {defaultPropertyId === p.id && <StarFilled style={{ color: "#faad14", fontSize: 12 }} />}
                  </Flex>
                )
              }))}
              style={{ width: 220 }}
              placeholder="Select a property"
              loading={ctxLoading}
              popupMatchSelectWidth={false}
            />
            <Button 
              icon={defaultPropertyId === activeProperty?.id ? <StarFilled style={{ color: "#faad14" }} /> : <StarOutlined />}
              onClick={() => activeProperty && setDefaultProperty(activeProperty.id)}
              disabled={!activeProperty || defaultPropertyId === activeProperty.id}
              title={defaultPropertyId === activeProperty?.id ? "Default property" : "Set as default"}
            />
          </Space.Compact>
          <Button icon={<DownloadOutlined />} style={{ borderRadius: 10, fontWeight: 600 }}>Export</Button>
          <Space.Compact>
            {["7D", "30D", "12M"].map(r => (
              <Button 
                key={r} 
                type={range === r ? "primary" : "default"}
                onClick={() => setRange(r)}
                style={{ 
                  fontWeight: 600,
                  background: range === r ? (isDark ? "#1b2945" : "#e2e8f0") : "transparent",
                  borderColor: isDark ? "#1e2c49" : "#e2e8f0",
                  color: range === r ? (isDark ? "#e8edf7" : "#0f172a") : (isDark ? "#8d9ab6" : "#475569")
                }}
              >
                {r}
              </Button>
            ))}
          </Space.Compact>
        </Space>
      </Flex>

      {/* Stats Grid */}
      <Row gutter={[16, 16]} style={{ marginBottom: 22 }}>
        {METRICS.map(m => (
          <Col xs={24} sm={12} lg={6} key={m.id}>
            <Card styles={{ body: { padding: "18px 20px" } }} variant="outlined" style={{ borderRadius: 16 }}>
              <Flex justify="space-between" align="center">
                <Text type="secondary" style={{ fontSize: 12.5, fontWeight: 500 }}>{m.label}</Text>
                <div style={{ 
                  width: 32, height: 32, borderRadius: 9, 
                  background: isDark ? "rgba(37, 99, 235, 0.16)" : "#eff6ff", 
                  display: "flex", alignItems: "center", justifyContent: "center" 
                }}>
                  {getIcon(m.id)}
                </div>
              </Flex>
              <div style={{ fontSize: 28, fontWeight: 600, letterSpacing: "-0.8px", marginTop: 14, fontFamily: "monospace" }}>
                {m.value}
              </div>
              <Space size={4} style={{ marginTop: 8 }}>
                <Text type={m.dir === "up" ? "success" : "danger"} style={{ fontSize: 12, fontWeight: 600 }}>
                  ↑ {m.delta}%
                </Text>
                <Text type="secondary" style={{ fontSize: 12, fontWeight: 500, color: isDark ? "#5b6987" : "#94a3b8" }}>{m.vs}</Text>
              </Space>
            </Card>
          </Col>
        ))}
      </Row>

      {/* Middle Grid */}
      <Row gutter={[18, 18]} style={{ marginBottom: 18 }}>
        {/* Revenue Chart */}
        <Col xs={24} lg={14} style={{ minWidth: 0 }}>
          <Card 
            variant="outlined" 
            style={{ borderRadius: 16, height: "100%", overflow: "hidden" }}
            title={
              <Flex justify="space-between" align="center">
                <div>
                  <Title level={5} style={{ margin: 0, fontSize: 15 }}>Revenue</Title>
                  <Text type="secondary" style={{ fontSize: 12.5, fontWeight: 400 }}>Gross booking value · last 12 months</Text>
                </div>
                <Tag color="success" style={{ borderRadius: 99, padding: "2px 8px", fontWeight: 600, margin: 0 }}>+12.3%</Tag>
              </Flex>
            }
          >
            <div style={{ height: 260, width: "100%", marginTop: 20 }}>
              <Area 
                data={REVENUE}
                xField="m"
                yField="v"
                autoFit={true}
                shapeField="smooth"
                style={{
                  fill: "linear-gradient(-90deg, rgba(37, 99, 235, 0.4) 0%, rgba(37, 99, 235, 0) 100%)",
                }}
                line={{
                  style: {
                    stroke: "#3b82f6",
                    lineWidth: 3,
                  }
                }}
                point={{
                  shapeField: "circle",
                  size: 4,
                  style: {
                    fill: isDark ? "#101a2e" : "#ffffff",
                    stroke: "#3b82f6",
                    lineWidth: 2,
                  }
                }}
                axis={{
                  x: {
                    grid: false,
                    line: false,
                    tick: false,
                    labelFill: isDark ? "#5b6987" : "#94a3b8",
                    labelFontSize: 12
                  } as any,
                  y: {
                    gridLineDash: [4, 4],
                    gridStroke: isDark ? "rgba(255,255,255,0.05)" : "rgba(0,0,0,0.05)",
                    labelFill: isDark ? "#5b6987" : "#94a3b8",
                    labelFontSize: 12
                  } as any
                }}
                tooltip={{
                  formatter: (datum: any) => {
                    return { name: "Revenue", value: `$${datum.v}K` };
                  },
                }}
                theme={isDark ? "dark" : "light"}
              />
            </div>
          </Card>
        </Col>

        {/* Channel Performance */}
        <Col xs={24} lg={10}>
          <Card 
            variant="outlined" 
            style={{ borderRadius: 16, height: "100%" }}
            title={
              <div>
                <Title level={5} style={{ margin: 0, fontSize: 15 }}>Channel Performance</Title>
                <Text type="secondary" style={{ fontSize: 12.5, fontWeight: 400 }}>Share of revenue · last 30 days</Text>
              </div>
            }
          >
            <div style={{ padding: "10px 0" }}>
              {CHANNEL_PERF.map(c => (
                <Flex key={c.name} align="center" style={{ marginBottom: 16 }}>
                  <div style={{ width: 100 }}>
                    <Space size={8}>
                      <div style={{ width: 8, height: 8, borderRadius: "50%", background: c.color }} />
                      <Text strong style={{ fontSize: 13 }}>{c.name}</Text>
                    </Space>
                  </div>
                  <div style={{ flex: 1, margin: "0 16px" }}>
                    <Progress 
                      percent={c.pct} 
                      strokeColor={c.color} 
                      railColor={isDark ? "#15213a" : "#f1f5f9"} 
                      showInfo={false} 
                      size="small" 
                    />
                  </div>
                  <div style={{ width: 40, textAlign: "right" }}>
                    <Text style={{ fontSize: 13, fontWeight: 600 }}>{c.pct}%</Text>
                  </div>
                </Flex>
              ))}
            </div>
            
            <div style={{ height: 1, background: isDark ? "#18253f" : "#f1f5f9", margin: "16px -24px" }} />
            
            <Flex justify="space-between" align="center" style={{ paddingBottom: 4 }}>
              <div>
                <Text type="secondary" style={{ fontSize: 12, color: isDark ? "#5b6987" : "#94a3b8" }}>Total bookings · 30d</Text>
                <div style={{ fontSize: 20, fontWeight: 600, fontFamily: "monospace", marginTop: 2 }}>1,329</div>
              </div>
              <div style={{ textAlign: "right" }}>
                <Text type="secondary" style={{ fontSize: 12, color: isDark ? "#5b6987" : "#94a3b8" }}>Avg. commission</Text>
                <div style={{ fontSize: 20, fontWeight: 600, fontFamily: "monospace", marginTop: 2 }}>11.4%</div>
              </div>
            </Flex>
          </Card>
        </Col>
      </Row>

      {/* Recent Bookings */}
      <Card 
        variant="outlined" 
        style={{ borderRadius: 16 }}
        styles={{ body: { padding: 0 } }}
        title={
          <Flex justify="space-between" align="center" style={{ padding: "4px 0" }}>
            <div>
              <Title level={5} style={{ margin: 0, fontSize: 15 }}>Recent Bookings</Title>
              <Text type="secondary" style={{ fontSize: 12.5, fontWeight: 400 }}>Across all connected channels</Text>
            </div>
            <Button type="text" size="small" style={{ fontWeight: 600, display: "flex", alignItems: "center" }} onClick={() => router.push("/dashboard/bookings")}>
              View all <RightOutlined style={{ fontSize: 12 }} />
            </Button>
          </Flex>
        }
      >
        <Spin spinning={loading}>
          <Table 
            columns={columns} 
            dataSource={bookings} 
            pagination={false} 
            rowKey={(record) => record.uid || record.id || Math.random().toString()}
            size="middle"
            className="custom-table"
          />
        </Spin>
      </Card>
      
      <style dangerouslySetInnerHTML={{__html: `
        @keyframes fade-up { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
        .custom-table .ant-table-thead > tr > th {
          font-size: 11px;
          font-weight: 600;
          letter-spacing: 0.4px;
          text-transform: uppercase;
          color: ${isDark ? "#5b6987" : "#64748b"};
        }
        .custom-table .ant-table-tbody > tr > td {
          padding: 13px 16px;
        }
        .custom-table .ant-table-thead > tr > th:first-child,
        .custom-table .ant-table-tbody > tr > td:first-child {
          padding-left: 24px !important;
        }
        .custom-table .ant-table-thead > tr > th:last-child,
        .custom-table .ant-table-tbody > tr > td:last-child {
          padding-right: 24px !important;
        }
      `}} />
    </div>
  );
}
