"use client";

import React, { useEffect, useState, useMemo } from "react";
import { usePropertyContext } from "@/components/property-provider";
import { Spin, Typography, Flex, Card, Button, App } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import CustomCalendarGrid from "@/components/calendar/CustomCalendarGrid";
import { useProcessedEvents } from "@/components/calendar/hooks/useProcessedEvents";
import { useTheme } from "@/components/antd-provider";
import { PmsBooking } from "@/lib/api";
import { fetchRoomTypesForProperty, fetchBookingsForProperty, updateBookingAction } from "@/app/dashboard/actions";
import { Room, Reservation, DEFAULT_CALENDAR_FILTERS } from "@/components/calendar/utils/types";

const parseDateString = (dateStr: string) => {
  const [year, month, day] = dateStr.split("-").map(Number);
  return { year, month, day };
};

const { Title, Text } = Typography;

export default function CalendarPage() {
  const { message } = App.useApp();
  const { mode } = useTheme();
  const isDark = mode === "dark";
  const { activeProperty, loading: propertyLoading } = usePropertyContext();
  const [rooms, setRooms] = useState<Room[]>([]);
  const [reservations, setReservations] = useState<Reservation[]>([]);
  const [loading, setLoading] = useState(true);
  
  const [visibleStartDate, setVisibleStartDate] = useState(() => {
    return new Date().toISOString().slice(0, 10);
  });
  const [visibleDays, setVisibleDays] = useState(14);

  const fetchBookings = async (propertyId: string) => {
    try {
      const fetchedBookings = await fetchBookingsForProperty(propertyId);
      const mappedReservations: Reservation[] = fetchedBookings.map(b => ({
        id: b.bookingId || b.booking_id || "",
        roomId: b.roomId || b.room_id || "",
        guestName: b.guestName || b.guest_name || "Unknown",
        checkIn: b.checkin,
        checkOut: b.checkout,
        adults: b.adults || 1,
        children: b.children || 0,
        status: b.status,
        notes: b.notes,
      }));
      setReservations(mappedReservations);
    } catch (err) {
      console.error("Error loading bookings", err);
      message.error("Failed to fetch reservations");
    }
  };

  useEffect(() => {
    if (!activeProperty?.id) return;
    
    setLoading(true);
    fetchRoomTypesForProperty(activeProperty.id).then((fetchedRoomTypes) => {
      const mappedRooms: Room[] = fetchedRoomTypes.map(rt => ({
        id: rt.id,
        title: rt.name,
        children: rt.rooms?.map(r => ({
          id: r.id,
          title: r.name
        })) || []
      }));
      setRooms(mappedRooms);
    }).catch(err => {
      console.error("Error loading room types", err);
    });

    fetchBookings(activeProperty.id).finally(() => setLoading(false));
  }, [activeProperty?.id]);

  const handleRefresh = async () => {
    if (!activeProperty?.id) return;
    setLoading(true);
    await fetchBookings(activeProperty.id);
    setLoading(false);
    message.success("Calendar synced");
  };

  const handleEventDrop = async (event: any, newRoomId: string, newStart: string, newEnd: string) => {
    if (!activeProperty?.id) return;
    
    const previousReservations = [...reservations];
    
    // Optimistic UI update
    setReservations(prev => prev.map(res => {
      if (res.id === event.id) {
        return { ...res, roomId: newRoomId, checkIn: newStart, checkOut: newEnd };
      }
      return res;
    }));

    try {
      await updateBookingAction(activeProperty.id, event.id, {
        room_id: newRoomId,
        checkin: parseDateString(newStart),
        checkout: parseDateString(newEnd)
      });
      message.success("Reservation updated");
    } catch (err) {
      console.error(err);
      message.error("Failed to update reservation");
      setReservations(previousReservations);
    }
  };

  const handleEventResize = async (event: any, newStart: string, newEnd: string) => {
    if (!activeProperty?.id) return;
    
    const previousReservations = [...reservations];
    
    // Optimistic UI update
    setReservations(prev => prev.map(res => {
      if (res.id === event.id) {
        return { ...res, checkIn: newStart, checkOut: newEnd };
      }
      return res;
    }));

    try {
      await updateBookingAction(activeProperty.id, event.id, {
        checkin: parseDateString(newStart),
        checkout: parseDateString(newEnd)
      });
      message.success("Reservation extended");
    } catch (err) {
      console.error(err);
      message.error("Failed to extend reservation");
      setReservations(previousReservations);
    }
  };

  const processedEvents = useProcessedEvents({
    reservations,
    blocks: [],
    isDarkMode: isDark
  });

  if (propertyLoading) {
    return <div className="flex h-full items-center justify-center"><Spin size="large" /></div>;
  }

  if (!activeProperty) {
    return <div className="p-8 text-center text-gray-500">Please select a property first.</div>;
  }

  return (
    <div style={{ padding: "30px 38px", margin: "0 auto", animation: "fade-up .32s ease both", height: "100%", display: "flex", flexDirection: "column" }}>
      <Flex justify="space-between" align="flex-end" style={{ marginBottom: 26 }}>
        <div>
          <Title level={2} style={{ margin: 0, fontSize: 25, letterSpacing: "-0.5px" }}>Calendar</Title>
          <Text type="secondary" style={{ fontSize: 14, marginTop: 4, display: "block" }}>
            View and manage rates across your property.
          </Text>
        </div>
        <Button 
          icon={<ReloadOutlined />} 
          onClick={handleRefresh} 
          loading={loading}
        >
          Refresh
        </Button>
      </Flex>
      
      <Card 
        variant="outlined" 
        style={{ borderRadius: 16, flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}
        styles={{ body: { padding: 0, flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" } }}
      >
        <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
          <CustomCalendarGrid
            resources={rooms}
            events={processedEvents}
            reservations={reservations}
            selectedDate={visibleStartDate}
            visibleDays={visibleDays}
            selectedResource={null}
            setSelectedResource={() => {}}
            holidays={{}}
            filters={DEFAULT_CALENDAR_FILTERS}
            onCellClick={(roomId, roomName, date) => { console.log("Cell clicked", date, roomId); }}
            onEventClick={(event) => { console.log("Event clicked", event); }}
            onEventDrop={handleEventDrop}
            onEventResize={handleEventResize}
            onDatesSet={(start) => setVisibleStartDate(start.toISOString().slice(0, 10))}
          />
        </div>
      </Card>
      
      <style dangerouslySetInnerHTML={{__html: `
        @keyframes fade-up { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
      `}} />
    </div>
  );
}
