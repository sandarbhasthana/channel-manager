"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { fetchDashboardData, setUserDefaultProperty } from "@/app/dashboard/actions";
import { Property, PmsBooking } from "@/lib/api";
import { App } from "antd";

interface PropertyContextType {
  properties: Property[];
  activeProperty: Property | null;
  setActiveProperty: (p: Property) => void;
  defaultPropertyId: string | null;
  setDefaultProperty: (id: string) => Promise<void>;
  loading: boolean;
  initialBookings: PmsBooking[];
}

const PropertyContext = createContext<PropertyContextType | null>(null);

export function PropertyProvider({ children }: { children: React.ReactNode }) {
  const [properties, setProperties] = useState<Property[]>([]);
  const [activeProperty, setActiveProperty] = useState<Property | null>(null);
  const [defaultPropertyId, setDefaultPropertyId] = useState<string | null>(null);
  const [initialBookings, setInitialBookings] = useState<PmsBooking[]>([]);
  const [loading, setLoading] = useState(true);
  const { message } = App.useApp();

  useEffect(() => {
    const load = async () => {
      try {
        const data = await fetchDashboardData();
        if (data.properties.length > 0) {
          setProperties(data.properties);
          const activeId = data.defaultPropertyId || data.properties[0].id;
          const active = data.properties.find(p => p.id === activeId) || data.properties[0];
          setActiveProperty(active);
          setDefaultPropertyId(data.defaultPropertyId);
          setInitialBookings(data.bookings || []);
        }
      } catch (e) {
        console.error(e);
      } finally {
        setLoading(false);
      }
    };
    load();
  }, []);

  const setDefaultProperty = async (id: string) => {
    try {
      await setUserDefaultProperty(id);
      setDefaultPropertyId(id);
      message.success("Default property updated");
    } catch (err) {
      console.error(err);
      message.error("Failed to set default property");
    }
  };

  return (
    <PropertyContext.Provider
      value={{
        properties,
        activeProperty,
        setActiveProperty,
        defaultPropertyId,
        setDefaultProperty,
        loading,
        initialBookings,
      }}
    >
      {children}
    </PropertyContext.Provider>
  );
}

export const usePropertyContext = () => {
  const ctx = useContext(PropertyContext);
  if (!ctx) throw new Error("usePropertyContext must be used within PropertyProvider");
  return ctx;
}
