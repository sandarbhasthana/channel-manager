// Fictional brand: "Marisol" — a beachfront resort in Tulum, Mexico.

export const PROPERTY = {
  id: 'marisol-tulum',
  name: 'Marisol Beach Resort & Spa',
  location: 'Tulum, Quintana Roo · Mexico',
  type: 'Resort & Spa',
  rooms: 80,
  rating: 4.8,
  reviews: 2147,
  occupancy: 87,
  status: 'Live',
  channelsLive: 6,
};

export const CHANNELS = [
  { id: 'bookr',   name: 'Bookr',          short: 'Bookr',   cat: 'OTA · Global', logo: 'B', color: '#2563eb', status: 'connected', sync: '2 min ago',  mapped: 7, bookings30: 412, rev: 32, commission: '15%', ari: true },
  { id: 'airhaus', name: 'Airhaus',        short: 'Airhaus', cat: 'Vacation Rental', logo: 'A', color: '#f43f5e', status: 'connected', sync: '6 min ago',  mapped: 7, bookings30: 233, rev: 17, commission: '3%',  ari: true },
  { id: 'direct',  name: 'Marisol Direct', short: 'Direct',  cat: 'Booking Engine', logo: 'M', color: '#34d399', status: 'connected', sync: 'Real-time',  mapped: 7, bookings30: 318, rev: 24, commission: '0%',  ari: true },
  { id: 'expedion',name: 'Expedion',       short: 'Expedion',cat: 'OTA · Global', logo: 'E', color: '#fbbf24', status: 'connected', sync: '11 min ago', mapped: 6, bookings30: 176, rev: 13, commission: '18%', ari: true },
  { id: 'globe',   name: 'Globe Hotels',   short: 'Globe',   cat: 'Metasearch', logo: 'G', color: '#38bdf8', status: 'syncing',   sync: 'Syncing…',  mapped: 7, bookings30: 121, rev: 9,  commission: '12%', ari: true },
  { id: 'agora',   name: 'Agora Travel',   short: 'Agora',   cat: 'OTA · APAC', logo: 'Ag',color: '#a78bfa', status: 'error',     sync: 'Failed · 1h ago', mapped: 5, bookings30: 68,  rev: 5,  commission: '17%', ari: false },
  { id: 'wanderly',name: 'Wanderly',       short: 'Wanderly',cat: 'Vacation Rental', logo: 'W', color: '#fb923c', status: 'disconnected', sync: '—', mapped: 0, bookings30: 0, rev: 0, commission: '8%', ari: false },
  { id: 'nomado',  name: 'Nomado',         short: 'Nomado',  cat: 'OTA · Boutique', logo: 'N', color: '#22d3ee', status: 'disconnected', sync: '—', mapped: 0, bookings30: 0, rev: 0, commission: '14%', ari: false },
];

export const METRICS = [
  { id: 'occ',  label: 'Occupancy',       value: '87.4%',  delta: 4.2,  dir: 'up',   icon: 'gauge',   vs: 'vs last 30d' },
  { id: 'adr',  label: 'ADR',             value: '$312',   delta: 6.1,  dir: 'up',   icon: 'dollar',  vs: 'vs last 30d' },
  { id: 'revpar',label: 'RevPAR',         value: '$273',   delta: 9.8,  dir: 'up',   icon: 'trendUp', vs: 'vs last 30d' },
  { id: 'rev',  label: 'Revenue · MTD',   value: '$642K',  delta: 12.3, dir: 'up',   icon: 'sparkle', vs: 'vs last month' },
];

export const REVENUE = [
  { m: 'Jul', v: 408 }, { m: 'Aug', v: 452 }, { m: 'Sep', v: 361 }, { m: 'Oct', v: 474 },
  { m: 'Nov', v: 583 }, { m: 'Dec', v: 761 }, { m: 'Jan', v: 728 }, { m: 'Feb', v: 694 },
  { m: 'Mar', v: 612 }, { m: 'Apr', v: 538 }, { m: 'May', v: 431 }, { m: 'Jun', v: 642 },
];

export const CHANNEL_PERF = CHANNELS.filter(c => c.rev > 0).sort((a, b) => b.rev - a.rev)
  .map(c => ({ name: c.name, pct: c.rev, color: c.color, bookings: c.bookings30 }));

export const MOCK_BOOKINGS = [
  { id: 'BK-90412', guest: 'Lena Hartmann',  rt: 'Ocean View King',      channel: 'Bookr',    ci: 'Jun 4', co: 'Jun 9',  nights: 5, amt: 1225, status: 'Confirmed' },
  { id: 'BK-90411', guest: 'Marcus Okafor',  rt: 'Swim-Up Suite',        channel: 'Direct',   ci: 'Jun 3', co: 'Jun 7',  nights: 4, amt: 2080, status: 'Checked-in' },
  { id: 'BK-90410', guest: 'Sophie Dubois',  rt: 'Deluxe Balcony Suite', channel: 'Airhaus',  ci: 'Jun 6', co: 'Jun 10', nights: 4, amt: 1280, status: 'Confirmed' },
  { id: 'BK-90409', guest: 'Akira Tanaka',   rt: 'Garden View Queen',    channel: 'Expedion', ci: 'Jun 2', co: 'Jun 5',  nights: 3, amt: 555,  status: 'Checked-in' },
  { id: 'BK-90408', guest: 'Camila Mendes',  rt: 'Beachfront Villa',     channel: 'Bookr',    ci: 'Jun 8', co: 'Jun 14', nights: 6, amt: 4680, status: 'Pending' },
  { id: 'BK-90407', guest: 'Jonas Andersson',rt: 'Junior Suite',         channel: 'Globe',    ci: 'Jun 5', co: 'Jun 8',  nights: 3, amt: 1230, status: 'Confirmed' },
  { id: 'BK-90406', guest: 'Priya Nair',     rt: 'Penthouse Suite',      channel: 'Direct',   ci: 'Jun 12',co: 'Jun 18', nights: 6, amt: 8700, status: 'Confirmed' },
  { id: 'BK-90405', guest: 'Elena Rossi',    rt: 'Ocean View King',      channel: 'Airhaus',  ci: 'Jun 3', co: 'Jun 6',  nights: 3, amt: 735,  status: 'Checked-in' },
];

export const BOOKING_STATUS = {
  Confirmed:   'success',
  'Checked-in':'processing',
  Pending:     'warning',
} as const;
