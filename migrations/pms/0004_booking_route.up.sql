-- Per-property routing for the booking engine's stay actions (Phase 4).
--
--   booking_route          — where the BE sends search/quote/create for this
--                            property: 'pms' (direct, pre-cutover) or 'cm'.
--   booking_route_percent  — canary ramp: the percentage of this property's
--                            bookings routed to 'cm' when booking_route = 'cm'.
--                            0 means none, 100 means all. Ignored for 'pms'.
--
-- Defaults keep every property on the pre-cutover PMS path until a config is
-- propagated (from the PMS for bundled tenants, or set in the CM dashboard for
-- self-managed ones).

ALTER TABLE pms.properties
    ADD COLUMN booking_route TEXT NOT NULL DEFAULT 'pms'
        CHECK (booking_route IN ('pms', 'cm')),
    ADD COLUMN booking_route_percent INTEGER NOT NULL DEFAULT 0
        CHECK (booking_route_percent BETWEEN 0 AND 100);
