"use client";

import React, { useState, useEffect } from "react";
import { Typography, Button, Space, Card, Table, Flex, Input, Select, DatePicker, Dropdown, App } from "antd";
import { DownloadOutlined, MoreOutlined, FilterOutlined } from "@ant-design/icons";
import { useTheme } from "../../../components/antd-provider";
import { usePropertyContext } from "../../../components/property-provider";
import { fetchBookingsForProperty, deleteBookingAction } from "../actions";

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;

export default function BookingsPage() {
  const { mode } = useTheme();
  const { message } = App.useApp();
  const [bookings, setBookings] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchText, setSearchText] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [dateRange, setDateRange] = useState<any>(null);
  const { activeProperty, loading: ctxLoading } = usePropertyContext();
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
        adults: r.adults !== undefined ? r.adults : 2,
        children: r.children || 0,
        status: (r.status || "CONFIRMED").toUpperCase(),
        payment: (r.paymentStatus || r.payment_status || "UNPAID").toUpperCase(),
        rawCheckin: r.checkin,
      };
    });
  };

  useEffect(() => {
    if (ctxLoading || !activeProperty) return;

    const loadData = async () => {
      setLoading(true);
      try {
        const data = await fetchBookingsForProperty(activeProperty.id);
        if (data && data.length > 0) {
          setBookings(mapBookings(data));
        } else {
          setBookings([]);
        }
      } catch (err) {
        setBookings([]);
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [activeProperty, ctxLoading]);

  const handleDelete = async (bookingId: string) => {
    if (!activeProperty) return;
    try {
      // Find original booking ID from mapped ID if possible
      const originalBooking = bookings.find(b => b.id === bookingId || b.uid.startsWith(bookingId));
      const targetId = originalBooking ? originalBooking.uid.substring(0, originalBooking.uid.lastIndexOf('-')) : bookingId;
      
      await deleteBookingAction(activeProperty.id, targetId);
      message.success(`Booking ${targetId} successfully deleted`);
      
      // Update local state
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

  const filteredBookings = bookings.filter(b => {
    const matchesSearch = b.guest.toLowerCase().includes(searchText.toLowerCase()) || 
                          b.id.toLowerCase().includes(searchText.toLowerCase()) ||
                          b.room.toLowerCase().includes(searchText.toLowerCase());
    const matchesStatus = statusFilter === "all" || 
                          b.status.toLowerCase() === statusFilter || 
                          b.payment.toLowerCase() === statusFilter;
                          
    let matchesDate = true;
    if (dateRange && dateRange[0] && dateRange[1]) {
      const checkinDate = new Date(b.rawCheckin);
      matchesDate = checkinDate >= dateRange[0].startOf('day').toDate() && checkinDate <= dateRange[1].endOf('day').toDate();
    }
    
    return matchesSearch && matchesStatus && matchesDate;
  });

  return (
    <div style={{ padding: "30px 38px", margin: "0 auto", animation: "fade-up .32s ease both" }}>
      <Flex justify="space-between" align="flex-end" style={{ marginBottom: 26 }}>
        <div>
          <Title level={2} style={{ margin: 0, fontSize: 25, letterSpacing: "-0.5px" }}>Bookings</Title>
          <Text type="secondary" style={{ fontSize: 14, marginTop: 4, display: "block" }}>
            {activeProperty ? `Manage all reservations for ${activeProperty.name}` : 'Loading...'}
          </Text>
        </div>
        <Space size="middle">
          <Button icon={<DownloadOutlined />} style={{ borderRadius: 10, fontWeight: 600 }}>Export Data</Button>
        </Space>
      </Flex>

      <Card 
        variant="outlined" 
        style={{ borderRadius: 16 }}
        styles={{ body: { padding: 0 } }}
        title={
          <Flex gap={16} align="center" style={{ padding: "8px 0" }}>
            <Input.Search 
              placeholder="Search by guest, room, or ID" 
              allowClear 
              onChange={e => setSearchText(e.target.value)} 
              style={{ width: 300 }}
            />
            <RangePicker 
              style={{ width: 260 }} 
              value={dateRange}
              onChange={(dates) => setDateRange(dates)}
            />
            <Select 
              value={statusFilter} 
              onChange={setStatusFilter}
              style={{ width: 160 }} 
              options={[
                { value: 'all', label: 'All Statuses' }, 
                { value: 'confirmed', label: 'Confirmed' }, 
                { value: 'checked-in', label: 'Checked-in' },
                { value: 'paid', label: 'Paid' },
                { value: 'partially_paid', label: 'Partially Paid' },
                { value: 'unpaid', label: 'Unpaid' }
              ]} 
            />
            <Button icon={<FilterOutlined />}>More Filters</Button>
          </Flex>
        }
      >
        <Table 
          columns={columns} 
          dataSource={filteredBookings} 
          pagination={{ 
            defaultPageSize: 10,
            showSizeChanger: true,
            pageSizeOptions: ['10', '20', '50'],
            showTotal: (total, range) => `${range[0]}-${range[1]} of ${total} bookings`,
            style: { padding: '16px 24px', margin: 0, borderTop: isDark ? '1px solid #18253f' : '1px solid #f1f5f9' }
          }} 
          rowKey={(record) => record.uid || record.id}
          size="middle"
          loading={loading || ctxLoading}
          className="custom-table"
        />
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
        .ant-picker-range-separator {
          display: flex;
          align-items: center;
        }
        .ant-picker-range-separator svg {
          vertical-align: middle;
          margin-top: -1px;
        }
        .custom-table .ant-pagination-item {
          border-radius: 6px !important;
        }
        .custom-table .ant-pagination-item-active {
          border-radius: 6px !important;
        }
      `}} />
    </div>
  );
}
