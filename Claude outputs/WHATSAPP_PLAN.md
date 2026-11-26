# WhatsApp Guest Channel (Twilio) — Multi‑tenant Plan for PMS / BE

Status: **PLAN — nothing implemented** (written 2026‑10‑17, audit against `feat/wifi-access` of pms-app; aura-hospitality and channel-manager checked for WhatsApp usage).
Owner: Sandarbh. Companions: `docs/AGENT_LAYER_PLAN.md` (staff bots; this plan is the first *guest‑facing* consumer of its foundation), `docs/VOICE_AGENT_REMAINING_FLOWS_PLAN.md` (guest voice; same identity/handoff/payment rules apply here), `docs/Multi-Tenant-ai-signup_PLAN.md` (org‑level AI provisioning).

**What the owner gets:** each property designates a WhatsApp number. Guests message it to **book a room, check a reservation, cancel, ask FAQs, or raise any other enquiry**; the bot answers as that property, writes to that property's data only, and hands off to staff when it should. Staff see and can take over every thread from the PMS.

---

## 0. Verdict & the decisions this plan takes

**Feasible, mostly composition.** The PMS already has the tool surface (35 property‑scoped, idempotent handlers in `src/app/api/webhooks/bookings/route.ts` + `src/lib/webhooks/*`), Twilio (`twilio@5`, `TwilioSMSService`, a `whatsapp:` branch in `handleSendBookingInvite`), BullMQ workers, Stripe Connect, `GuestRequest`/handoff, and the `openai` SDK that talks to xAI. New are: a **per‑property sender registry with tenant routing**, a **guest conversation runtime** (hybrid deterministic + LLM), a **property knowledge base** for FAQs, **WhatsApp templates/opt‑in** for anything we send outside the 24 h window, and a **staff inbox** for takeover.

Decisions (overridable, see §13):

