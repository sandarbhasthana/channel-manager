"use client";

import { useEffect, useState } from "react";

interface InventoryUpdate {
  room_type_id: string;
  date: string;
  available: number;
  stop_sell: boolean;
  min_stay: number;
  max_stay: number;
  provider: string;
}

interface RateUpdate {
  room_type_id: string;
  date: string;
  price: number;
  currency: string;
  provider: string;
}

interface Reservation {
  id: string;
  provider: string;
  status: string;
  check_in: string;
  check_out: string;
  guest_name: string;
  room_type_id: string;
  total_price: number;
  currency: string;
}

interface OtaData {
  reservations: Reservation[];
  inventory: InventoryUpdate[];
  rates: RateUpdate[];
}

export default function Home() {
  const [data, setData] = useState<OtaData>({ reservations: [], inventory: [], rates: [] });
  const [provider, setProvider] = useState("bookingcom");
  const [loading, setLoading] = useState(true);

  // Reservation Form State
  const [guestName, setGuestName] = useState("");
  const [checkIn, setCheckIn] = useState("");
  const [checkOut, setCheckOut] = useState("");
  const [roomTypeId, setRoomTypeId] = useState("");
  const [price, setPrice] = useState("");

  const fetchData = async () => {
    try {
      const res = await fetch("/api/data");
      const json = await res.json();
      setData(json);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const int = setInterval(fetchData, 5000);
    return () => clearInterval(int);
  }, []);

  const handleCreateReservation = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await fetch("/api/reservations", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          provider,
          guest_name: guestName,
          check_in: checkIn,
          check_out: checkOut,
          room_type_id: roomTypeId,
          total_price: parseFloat(price) || 0,
          currency: "USD",
        }),
      });
      setGuestName("");
      setCheckIn("");
      setCheckOut("");
      setRoomTypeId("");
      setPrice("");
      fetchData();
    } catch (err) {
      console.error(err);
    }
  };

  if (loading) return <div className="p-10 flex items-center justify-center text-slate-400">Loading OTA Data...</div>;

  const currentInventory = data.inventory.filter((i) => i.provider === provider);
  const currentRates = data.rates.filter((r) => r.provider === provider);
  const currentReservations = data.reservations.filter((r) => r.provider === provider);

  const colors = {
    bookingcom: "bg-blue-600 hover:bg-blue-700",
    expedia: "bg-yellow-500 hover:bg-yellow-600 text-black",
  };

  return (
    <main className="min-h-screen bg-slate-900 text-slate-200 p-8 font-sans">
      <div className="max-w-6xl mx-auto space-y-8">
        {/* Header & Toggle */}
        <div className="flex items-center justify-between bg-slate-800 p-6 rounded-2xl shadow-xl border border-slate-700">
          <div>
            <h1 className="text-3xl font-bold text-white tracking-tight">Dummy OTA Extranet</h1>
            <p className="text-slate-400 mt-1">Live simulation of partner integrations</p>
          </div>
          <div className="flex bg-slate-900 rounded-lg p-1 border border-slate-700">
            <button
              onClick={() => setProvider("bookingcom")}
              className={`px-6 py-2 rounded-md font-medium transition-all ${
                provider === "bookingcom" ? "bg-blue-600 text-white shadow-lg" : "text-slate-400 hover:text-slate-200"
              }`}
            >
              Booking.com
            </button>
            <button
              onClick={() => setProvider("expedia")}
              className={`px-6 py-2 rounded-md font-medium transition-all ${
                provider === "expedia" ? "bg-yellow-500 text-black shadow-lg" : "text-slate-400 hover:text-slate-200"
              }`}
            >
              Expedia
            </button>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Main Dashboard: Synced Rates & Inventory */}
          <div className="lg:col-span-2 space-y-8">
            <section className="bg-slate-800 p-6 rounded-2xl border border-slate-700 shadow-xl">
              <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                Synced Availability
              </h2>
              {currentInventory.length === 0 ? (
                <p className="text-slate-500 italic">No availability data pushed yet.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left border-collapse">
                    <thead>
                      <tr className="border-b border-slate-700 text-slate-400 text-sm">
                        <th className="pb-3 font-medium">Room Type</th>
                        <th className="pb-3 font-medium">Date</th>
                        <th className="pb-3 font-medium">Available</th>
                        <th className="pb-3 font-medium">Min/Max Stay</th>
                        <th className="pb-3 font-medium">Stop Sell</th>
                      </tr>
                    </thead>
                    <tbody className="text-sm">
                      {currentInventory.map((i, idx) => (
                        <tr key={idx} className="border-b border-slate-700/50 hover:bg-slate-700/30 transition-colors">
                          <td className="py-3 font-mono text-slate-300">{i.room_type_id}</td>
                          <td className="py-3">{i.date.split("T")[0]}</td>
                          <td className="py-3">
                            <span className={`px-2 py-1 rounded-full text-xs font-bold ${i.available > 0 ? "bg-emerald-500/20 text-emerald-400" : "bg-red-500/20 text-red-400"}`}>
                              {i.available}
                            </span>
                          </td>
                          <td className="py-3">{i.min_stay} - {i.max_stay}</td>
                          <td className="py-3">{i.stop_sell ? "🔴 Yes" : "🟢 No"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>

            <section className="bg-slate-800 p-6 rounded-2xl border border-slate-700 shadow-xl">
              <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-blue-400 animate-pulse"></span>
                Synced Rates
              </h2>
              {currentRates.length === 0 ? (
                <p className="text-slate-500 italic">No rate data pushed yet.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left border-collapse">
                    <thead>
                      <tr className="border-b border-slate-700 text-slate-400 text-sm">
                        <th className="pb-3 font-medium">Room Type</th>
                        <th className="pb-3 font-medium">Date</th>
                        <th className="pb-3 font-medium">Price</th>
                      </tr>
                    </thead>
                    <tbody className="text-sm">
                      {currentRates.map((r, idx) => (
                        <tr key={idx} className="border-b border-slate-700/50 hover:bg-slate-700/30 transition-colors">
                          <td className="py-3 font-mono text-slate-300">{r.room_type_id}</td>
                          <td className="py-3">{r.date.split("T")[0]}</td>
                          <td className="py-3 font-medium text-emerald-400">{r.price} {r.currency}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
          </div>

          {/* Right Column: Reservations */}
          <div className="space-y-8">
            <section className="bg-slate-800 p-6 rounded-2xl border border-slate-700 shadow-xl">
              <h2 className="text-xl font-semibold text-white mb-4">Make a Booking</h2>
              <form onSubmit={handleCreateReservation} className="space-y-4">
                <div>
                  <label className="block text-xs text-slate-400 font-medium mb-1 uppercase tracking-wider">Guest Name</label>
                  <input required value={guestName} onChange={e => setGuestName(e.target.value)} type="text" className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-blue-500 transition-colors" placeholder="John Doe" />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs text-slate-400 font-medium mb-1 uppercase tracking-wider">Check-in</label>
                    <input required value={checkIn} onChange={e => setCheckIn(e.target.value)} type="date" className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-blue-500 transition-colors" />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-400 font-medium mb-1 uppercase tracking-wider">Check-out</label>
                    <input required value={checkOut} onChange={e => setCheckOut(e.target.value)} type="date" className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-blue-500 transition-colors" />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-slate-400 font-medium mb-1 uppercase tracking-wider">Room Type ID</label>
                  {currentInventory.length > 0 || currentRates.length > 0 ? (
                    <select
                      required
                      value={roomTypeId}
                      onChange={e => setRoomTypeId(e.target.value)}
                      className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-blue-500 transition-colors font-mono"
                    >
                      <option value="" disabled>Select a synced room type</option>
                      {Array.from(new Set([
                        ...currentInventory.map(i => i.room_type_id),
                        ...currentRates.map(r => r.room_type_id)
                      ])).map(rt => (
                        <option key={rt} value={rt}>{rt}</option>
                      ))}
                    </select>
                  ) : (
                    <input required value={roomTypeId} onChange={e => setRoomTypeId(e.target.value)} type="text" className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-blue-500 transition-colors font-mono" placeholder="Enter ID (or wait for sync)" />
                  )}
                </div>
                <div>
                  <label className="block text-xs text-slate-400 font-medium mb-1 uppercase tracking-wider">Total Price (USD)</label>
                  <input required value={price} onChange={e => setPrice(e.target.value)} type="number" step="0.01" className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-blue-500 transition-colors" placeholder="199.99" />
                </div>
                <button type="submit" className={`w-full py-3 rounded-lg font-bold text-white transition-all shadow-md ${provider === 'bookingcom' ? colors.bookingcom : colors.expedia}`}>
                  Book on {provider === 'bookingcom' ? 'Booking.com' : 'Expedia'}
                </button>
              </form>
            </section>

            <section className="bg-slate-800 p-6 rounded-2xl border border-slate-700 shadow-xl">
              <h2 className="text-xl font-semibold text-white mb-4">OTA Bookings</h2>
              {currentReservations.length === 0 ? (
                <p className="text-slate-500 italic text-sm">No reservations made.</p>
              ) : (
                <div className="space-y-3">
                  {currentReservations.map((r) => (
                    <div key={r.id} className="bg-slate-900 p-4 rounded-xl border border-slate-700/50">
                      <div className="flex justify-between items-start mb-2">
                        <span className="font-medium text-white">{r.guest_name}</span>
                        <span className="text-xs bg-emerald-500/20 text-emerald-400 px-2 py-1 rounded-full font-bold uppercase">{r.status}</span>
                      </div>
                      <div className="text-xs text-slate-400 space-y-1">
                        <p>ID: <span className="font-mono text-slate-300">{r.id}</span></p>
                        <p>Dates: {r.check_in} to {r.check_out}</p>
                        <p>Room: <span className="font-mono text-slate-300">{r.room_type_id}</span></p>
                        <p className="text-white font-medium mt-2">{r.total_price} {r.currency}</p>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
        </div>
      </div>
    </main>
  );
}
