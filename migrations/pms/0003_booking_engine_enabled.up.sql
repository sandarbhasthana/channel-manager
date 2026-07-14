-- The direct sales channel (booking engine) on/off switch, per property.
--
-- When false, the storefront ingress refuses to quote or create new direct
-- bookings for the property. Existing reservations are unaffected. Defaults to
-- true so the channel is on for every property unless explicitly disabled.

ALTER TABLE pms.properties
    ADD COLUMN booking_engine_enabled BOOLEAN NOT NULL DEFAULT TRUE;