1. **Tenancy = Twilio subaccount per Organization, one sender per Property.** Twilio allows exactly one WhatsApp Business Account (WABA) per Twilio account, so an org's WABA lives in its own subaccount; each property number is a sender under it. Tenant resolution for every inbound message is **by the `To` number only** → `PropertyMessagingChannel` → `{organizationId, propertyId}`. Nothing in the message body ever selects a tenant.
2. **Properties bring their own number** (Indian local numbers are not purchasable on Twilio; a Twilio US number is the fallback and shows as +1). The number must not be on the WhatsApp/WhatsApp Business *app* — this is the #1 onboarding friction for Indian hotels and gets a checklist (§4.3).
3. **Hybrid runtime, not a pure chatbot.** Menus, button taps and list selections route **deterministically** to handlers (no model call, no cost, no hallucination). Free text goes to the model with the same typed tool registry the agent layer defines (AG‑3), under a *Guest Concierge* persona. One runtime, two entry paths.
4. **Identity on WhatsApp = Meta‑verified phone + surname.** The sender's number (`WaId`) is verified by WhatsApp, which is stronger than caller‑ID, so phone‑match lookups (`phone_match_last10`) are allowed for *finding* a booking; *revealing or mutating* one still requires the surname (the voice‑plan rule). No OTP needed on this channel — WhatsApp **is** the OTP channel.
5. **Cancellation autonomy mirrors the agent layer:** free cancellation → bot executes after an explicit Confirm button; cancellation with penalty/refund → booking is flagged and a staff approval/handoff is created; the guest is told a human will confirm. Bots never move money on their own.
6. **Payments only by hosted Stripe link** (Checkout Session on the org's connected account). Card numbers are never typed into WhatsApp (PCI + Meta policy). Whether a property allows *book now, pay at hotel* is a per‑property toggle.
7. **Model: Grok via the shared `ChatProvider` adapter** (agent‑layer decision), `grok-4.3`, `reasoning_effort: none`, OpenAI fallback. Guest turns are short; cost ≈ $0.003–0.005/turn.
8. **Staff reply from the PMS, not from a phone.** Once a number is on the Business Platform it cannot also be in the app, so a minimal **WhatsApp Inbox** with takeover is part of v1 scope, not a nice‑to‑have.
9. **Commercially an add‑on per property** with a flat monthly price (consistent with the 2026‑10‑08 flat‑price stakeholder decision); Meta/Twilio per‑message fees are small enough to absorb (§10).

---

## 1. Current state (audit 2026‑10‑17)

| Area | What exists | Gap / reuse |
|---|---|---|
| Twilio | `twilio@^5.10.2`; `src/lib/notifications/twilio-sms-service.ts` (staff SMS, global `TWILIO_PHONE_NUMBER`); `handleSendBookingInvite` sends `whatsapp:` via global `TWILIO_WHATSAPP_NUMBER`; `src/app/api/webhooks/twilio/route.ts` consumes status callbacks | **No inbound message handling. No `X-Twilio-Signature` validation anywhere** (`validateRequest` is never called) — fix for SMS too (WA‑1). No per‑tenant credentials or sender registry. |
| Tool surface | `[propertyId]/route.ts` dispatch: `search_availability`, `search_flexible_availability`, `get_quote`, `hold_rooms`/`release_rooms`, `create_booking` (idempotency_key), `get_booking` (`guest_surname`, `phone`, `phone_match_last10`), `list_bookings`, `update_booking`, `cancel_booking`, `create_group_inquiry`, `create_guest_request`, `create_handoff`, `in_house_*`, `guest_profile_lookup`, `get_voice_context`, payments | Call **in‑process** (exported `handle*` functions), never over HTTP. `get_voice_context` already returns property description, check‑in/out, room types + amenities → seed of the context pack. |
| Guest identity | `Guest.phoneNormalized` indexed per org; `GetBookingParams.phone_match_last10`; voice plan's surname rule | Reuse as‑is. |
| Conversations | Nothing for guests; `InHouseStayContext` (token‑bound in‑house session); agent‑layer `AgentConversation/AgentTurn/AgentToolInvocation` **planned** (AG‑1) | WhatsApp needs a long‑lived per‑guest thread plus the per‑session LLM ledger. |
| FAQ / policies | None in PMS. aura's website bot uses a hardcoded `PROPERTY_INFO` + `gpt-4o-mini`; `PropertySettings.description` (Json) and `checkInTime/checkOutTime` exist | New `PropertyKnowledgeEntry` model + settings UI; later shared with aura's chatbot and the voice agent. |
| Handoff | `create_handoff` → `GuestRequest` + staff notification; `/tasks` UI | Reuse for "talk to staff"; add the inbox for live takeover. |
| Payments | Stripe Connect per org; `create_payment_intent` etc. (BE‑oriented, needs a browser) ; `charge-handlers.ts` | Add `create_payment_link` (Checkout Session) + webhook → existing payment recording. |
| Workers | BullMQ queues, `npm run workers` (Railway service) | Add `whatsapp-inbound` + `whatsapp-outbound` queues; later move the LLM part to the agent‑server when it exists. |
| Secrets | `PMS_WEBHOOK_SECRET` env map; no KMS | Per‑org Twilio subaccount tokens need an encrypted column (AES‑GCM with `APP_ENCRYPTION_KEY`), same approach recommended for CM's `SecretResolver`. |
| Public URL | `src/lib/app-url.ts` (`isPubliclyReachable`) | Twilio webhooks need the public URL; ngrok in dev. |

Platform facts that shape the design (verified 2026‑10‑17, re‑verify at build):

- **Twilio ISV model:** Tech Provider Program + Meta Embedded Signup; **one WABA per Twilio account** → a subaccount per customer; additional senders must be in the same WABA; **Senders API** (`POST /v2/Channels/Senders`, OTP via `configuration.verification_code`, poll `status` → `ONLINE`), GA since Nov 2025, also exposes `quality_rating` and `messaging_limit`. Non‑Twilio numbers must receive an SMS or voice OTP once; a number already on the WhatsApp app must have that account deleted first.
- **24‑hour customer‑service window:** opens on every guest message; inside it we may send free‑form text and interactive messages; outside it **only approved templates**. From **1 Oct 2026** Meta charges service (free‑form) messages beyond **1,000 per number per month** at the utility rate, and in‑window utility templates from the first message. India per‑message rates: utility/authentication ≈ ₹0.145, marketing ≈ ₹1.09 (Jan 2026 card). Twilio adds its own per‑message fee (~$0.005; confirm on the Twilio WhatsApp pricing page).
- **Interactive messages in‑session need no template approval:** quick‑reply (≤ 3 buttons in‑session, 20‑char titles, `ButtonPayload` ≤ 200 chars echoed back), list‑picker (menu of up to 10 rows), call‑to‑action URL buttons, location. Replies arrive on the webhook as `Body` + `ButtonText`/`ButtonPayload` (buttons) or `ListId`/`ListTitle` (lists). **WhatsApp Flows** (multi‑screen forms with date pickers) are supported by Twilio and are the v2 path for date/guest collection.
- **Meta policy (effective 15 Jan 2026):** general‑purpose AI assistants are barred from the Business API; business‑specific customer‑service bots are fine. The persona must stay scoped to the property (also the cheapest token is the one not generated).
- **Opt‑in** is required for business‑initiated (template) messages; STOP/opt‑out must be honoured.

---

## 2. Target architecture

```
 Guest's WhatsApp ──▶ Meta ──▶ Twilio (org subaccount, property sender)
                                   │ POST (form, X-Twilio-Signature)
                                   ▼
 ┌──────────────── pms-app (Next.js web service) ─────────────────────────────────────────┐
 │ /api/webhooks/whatsapp/inbound   1) resolve channel by `To` → {org, property, creds}      │
 │                                  2) validate signature with THAT org's token               │
 │                                  3) dedupe MessageSid (WebhookEvent)                        │
 │                                  4) persist GuestMessage, enqueue whatsapp-inbound          │
 │                                  5) return 200 <Response/> in < 1 s (Twilio 15 s limit)    │
 │ /api/webhooks/whatsapp/status    delivery receipts → GuestMessage.status (+ price)          │
 │ /api/messaging/whatsapp/**       staff inbox + takeover, channel settings, KB CRUD (session)│
 │ /api/webhooks/stripe (existing)  checkout.session.completed → record payment → confirm msg  │
 └──────────────────────────────────┬─────────────────────────────────────────────────────────┘
                                    │ Redis (BullMQ) + Postgres
                                    ▼
 ┌──────────── workers (npm run workers today; agent-server once AG‑5 exists) ───────────────┐
 │ whatsapp-inbound worker  → GuestConversationRuntime                                        │
 │     intent router: button/list payload ──▶ deterministic flow step (no model)              │
 │                    free text ────────────▶ ChatProvider (Grok) + ToolRegistry (guest tools)│
 │     flows: Book · MyBooking · Cancel · FAQ · Enquiry · Handoff · InHouse                   │
 │     every tool call: withPropertyContext(botUser, propertyId), own tx, idempotent          │
 │ whatsapp-outbound worker → Twilio REST (free-form | ContentSid template), rate-limited     │
 │ cron: window-expiry nudges, hold expiry, unanswered-question digest, KB warm cache         │
 └────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 2.1 Tenant model

| Level | Twilio | Ours | Notes |
|---|---|---|---|
| Platform | Parent account (Tech Provider) | env `TWILIO_ACCOUNT_SID/AUTH_TOKEN` | Creates subaccounts; parent sees all usage for billing. |
| Organization | **Subaccount** + the org's **WABA** (via Embedded Signup under the owner's Meta Business Portfolio) | `MessagingProviderAccount` | Encrypted subaccount auth token / API key. Templates are per WABA → per org. |
| Property | **Sender** (E.164) | `PropertyMessagingChannel` | `@@unique(phoneE164)` → inbound routing key. Bot user, persona config, hours, payment toggle. |
| Guest | `WaId` (phone) | `GuestConversation` `@@unique([channelId, waId])` | One thread per guest × property; the same guest at two properties = two threads. |

Pilot shortcut (decide in §13‑1): for the first in‑house properties, one parent account with **our** WABA and several senders is allowed by Twilio and skips Embedded Signup — but display name, business verification and quality rating are then shared across tenants. Keep the data model identical (one `MessagingProviderAccount` row marked `SHARED_PLATFORM_WABA`) so moving a property to its own org WABA later is a re‑registration, not a migration.

### 2.2 The bot is a `User`

Same principle as the agent layer: each `PropertyMessagingChannel` owns a bot `User` (`kind = BOT`, `FRONT_DESK` property role, email `whatsapp+<propertyId>@bots.<domain>`). Every reservation it creates/cancels is attributed to "WhatsApp Concierge (Grand Palace)" in `ReservationAuditLog`; `validatePropertyAccess`/`withPropertyContext` scope it for free; cancelling the channel deactivates the user. Reuses `UserKind`, the sign‑in/invite/roster guards and the subscription service from AG‑1/AG‑2 — **build those first** (they are 3 days of the agent plan and this plan needs them anyway).

### 2.3 Guest Conversation Runtime (hybrid)

```
inbound → normalise (text | button payload | list id | location | media)
       → load GuestConversation (mode BOT|HUMAN|PAUSED, fsm state, window)
       → mode HUMAN? → deliver to inbox only, notify assignee, done
       → payload present? → FlowEngine.step(state, payload)        // deterministic
       → else free text  → IntentRouter: cheap structured-output call → {intent, slots}
                           ├─ intent maps to a flow → FlowEngine.start/resume with slots
                           └─ FAQ/other → ConciergeAgent (ToolRegistry guest allowlist, ≤4 tools, ≤2 round trips)
       → render reply as WhatsApp message(s): text + quick-replies / list-picker / CTA / location
       → enqueue whatsapp-outbound (ordered per conversation), persist turns & invocations
```

- **FlowEngine** is a small typed state machine per flow (`state: Json` on the conversation, versioned). Steps call handlers directly: `handleSearchAvailability`, `handleGetQuote`, `handleHoldRooms`, `handleCreateBooking`, `handleGetBooking`, `handleListBookings`, `handleCancelBooking`, `handleCreateGroupInquiry`, `handleCreateGuestRequest`, `handleCreateHandoff`. A flow step is one BullMQ job, one property context, ≤ 10 s.
- **Natural‑language slots** (dates "next Friday for 2 nights", "2 adults 1 kid", names) are parsed by a tiny structured‑output model call (`grok-4.3`, `reasoning_effort: none`, JSON schema), never by regex; the parsed value is **echoed back as buttons for confirmation** before any search ("12–14 Oct · 2 adults — Correct? / Change").
- **ConciergeAgent** = the agent‑layer loop (AG‑5) in guest mode: persona prompt + property context pack + last N turns; tools = guest allowlist only (`faq.search`, `bookings.search_availability`, `bookings.get_quote`, `bookings.get_booking` (verified), `property.info`, `handoff.create`, `enquiry.create`); WRITE tools are *not* callable from free text — the model's job is to route into a flow, which owns the confirm step. That keeps writes deterministic and auditable without a separate approval system for guests.
- **Session boundaries:** `AgentConversation` (LLM ledger) is per session (closed after 24 h idle, summary kept); `GuestConversation` is forever (history, guest link, consent).

### 2.4 Property context pack (guest edition)

Redis `wa:ctx:<propertyId>` (TTL 10 min, invalidated by settings/room‑type/KB writes): name, address + Google Maps link, phone/email, timezone/currency, check‑in/out times, room types (name, occupancy, short description, from‑price today), amenities, policies (cancellation, children, pets, smoking, ID), payment options, directions/parking, breakfast hours, nearby landmarks, and the KB entries (§3 `PropertyKnowledgeEntry`) up to ~3 k tokens. Larger KBs → top‑k by embedding (pgvector) in v2. The model answers FAQ from this pack; it never invents a price (prices come from `search_availability`/`get_quote`).

### 2.5 Flows (v1 spec)

**Main menu** (first contact, "hi", "menu", or after any completed flow) — list‑picker, property‑branded header, in the guest's language:
`📅 Book a room · 🔎 My reservation · ❌ Cancel a reservation · ❓ Hotel info & FAQs · 👥 Group / event enquiry · 🙋 Talk to the front desk` (+ `🛎️ Room service / requests` when the phone matches an **in‑house** reservation).

**Book a room**
1. Dates & guests (NL or Flow in v2) → confirm buttons.
2. `search_availability` → list‑picker of ≤ 10 offers "Deluxe King · ₹6,500/night · 2 adults". No availability → `search_flexible_availability` → "nearest dates" list → still none → offer handoff/waitlist (never invent inventory).
3. Offer picked → `get_quote` (total, taxes, rate payment type, policy) → "Hold this room for 15 min" → `hold_rooms` (reuses the BE hold).
4. Guest details: prefill from `guest_profile_lookup` by phone; ask only what's missing (name, email); surname captured here is the one later used for verification.
5. Summary card + **Confirm booking** / **Change** / **Cancel**. Confirm has a one‑shot nonce bound to the hold.
6. `create_booking` (idempotency_key = invocation id, source `WHATSAPP` — add to `ReservationSource`, lands **UNASSIGNED** per RA plan), then by rate policy: PREPAID/DEPOSIT → `create_payment_link` → CTA button "Pay ₹X securely" (Stripe Checkout, 30‑min expiry; hold extended to match); FLEXIBLE → booking confirmed as pay‑at‑hotel **only if** `PropertyMessagingChannel.allowPayAtHotel`, else payment link for the card guarantee amount per policy.
7. Confirmation message (in‑window, free‑form) with reference, dates, policy, property contact; email confirmation via existing SendGrid path; opt‑in question for reminders ("Send me check‑in reminders on WhatsApp — Yes/No").
   Hold expiry / link expiry → `release_rooms` + a one‑line nudge inside the window; outside the window → nothing (no template spam).

**My reservation**
1. Find: `list_bookings` by `phone_match_last10` (upcoming/in‑house) → exactly one → step 3; several → list‑picker by dates; none → ask for booking reference.
2. Verify: ask surname (or reference + surname if found by phone of a *different* booker) → `get_booking` with `guest_surname`; mismatch → the generic "I couldn't find a booking with those details" (no enumeration), 3 attempts → handoff.
3. Status card: status, dates, room type (room number only if checked in), guests, balance due, check‑in time, "Pay balance" CTA if due, buttons: `Modify (v2)`, `Cancel`, `Get directions` (WhatsApp location message), `Add to calendar (.ics link)`.

**Cancel a reservation**
1. Find + verify as above.
2. Show the cancellation policy outcome computed server‑side (free / penalty ₹X / non‑refundable) — new helper `evaluateCancellation(reservation, now)` (today the voice agent cancels on "explicit confirmation" with no penalty maths; this helper is new and also useful to the BE).
3. Free → **Confirm cancellation** button (nonce) → `cancel_booking` with verified surname, reason `GUEST_WHATSAPP` → confirmation message + email; room released.
4. Penalty/refund due → booking gets `cancellationRequestedAt` + a `GuestRequest(kind=CANCELLATION_REVIEW)` with the amounts; guest told "the front desk will confirm the refund of ₹Y within N hours"; staff action in `/tasks` executes cancel + `refund_payment`. (Autonomy knob per property later: `AUTO_IF_REFUND_LTE`.)

**Hotel info & FAQs** — free text to ConciergeAgent with the context pack; answers cite the KB entry internally (for the unanswered‑question digest). Unanswered → "I'll ask the team" → `create_guest_request(kind=QUESTION)` + logged as an **unanswered question** for the owner to turn into a KB entry (Settings shows "12 questions this week you haven't answered").

**Group / event enquiry** — `create_group_inquiry` (exists: dates, rooms, purpose, contact) → staff follow‑up.

**Other enquiries / in‑house** — phone matches an in‑house reservation → routine requests via `create_guest_request` (housekeeping, towels, late checkout) honouring `inHouseOtp*` policy (if OTP policy on → the WhatsApp channel counts as verified contact only when the number matches the reservation contact; otherwise handoff). Anything else → `create_handoff` with a structured packet (intent, captured fields, last 10 messages, risk flags).

**Talk to the front desk** — conversation `mode = HUMAN`, staff notified (existing notification service + Ably), inbox thread opens; off‑hours (per `PropertyMessagingChannel.hours`) → bot says when staff are back and creates the handoff task; auto‑return to `BOT` after staff close the thread or 12 h idle.

**Cross‑cutting:** language detection on first message (reply in guest's language; menu labels localised for en/hi to start); "menu"/"start over" always works; STOP/UNSUBSCRIBE → consent revoked + ack; media (ID photos, PDFs) acknowledged and attached to the handoff, never processed by the bot in v1.

### 2.6 Outbound & templates

- Inside the window: free‑form text and interactive content (quick‑reply/list/CTA/location) via `messages.create({ from: "whatsapp:+…", to, body | contentSid + contentVariables })`.
- Outside the window: **approved utility templates only**, per org WABA, provisioned at org onboarding through the Content API and tracked in `MessageTemplate {key, contentSid, lang, status}`: `booking_confirmed`, `booking_cancelled`, `payment_link`, `payment_received`, `pre_arrival_reminder` (opt‑in only), `handoff_followup` ("Your question from earlier: …"), `otp_fallback` (if ever needed for voice). Marketing templates are out of scope for this plan.
- All sends go through one `WhatsAppOutbound` service: per‑conversation ordering, per‑sender rate limit (respect `messaging_limit` tier), consent check for templates, cost estimate per message, retries with Twilio error‑code mapping (63016 = outside window → switch to template or drop; 63024/63032 = invalid recipient; 21610 = opted out).
- Status callbacks update `GuestMessage.status` (`queued→sent→delivered→read/failed`) and `priceUsd`.

### 2.7 Staff inbox & takeover (v1, minimal)

`/messaging/whatsapp` (property‑scoped, roles with front‑desk access): thread list (guest, last message, mode, unread, linked reservation), thread view (transcript with bot/tool traces collapsed, guest profile card, reservation card), **Take over** (mode → HUMAN, bot silent), composer (text + a few canned replies; template picker when the window is closed), **Hand back to bot**, **Close**. Realtime via Ably channel `wa:<propertyId>`. Not built on `ChatRoom` (that model is staff‑only with Ably auth per user; bridging it would leak staff‑chat semantics into a guest thread) — but the inbox reuses the chat UI components.

---

## 3. Data model (Prisma, additive)

```prisma
enum MessagingProvider { TWILIO }
enum WabaOwnership { ORG_EMBEDDED_SIGNUP SHARED_PLATFORM_WABA }
enum MessagingAccountStatus { PENDING ACTIVE SUSPENDED DISABLED }
enum SenderStatus { DRAFT PENDING_OTP PENDING_REVIEW ONLINE OFFLINE REJECTED DISABLED }
enum GuestConversationMode { BOT HUMAN PAUSED CLOSED }
enum GuestMessageDirection { INBOUND OUTBOUND }
enum GuestMessageStatus { RECEIVED QUEUED SENT DELIVERED READ FAILED }
enum MessageTemplateStatus { DRAFT PENDING APPROVED REJECTED PAUSED }
enum KnowledgeKind { FAQ POLICY INFO }
// ReservationSource += WHATSAPP ; GuestRequest taskKind += CANCELLATION_REVIEW, QUESTION ; AgentChannel += WHATSAPP (AG‑1)

model MessagingProviderAccount {          // one per org
  id               String @id @default(cuid())
  organizationId   String @unique
  provider         MessagingProvider @default(TWILIO)
  subaccountSid    String @unique
  authTokenEnc     String                  // AES‑GCM, APP_ENCRYPTION_KEY; or API key SID/secret
  wabaId           String?
  metaBusinessId   String?
  ownership        WabaOwnership
  status           MessagingAccountStatus @default(PENDING)
  businessVerified Boolean @default(false)
  createdByUserId  String
  createdAt        DateTime @default(now())  updatedAt DateTime @updatedAt
  channels         PropertyMessagingChannel[]
  templates        MessageTemplate[]
}

model PropertyMessagingChannel {          // one WhatsApp sender per property
  id               String @id @default(cuid())
  organizationId   String
  propertyId       String @unique          // v1: one number per property
  accountId        String
  phoneE164        String @unique          // inbound routing key
  senderSid        String? @unique         // Twilio XE… sender
  displayName      String
  status           SenderStatus @default(DRAFT)
  qualityRating    String?   messagingLimit String?   lastSyncedAt DateTime?
  botUserId        String  @unique         // the bot principal
  botEnabled       Boolean @default(true)
  persona          Json    @default("{}")  // tone, languages[], greeting, signature
  hours            Json?                   // staff hours per weekday, tz = Property.timezone
  offHoursBehaviour String @default("BOT_ONLY") // BOT_ONLY | BOT_AND_TASK
  allowPayAtHotel  Boolean @default(false)
  autoCancelIfFree Boolean @default(true)
  dailyMessageCap  Int     @default(5000)  // our own safety cap
  killSwitch       Boolean @default(false)
  createdAt        DateTime @default(now())  updatedAt DateTime @updatedAt
  conversations    GuestConversation[]
  @@index([organizationId])
}

model MessageTemplate {
  id          String @id @default(cuid())
  accountId   String
  key         String                       // booking_confirmed …
  language    String                       // en, hi
  contentSid  String @unique
  category    String                       // UTILITY | AUTHENTICATION | MARKETING
  status      MessageTemplateStatus @default(DRAFT)
  variables   String[]
  rejectionReason String?
  @@unique([accountId, key, language])
}

model GuestConversation {
  id              String @id @default(cuid())
  organizationId  String
  propertyId      String
  channelId       String
  waId            String                   // E.164 digits as Twilio sends WaId
  profileName     String?
  guestId         String?                  // linked Guest (by phoneNormalized) when known
  mode            GuestConversationMode @default(BOT)
  flow            String?                  // BOOK | MY_BOOKING | CANCEL | …
  flowState       Json?                    // versioned FSM state
  flowVersion     Int     @default(1)
  language        String  @default("en")
  windowExpiresAt DateTime?                // lastInboundAt + 24h
  lastInboundAt   DateTime?  lastOutboundAt DateTime?
  assignedUserId  String?                  // staff on takeover
  agentConversationId String?              // current AgentConversation (LLM ledger)
  unreadForStaff  Int     @default(0)
  closedAt        DateTime?
  createdAt       DateTime @default(now())  updatedAt DateTime @updatedAt
  messages        GuestMessage[]
  @@unique([channelId, waId])
  @@index([propertyId, mode, lastInboundAt])
  @@index([guestId])
}

model GuestMessage {
  id             String @id @default(cuid())
  conversationId String
  direction      GuestMessageDirection
  providerSid    String? @unique           // MessageSid (dedupe)
  body           String? @db.Text
  contentSid     String?                   // template / interactive used
  payload        Json?                     // ButtonPayload, ListId, media urls, location
  authorUserId   String?                   // bot or staff for outbound
  status         GuestMessageStatus
  errorCode      String?
  priceUsd       Decimal? @db.Decimal(10,5)
  category       String?                   // service | utility | marketing
  createdAt      DateTime @default(now())
  @@index([conversationId, createdAt])
}

model GuestContactConsent {                // per org, per phone
  id             String @id @default(cuid())
  organizationId String
  phoneE164      String
  whatsappOptIn  Boolean @default(false)
  source         String?                   // WHATSAPP_REPLY | BOOKING_ENGINE | PMS_STAFF
  optedInAt      DateTime?  optedOutAt DateTime?
  @@unique([organizationId, phoneE164])
}

model PropertyKnowledgeEntry {             // FAQ / policies, shared later with web chat + voice
  id          String @id @default(cuid())
  propertyId  String
  kind        KnowledgeKind
  question    String?                      // FAQ
  title       String?                      // POLICY/INFO
  answer      String  @db.Text
  tags        String[]
  language    String  @default("en")
  isActive    Boolean @default(true)
  sortOrder   Int     @default(0)
  source      String  @default("STAFF")    // STAFF | SUGGESTED (from unanswered questions)
  createdByUserId String?
  updatedAt   DateTime @updatedAt
  @@index([propertyId, kind, isActive])
}

model UnansweredQuestion {
  id String @id @default(cuid()); propertyId String; conversationId String; question String; askedAt DateTime @default(now())
  resolvedEntryId String?; @@index([propertyId, resolvedEntryId])
}
```

The LLM ledger (`AgentConversation`, `AgentTurn`, `AgentToolInvocation`, `AgentUsage`) comes from AG‑1 unchanged, with `AgentChannel.WHATSAPP`. Migration is additive; **`prisma generate` and `migrate dev` run on the Mac** (project rule).

---

## 4. Provisioning & onboarding

### 4.1 Org (super‑admin, extends `Multi-Tenant-ai-signup_PLAN` step "AI channels")
1. Create Twilio **subaccount** (`client.api.accounts.create({friendlyName: orgId})`), create an API key in it, store encrypted → `MessagingProviderAccount(PENDING)`.
2. Owner runs **Embedded Signup** from Settings → Messaging → WhatsApp ("Connect your Meta Business"): Facebook login, pick/create Business Portfolio + WABA, enter the first property number, OTP. We then `POST /v2/Channels/Senders` in the subaccount to attach the WABA + sender. (Pilot alternative: `SHARED_PLATFORM_WABA`, we register the sender ourselves.)
3. Submit the standard template set (en + hi) via Content API → `MessageTemplate(PENDING)`; poll approval; the channel is usable for inbound replies immediately, templates gate only business‑initiated sends.
4. Configure webhooks on the sender: inbound → `/api/webhooks/whatsapp/inbound`, status → `/api/webhooks/whatsapp/status` (one global URL; tenant is resolved from `To`/`AccountSid`).

### 4.2 Property (Settings → Messaging → WhatsApp)
Number entry (own number or "get me a number"), display name (Meta rules), OTP step (SMS or voice; the Senders API returns the status), persona (greeting, languages, tone), staff hours & off‑hours behaviour, pay‑at‑hotel toggle, cancellation autonomy, KB editor (FAQ/policies with a starter set generated from `get_voice_context` data for the owner to edit), test console ("send yourself a message"), go‑live checklist. Creates the bot `User` via the AG‑2 subscription service.

### 4.3 Onboarding checklist for Indian properties (the real friction)
- Number must **not** be on the WhatsApp/WhatsApp Business **app**; if it is, the owner deletes that account first (chat history is lost — tell them, suggest a dedicated SIM or a new number and forward old chats by hand).
- Number must receive an SMS or voice OTP once (no IVR‑only lines).
- Meta display name = legal/brand name of the property; rejection caps the sender at 250 business‑initiated/day until fixed.
- Business verification on the owner's Meta Business Portfolio raises messaging limits (1k → 10k → 100k/day); start it during trial.
- INR billing migration for Meta is mandatory by Dec 2026 (Twilio handles Meta billing for us; confirm nothing is needed on the org side).

---

## 5. API surface

**Webhooks (Twilio signature per org token; form‑encoded)**
- `POST /api/webhooks/whatsapp/inbound` — resolve → validate → dedupe → persist → enqueue → `200 <Response/>`.
- `POST /api/webhooks/whatsapp/status` — delivery receipts.
- `POST /api/webhooks/stripe` (existing endpoint) — handle `checkout.session.completed` with `metadata.source = whatsapp` → `record_booking_payment` → `payment_received` message.

**Staff (session auth, `validatePropertyAccess`)**
- `GET/POST/PATCH /api/messaging/whatsapp/channel?propertyId=` — channel CRUD, OTP submit, status refresh (Senders API fetch), kill switch.
- `GET /api/messaging/whatsapp/conversations?propertyId=&mode=` · `GET /:id` (messages, linked reservation) · `POST /:id/takeover` · `POST /:id/handback` · `POST /:id/close` · `POST /:id/messages` (text | templateKey+vars) · `POST /:id/assign`.
- `GET/POST/PATCH/DELETE /api/properties/:id/knowledge` — KB CRUD; `GET /unanswered` · `POST /unanswered/:id/convert`.
- `GET /api/messaging/whatsapp/usage?propertyId=&month=` — messages by category, est. Meta/Twilio/LLM cost.

**Super‑admin**
- `POST /api/admin/messaging/accounts` (create subaccount) · `POST /:id/templates/sync` · `POST /api/admin/messaging/kill-switch {organizationId?|propertyId?}`.

**Internal / new PMS actions (also exposed on `[propertyId]/route.ts` so voice and BE can use them)**
- `create_payment_link` — Stripe Checkout Session on the connected account (`{ stripeAccount }`), `metadata {reservationId, propertyId, source}`, `expires_at ≤ 30 min`, returns `url`; `get_payment_link` status.
- `evaluate_cancellation` — policy outcome (free | penalty amount | non‑refundable | refund due) from the reservation's rate policy + deadlines; pure read.

---

## 6. Module layout

```
src/lib/messaging/whatsapp/
  provider/twilio.ts          // subaccount client factory, send (text|content), senders API, signature validation
  provider/crypto.ts          // AES‑GCM for tokens
  inbound/parse.ts            // Twilio form → NormalisedInbound {kind:text|button|list|location|media, …}
  inbound/route.ts            // resolve channel by To, dedupe, enqueue
  runtime/index.ts            // GuestConversationRuntime (mode, window, dispatch)
  runtime/intent-router.ts    // structured-output intent+slots (ChatProvider)
  runtime/flows/{menu,book,my-booking,cancel,faq,enquiry,in-house,handoff}.ts
  runtime/render.ts           // reply → Twilio payload (quick-reply/list/CTA/location), localisation
  runtime/context-pack.ts     // property pack + KB
  tools/guest-registry.ts     // guest allowlist over src/lib/agents/tools (AG‑3)
  outbound/service.ts         // ordering, rate limits, consent, templates, error mapping, cost
  outbound/templates.ts       // template catalog + Content API sync
  inbox/service.ts            // takeover/handback/assign, Ably events
  knowledge/service.ts        // KB CRUD, unanswered questions, starter KB
  payments/links.ts           // create_payment_link + webhook handling
  cancellation/evaluate.ts    // evaluateCancellation()
src/lib/queue/workers/whatsapp-worker.ts   // whatsapp-inbound / whatsapp-outbound / wa-cron
src/app/api/webhooks/whatsapp/{inbound,status}/route.ts
src/app/api/messaging/whatsapp/**  ·  src/app/api/properties/[id]/knowledge/**
src/app/(app)/messaging/whatsapp/**            // inbox UI
src/app/(app)/settings/messaging/whatsapp/**   // channel + KB settings
scripts/wa-sandbox.ts                          // dev: Twilio sandbox join + local tunnel check
```

---

## 7. Security, privacy, compliance

- **Signature validation on every Twilio webhook** with the resolved org's token (`twilio.validateRequest(token, signature, publicUrl, params)`); public URL from `app-url.ts`; reject on mismatch *after* logging the `AccountSid`. Apply the same to the existing SMS status webhook (today unvalidated).
- **Tenant isolation:** `To` → channel → `{org, property}`; the bot `User` is bound to that property; every handler runs in `withPropertyContext(botUser, propertyId)`; reservation lookups add `propertyId` to the where‑clause (they already do). A guest's booking at property B is invisible to property A's bot even for the same phone.
- **Identity:** phone match finds, surname reveals/mutates; mismatch returns the one generic message; 3 strikes → handoff + conversation flagged; staff can see the attempts.
- **Prompt injection:** guest text, profile names, notes and OTA remarks are data inside `<guest_message>`/`<data>` envelopes; the model cannot call write tools; flows own confirmations; KB content is staff‑authored.
- **PII to the model vendor:** names, dates, phone (masked to last 4 unless needed) — same DPA note as the agent plan; **card data never** (links only). Transcript retention follows `PropertySettings.auditLogRetentionDays`; media URLs from Twilio expire — copy to S3 only when attached to a handoff.
- **Consent (Meta + India DPDP):** templates only with `whatsappOptIn`; STOP honoured instantly; opt‑in captured on WhatsApp ("Yes" button), in the BE checkout (checkbox, written to `GuestContactConsent` via a new `record_contact_consent` action), and by staff.
- **Abuse & cost caps:** per‑waId rate limit (e.g. 20 msgs/5 min → cool‑down message), per‑channel `dailyMessageCap`, per‑org monthly LLM cap (`AgentUsage`), model‑call budget per turn (≤ 4 tools, ≤ 2 round‑trips), persona refuses off‑topic (Meta policy + cost).
- **Kill switches:** channel, org, global `WHATSAPP_ENABLED`.
- **Secrets:** subaccount API keys encrypted at rest; rotate via admin; never log tokens or full phone numbers.
- **Idempotency:** `MessageSid` dedupe (`WebhookEvent`), flow nonces single‑use, `create_booking`/`cancel_booking` keyed by invocation id, Stripe webhook dedupe already exists.

---

## 8. Failure modes

| Failure | Behaviour |
|---|---|
| Twilio webhook retries (timeout/5xx) | Fast‑ack design; duplicates dropped by `MessageSid`. |
| Model down / slow | Deterministic flows keep working (menu, status, cancel); FAQ says "let me get a human" → handoff; circuit breaker per org. |
| Tool timeout / unknown write | Invocation `UNKNOWN`; never claim success; reconcile via `BookingOperation` idempotency key; guest told "verifying, you'll get a confirmation shortly". |
| Hold expired before confirm | Re‑quote and re‑hold once; price changed → show new price. |
| Payment link expired / abandoned | Hold released, booking stays `CONFIRMATION_PENDING` until `confirmationPendingTimeoutHours`; in‑window nudge once. |
| Outside 24 h window | Outbound service switches to the matching template or drops with a reason; staff composer shows template picker. |
| Sender OFFLINE / quality drop / Meta rejection | Status sync from Senders API; channel banner in settings; inbound still processed; outbound queued. |
| Guest spams / abusive | Rate‑limit, auto‑PAUSED after N, staff notified. |
| Staff forgets a HUMAN thread | 12 h idle → bot resumes with "I'm back; staff will follow up"; unresolved → task escalation (existing SLA machinery). |

---

## 9. Performance

Budgets: webhook ack p95 < 500 ms; deterministic flow step reply p95 < 2 s (one handler, one Twilio call); free‑text turn p95 < 5 s (intent call ≈ 0.7 s + ≤ 1 tool + reply); Twilio adds ~0.5–1.5 s delivery. One worker instance (concurrency 10) handles ≈ 100+ properties; the per‑conversation lock (`wa:lock:<convId>`) keeps ordering; all handlers are the same ones the BE uses, so no new DB hot paths. Typing indicators where Twilio exposes them for the Messaging API, else a quick "One moment…" only when a step will exceed ~3 s.

---

## 10. Cost & pricing

Per busy property / month (assumptions: 1,500 inbound, 2,000 outbound of which 300 templates; 1,200 model turns):

| Item | Basis | ≈ cost |
|---|---|---|
| Meta service messages | 1,000 free, then 1,000 × ₹0.145 | ₹145 (~$1.7) |
| Meta utility templates | 300 × ₹0.145 | ₹44 (~$0.5) |
| Twilio per‑message fee | 3,500 × ~$0.005 (verify) | ~$17.5 |
| Number | property's own ₹0 / Twilio US number ~$1.15 | ~$0–1 |
| LLM (grok‑4.3, cached prefix) | 1,200 × ~$0.004 | ~$5 |
| **Total COGS** | | **≈ $25–27** |

Idle property ≈ $2–3. Flat add‑on price in the $49–79/month range per property (or bundled into the Front Desk bot's $200) keeps >60 % margin at the busy case; the physical caps in §7 bound the worst case the way §15 of the agent plan does. Meter everything in `AgentUsage` + `GuestMessage.priceUsd`; show "this month" in Settings.

---

## 11. Ticketed plan

Phases are shippable; estimates are engineer‑days for someone who knows the repo. Depends on agent‑layer **AG‑1 (schema), AG‑2 (bot principals), AG‑3 (tool registry/policy), AG‑4 (Grok provider)** ≈ 7 d — build those first; WhatsApp becomes their first consumer and AG‑5's guest mode.

**Phase 0 — Plumbing (no model)**
- **WA‑0** Spike: Twilio sandbox end‑to‑end (inbound → reply with quick‑reply/list → status callback); subaccount + Senders API dry run on a test number; confirm typing‑indicator availability and the Twilio per‑message fee; record §13 answers. *1–2 d.*
- **WA‑1** Twilio signature validation helper + apply to existing SMS status webhook; `WebhookEvent` dedupe helper. *0.5 d.*
- **WA‑2** Prisma: §3 models/enums, `ReservationSource.WHATSAPP`, `AgentChannel.WHATSAPP`, `GuestRequest` kinds; migration; generate on Mac. *1 d.*
- **WA‑3** Provider layer: encrypted credentials, subaccount client factory, send (text/content), Senders API wrapper, status sync job. *2 d.*
- **WA‑4** Inbound pipeline: parse → resolve by `To` → validate → dedupe → persist → `whatsapp-inbound` queue; outbound service with ordering, rate limits, consent, error mapping, status webhook. *2 d.*
- **WA‑5** Channel provisioning: org subaccount creation (admin), property channel settings API + UI (number, OTP, display name, persona, hours, toggles, test console), bot `User` via AG‑2. *3 d.*
  *Exit:* a property number is ONLINE; "hi" gets a static menu; staff can see the message in a bare inbox list.

**Phase 1 — Transactional flows (deterministic) + inbox**
- **WA‑6** FlowEngine + conversation runtime (modes, window, locks, versioned state, "menu/start over", language). *2 d.*
- **WA‑7** My‑reservation flow (phone‑match find, surname verify, status card, directions/location, pay‑balance CTA via `create_payment_link`). *2 d.*
- **WA‑8** `evaluateCancellation()` + Cancel flow (free → auto with nonce; penalty → `CANCELLATION_REVIEW` task + staff execution in `/tasks`). *2 d.*
- **WA‑9** `create_payment_link` / `get_payment_link` actions + Stripe webhook handling + `payment_received` message. *1.5 d.*
- **WA‑10** Book flow with NL slot parsing (structured output), offers list, hold, guest details prefill, confirm, create, payment by rate policy, confirmation + consent prompt, hold/link expiry jobs. *4 d.*
- **WA‑11** Staff inbox v1 (threads, transcript, takeover/handback/close, composer with templates when window closed, Ably live updates, notifications). *4 d.*
- **WA‑12** Handoff/enquiry/in‑house flows (`create_handoff` packet, `create_group_inquiry`, `create_guest_request` for in‑house with OTP‑policy respect, off‑hours behaviour). *1.5 d.*
- **WA‑13** Eval harness: scripted WhatsApp conversations against recorded providers (book → pay → confirm; status with wrong surname; cancel free vs penalty; cross‑property phone probe must fail; STOP; window‑closed template path); property‑isolation and signature tests in CI. *2 d.*
  *Exit:* a guest books, pays and cancels a booking entirely in WhatsApp; staff take over a thread from the PMS; every write is attributed to the property's WhatsApp bot user.

**Phase 2 — FAQ & concierge (model)**
- **WA‑14** `PropertyKnowledgeEntry` service + Settings KB editor + starter KB from `get_voice_context`; context pack (guest edition) with Redis cache + invalidation. *2.5 d.*
- **WA‑15** ConciergeAgent on the AG‑5 loop in guest mode (read‑only allowlist, ≤ 4 tools, persona, injection envelopes, localisation), intent router in front of flows, unanswered‑question capture + weekly digest. *3 d.*
- **WA‑16** Templates: standard set (en/hi) via Content API at org onboarding, approval polling, template picker in inbox, `pre_arrival_reminder` opt‑in job. *1.5 d.*
- **WA‑17** Usage & cost: per‑message price capture, `AgentUsage` for WA turns, Settings "this month" meter, org/channel caps, kill switches, admin page. *1.5 d.*

**Phase 3 — Polish & scale**
- **WA‑18** WhatsApp Flows for date/guest capture and guest details (replaces NL parsing for the Book flow where supported); typing indicators. *3 d.*
- **WA‑19** Modify‑booking flow (`update_booking` with requote), add‑to‑calendar, multi‑room (group) booking via the GB plan's inquiry path. *2 d.*
- **WA‑20** Embedded Signup UI (Tech Provider) in Settings for org WABA onboarding; business‑verification guidance; Senders API bulk path. *2 d.*
- **WA‑21** Share the KB + persona with aura's website chatbot and the voice agent (one `PropertyKnowledgeEntry` source of truth; aura's `PROPERTY_INFO` retired). *1.5 d.*
- **WA‑22** Docs: `APP_FEATURES.md`, `APP_MAP.md`, Glossary (Sender, Window, Template, Takeover), runbook (number migration, template rejection, quality drops). *0.5 d.*

Total ≈ 45 engineer‑days + the 7 agent‑foundation days. **Phase 0 + 1 (≈ 26 d) is the MVP** that proves the channel without depending on model quality; Phase 2 adds the FAQ/concierge value.

---

## 12. Runbook to land it

1. Branch `feat/whatsapp-channel` off `master` after `feat/wifi-access` merges (migration ordering); land AG‑1..AG‑4 first (one PR, no UI), then WA‑0..WA‑5.
2. Dev: Twilio sandbox (`join <code>` from your phone) + ngrok; `isPubliclyReachable` guards the webhook URL registration; `WHATSAPP_ENABLED=false` default in prod.
3. Pilot on your own org with `SHARED_PLATFORM_WABA` and one Indian number (dedicated SIM); measure window/template mix and per‑turn cost for a month before pricing.
4. Deploy: webhooks on the web service; `whatsapp-*` queues in the workers service (same image, `npm run workers`); move the model path to the agent‑server when AG‑5 ships.
5. Each phase: evals green, `npm run test`, `tsc`, `eslint`, build on Mac (per `tailwind_sandbox_workaround` / `prisma_generate_on_device`).

---

## 13. Decisions to confirm before WA‑0

1. **WABA ownership for launch:** per‑org Embedded Signup (clean tenancy, owner's Meta Business, their verification) vs our shared WABA for the pilot (faster, shared reputation). Plan assumes shared for pilot, per‑org for GA.
2. **Numbers:** properties bring their own (recommended for India) vs we provision Twilio US numbers everywhere.
3. **Pay‑at‑hotel on WhatsApp** default off? And whether FLEXIBLE rates may book with no card guarantee on this channel.
4. **Cancellation autonomy:** free‑only auto (plan) vs also auto‑refund below a per‑property threshold.
5. **Inbox vs tasks:** is the minimal inbox in v1 acceptable, or should "talk to staff" create a task only until Phase 2?
6. **Languages at launch:** en + hi menus; others model‑only?
7. **Model:** Grok as per the agent plan, or OpenAI for the guest channel until the xAI data‑retention question (agent plan §12‑1) is settled?
8. **Pricing:** standalone per‑property add‑on vs included with the Front Desk bot; flat price level.
9. **Proactive messages in scope?** Pre‑arrival reminders are planned (opt‑in); marketing/promotions are explicitly out.
10. **Standalone CM tenants:** out of scope (bots require the PMS as the system of record) — confirm.

---

## Sources (checked 2026‑10‑17)
- Twilio — Register WhatsApp senders (ISVs): one WABA per account, subaccount per customer, Senders API, OTP rules: https://www.twilio.com/docs/whatsapp/isv/register-senders
- Twilio — Senders API GA, typing indicators, Flows, ISV changes (Nov 2025): https://www.twilio.com/en-us/blog/products/launches/launch-whatsapp-faster-with-twilio--senders-api-ga--plus-typing-
- Twilio — `twilio/quick-reply` (in‑session 3 buttons, `ButtonText`/`ButtonPayload`): https://www.twilio.com/docs/content/twilio-quick-reply · list picker: https://www.twilio.com/docs/content/twiliolist-picker
- Meta — WhatsApp Business Platform pricing (per‑message since Jul 2025; 24 h window; 2026 changes): https://developers.facebook.com/docs/whatsapp/pricing
- respond.io — Oct 2026 change: service messages chargeable after 1,000/number/month, in‑window utility templates chargeable: https://respond.io/blog/whatsapp-pricing-change-2026
- AiSensy — India per‑message rates (utility/auth ₹0.145, marketing ₹1.09): https://m.aisensy.com/blog/whatsapp-per-message-pricing-update-effective-january-1-2026/
