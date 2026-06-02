"use client";

import React, { useState, useEffect } from "react";
import { Typography, Button, Space, Card, Row, Col, Progress, Table, Badge, Flex, Tag, Avatar, Spin, Select } from "antd";
import { 
  DownloadOutlined, 
  DashboardOutlined, 
  DollarOutlined, 
  LineChartOutlined, 
  StarOutlined, 
  RightOutlined 
} from "@ant-design/icons";
import dynamic from 'next/dynamic';
import { useTheme } from "../../components/antd-provider";
import { METRICS, REVENUE, CHANNEL_PERF, MOCK_BOOKINGS, CHANNELS } from "../../lib/mock-data";
import { fetchDashboardData, fetchBookingsForProperty } from "./actions";

// Dynamically import the Area chart because Ant Design Charts uses canvas which isn't SSR compatible
const Area = dynamic(() => import('@ant-design/plots').then((mod) => mod.Area), { ssr: false });

const { Title, Text } = Typography;

export default function DashboardRoot() {
  const [range, setRange] = useState("12M");
  const { mode } = useTheme();
  const [bookings, setBookings] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  const [properties, setProperties] = useState<any[]>([]);
  const [activeProperty, setActiveProperty] = useState<any>(null);
  const isDark = mode === "dark";

  useEffect(() => {
    const loadDashboard = async () => {
      try {
        const data = await fetchDashboardData();
        if (data.properties.length > 0) {
          setProperties(data.properties);
          setActiveProperty(data.properties[0]);
        }
        
        if (data.bookings && data.bookings.length > 0) {
          // Map the API reservations to the expected format
          const mapped = data.bookings.map((r: any, i: number) => {
            return {
              uid: `${r.bookingId}-${i}`,
              id: r.bookingId.substring(0, 8).toUpperCase(),
              guest: r.guestName || "Unknown Guest",
              rt: r.roomType || r.roomName || "Unknown Room",
              channel: r.source || "Direct",
              ci: new Date(r.checkin).toLocaleDateString("en-US", { month: "short", day: "numeric" }),
              co: new Date(r.checkout).toLocaleDateString("en-US", { month: "short", day: "numeric" }),
              nights: Math.ceil((new Date(r.checkout).getTime() - new Date(r.checkin).getTime()) / (1000 * 60 * 60 * 24)),
              amt: 0, // Since amount is not available in PmsBooking directly, default to 0 for now
              status: r.status === "CONFIRMED" ? "Confirmed" : (r.status === "IN_HOUSE" ? "Checked-in" : "Pending"),
            };
          });
          setBookings(mapped.slice(0, 8)); // Just show recent 8
          setLoading(false);
          return;
        }
      } catch (err) {
        console.error("Failed to fetch dashboard data:", err);
      }
      
      // Fallback to mock data with unique uid added
      setBookings(MOCK_BOOKINGS.map((b, i) => ({ ...b, uid: `${b.id}-${i}` })));
      setLoading(false);
    };

    loadDashboard();
  }, []);

  const handlePropertyChange = async (propertyId: string) => {
    const prop = properties.find(p => p.id === propertyId);
    if (prop) setActiveProperty(prop);
    
    setLoading(true);
    try {
      const newBookings = await fetchBookingsForProperty(propertyId);
      if (newBookings && newBookings.length > 0) {
        const mapped = newBookings.map((r: any, i: number) => {
            return {
              uid: `${r.bookingId}-${i}`,
              id: r.bookingId.substring(0, 8).toUpperCase(),
              guest: r.guestName || "Unknown Guest",
              rt: r.roomType || r.roomName || "Unknown Room",
              channel: r.source || "Direct",
              ci: new Date(r.checkin).toLocaleDateString("en-US", { month: "short", day: "numeric" }),
              co: new Date(r.checkout).toLocaleDateString("en-US", { month: "short", day: "numeric" }),
              nights: Math.ceil((new Date(r.checkout).getTime() - new Date(r.checkin).getTime()) / (1000 * 60 * 60 * 24)),
              amt: 0,
              status: r.status === "CONFIRMED" ? "Confirmed" : (r.status === "IN_HOUSE" ? "Checked-in" : "Pending"),
            };
          });
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

  const getIcon = (id: string) => {
    switch (id) {
      case "occ": return <DashboardOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      case "adr": return <DollarOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      case "revpar": return <LineChartOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      case "rev": return <StarOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
      default: return <DashboardOutlined style={{ color: "#3b82f6", fontSize: 18 }} />;
    }
  };

  const columns = [
    {
      title: "GUEST",
      key: "guest",
      render: (record: any) => (
        <Flex align="center" gap={12}>
          <Avatar 
            style={{ 
              backgroundColor: isDark ? "#1b2945" : "#e2e8f0", 
              color: isDark ? "#e8edf7" : "#0f172a",
              fontWeight: 600,
              fontSize: 12
            }}
          >
            {record.guest.split(" ").map((n: string) => n[0]).join("").substring(0,2)}
          </Avatar>
          <Flex vertical>
            <Text strong style={{ fontSize: 13.5 }}>{record.guest}</Text>
            <Text type="secondary" style={{ fontSize: 11.5, fontFamily: "monospace" }}>{record.id}</Text>
          </Flex>
        </Flex>
      )
    },
    {
      title: "ROOM TYPE",
      dataIndex: "rt",
      key: "rt",
      render: (text: string) => <Text type="secondary" style={{ fontSize: 13.5 }}>{text}</Text>
    },
    {
      title: "CHANNEL",
      key: "channel",
      render: (record: any) => {
        const ch = CHANNELS.find(c => c.short === record.channel || c.name === record.channel) || CHANNELS[2];
        return (
          <Space size={8}>
            <div style={{ width: 9, height: 9, borderRadius: 3, background: ch.color }} />
            <Text strong style={{ fontSize: 13 }}>{ch.short}</Text>
          </Space>
        );
      }
    },
    {
      title: "DATES",
      key: "dates",
      render: (record: any) => (
        <Text type="secondary" style={{ fontSize: 13 }}>
          {record.ci} → {record.co} · <Text type="secondary" style={{ fontSize: 13, color: isDark ? "#5b6987" : "#94a3b8" }}>{record.nights}n</Text>
        </Text>
      )
    },
    {
      title: "AMOUNT",
      dataIndex: "amt",
      key: "amt",
      align: "right" as const,
      render: (amt: number) => <Text strong style={{ fontFamily: "monospace", fontSize: 14 }}>${amt?.toLocaleString()}</Text>
    },
    {
      title: "STATUS",
      key: "status",
      align: "right" as const,
      render: (record: any) => {
        let color = "default";
        if (record.status === "Confirmed") color = "cyan";
        else if (record.status === "Checked-in") color = "green";
        else if (record.status === "Pending") color = "orange";
        return <Badge color={color} text={<Text style={{ fontSize: 12 }}>{record.status}</Text>} />;
      }
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
          <Select 
            value={activeProperty?.id} 
            onChange={handlePropertyChange}
            options={properties.map(p => ({ value: p.id, label: p.name }))}
            style={{ width: 220 }}
            placeholder="Select a property"
            loading={properties.length === 0}
            popupMatchSelectWidth={false}
          />
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
        <Col xs={24} lg={14}>
          <Card 
            variant="outlined" 
            style={{ borderRadius: 16, height: "100%" }}
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
            <Button type="text" size="small" style={{ fontWeight: 600, display: "flex", alignItems: "center" }}>
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
      `}} />
    </div>
  );
}
