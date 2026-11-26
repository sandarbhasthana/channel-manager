# Handoff — PMS staff mobile app (pms-app + pms-mobile)

Updated 2026-10-29 (after MOB-8 — typed client, persisted cache, offline task queue, Tasks tab — and MOB-7 — hardware device key, invitations, signed-out and update screens). Next up: §5. Start a new chat in the **channel-manager** project with:
"Read `claude/handoff.md` and `claude/mobile-app-plan.md`, then continue the staff app."

The full plan (architecture, tickets, estimates) is the project doc `claude/mobile-app-plan.md`.
The same file is in `pms-app/docs/MOBILE_APP_PLAN.md`, which is git-ignored. This file covers
where things stand and how to keep working.

---

## 1. Where everything is

| What | Where | State |
|---|---|---|
| pms-app (Next.js 15.5.9, next-auth 4.24.13, Prisma 7.9.1) | Mac: `~/Desktop/Study/pms-app` · GitHub `sandarbhasthana/pms-app` | Branch **`feat/mobile-app` @ `1bdcd4e`** (`1bdcd4e` dev:set-password shows the real DB error; `cc4b7b7` MOB-7 invite routes; `1a71a7c` MOB-4 contract; `8183548` MOB-6 token export). **`cc4b7b7` and `1bdcd4e` are unpushed.** Local DB: all 80 migrations applied (incl. `mobile_idempotency_records`, confirmed 10-29); `User.platformAdminGrantedAt` was added by hand on 10-29 (see §4). Migration `20261027120000_mobile_idempotency_records` must be applied on the Mac (`npx prisma migrate deploy`) before idempotent task calls work locally — **not yet confirmed** (the VM can't reach the Mac's Postgres; check with `npx prisma migrate status`). Contains GitHub `master` (`9e51f45`, PR #25) as an ancestor, so merging it into master is a **fast-forward**. |
| pms-app master | same | `9e51f45` = origin/master. Local `fix/access-hardening` (`9765fa7`) is superseded; safe to delete. |
| pms-mobile (Expo SDK 58, RN 0.88, React 19.3, TS 6, expo-router 58) | Mac: `~/Desktop/Study/pms-mobile` · GitHub `sandarbhasthana/pms-mobile` | `main` @ **`a4cdbd7`**, **9 commits ahead of origin, unpushed** (`34e6e5f`…`0122603` from 10-28, `6321236` MOB-8, `62580a3` storage fallback, `66c3994` tab highlight, `6e919e7` MOB-7, `a4cdbd7` device-key label). 161 tests across 27 files. **pnpm 10.28.1** (`packageManager`, `pnpm-lock.yaml`; CI `pnpm install --frozen-lockfile`). |
| Plan | project `claude/mobile-app-plan.md` and `pms-app/docs/MOBILE_APP_PLAN.md` | Progress table updated through MOB-8. |
| Feature log | `pms-app/docs/APP_FEATURES.md`, `docs/APP_MAP.md` | Tracked in git even though `docs/` is ignored, so use `git add -f`. |

**Native modules:** MOB-8 added `react-native-mmkv`, `react-native-nitro-modules`, `expo-network`, `expo-crypto`; MOB-7 added the local `modules/device-key`. After pulling, `pnpm install` **finished**, then rebuild the dev client (`pnpm expo run:android`). On 10-29 the first MOB-8 build ran before `pnpm install` had finished and shipped without MMKV — check the autolinking list (`android/build/generated/autolinking/autolinking.json`) if a native module seems missing.
**That list is cached on the hash of `package.json` + npm/yarn lockfiles — React Native's Gradle plugin ignores
`pnpm-lock.yaml`.** A build that runs while `pnpm install` is still going caches an incomplete list under the final
`package.json` hash and every later build reuses it. Fix: delete `android/build/generated/autolinking/` and rebuild
(done 2026-10-29 for MMKV / Nitro). **Verified on the Android emulator 2026-10-29:** MMKV + Nitro linked (no storage warning), `modules/device-key` compiles
and registers (`com.pmsbeds.devicekey.DeviceKeyModule`), sign-in creates the key and binds the session — Account →
Devices shows "sdk_gphone16k_arm64 · This device · software key" (emulators have no TEE; real phones report "secure
hardware" / "StrongBox"). **iOS (Swift, Secure Enclave) has not been built yet.**

## 2. Decisions that are locked

1. **Admins are the owners. There is no OWNER role anywhere.** Migration `20261019120000_remove_owner_role`
   turns OWNER into ORG_ADMIN. Master's read-only OWNER branches (content sync, property secrets,
   policy/FAQ export) were removed in `7ff4211`. Don't reintroduce any owner-only behaviour.
2. **Sign-out happens only two ways:** the user taps *Sign out*, or an admin revokes access. The six revoke
   reasons are `USER_SIGNED_OUT`, `USER_REMOVED`, `USER_DEACTIVATED`, `ACCESS_REMOVED`, `ADMIN_SIGNED_OUT` and `ORG_DEACTIVATED`.
   Never sign out on idle time, network errors, 5xx, an expired access token, an update or a bad token.
3. **Phone and tablet layouts are both designed for v1.** Nothing is ported from the desktop layout.
   Size classes come from the window: compact under 600 dp, medium 600–839 dp, expanded 840 dp and up.
4. **Calendar:** tablets get a native tape chart (Skia). Phones get a day board, a 3-day timeline, an
   agenda and a month heatmap, and edit through sheet pickers instead of drag (plan §2.11).
5. **Separate `pms-mobile` repo** (confirmed 2026-10-28, decision 9).
6. **Access hardening uses master's design** (PRs #26–#35):
   - `requireOrgContext` / `requirePropertyOrgContext` in `src/lib/auth/org-scope.ts`.
   - Platform admin is `User.isPlatformAdmin`, re-read from the database on every check.
   - The migration `20261024060000_user_platform_admin_flag` is fail-closed, so nobody is a platform admin
     until someone runs `scripts/setup-platform-superadmin.ts`.
   - Our additions on top: active-user and active-org checks (`cc3579a`).
7. **Colours come from the web's CSS, never typed into the app** (MOB-6). Change palette.css / theme.css in
   pms-app → `npm run tokens:export` (pms-app) → commit `design-tokens/cellar.json` → `pnpm tokens:sync`
   (pms-mobile) → commit. The only hand-written part is the `MAP` in `pms-mobile/scripts/sync-tokens.mjs`.
8. **Android bottom navigation is the reference "Material 3 bar, Cellar tint"** (Sandarbh's brand zip
   "Mobile Bottom Nav", 2026-10-28), not the stock NativeTabs bar: `pms-mobile/src/features/shell/PersonaTabs.android.tsx`
   + `src/navigation/bottom-bar/` (`a1a52b2`). iOS keeps NativeTabs. Spec: 80 dp on `sunken`, 64×32 `brandSubtle` pill
   (400 ms emphasized decelerate), Material Symbols Rounded 24 outline→filled, Jost 12 labels, 12 % flash, `danger`
   badges, screen slides 16 dp from the direction of travel. New tab icon → re-run `scripts/subset-tab-icons.py`
   (the test fails until you do). **iOS keeps the native tab bar** (answered 2026-10-28). **Android tablets get a
   navigation rail in the same style** (same icons, pill, motion, badges; vertical, on medium / expanded windows) —
   to build with Spike S5; until then the bar shows at every width.
9. **Answers to the plan's open questions (§12), 2026-10-28** — each changes scope; fold into the tickets named:
   - **Sign-in without email: both SMS code and QR pairing.** SMS (6-digit, Twilio, hashed, 10 min, 5 attempts) is
     the default; a manager-shown QR pairs a phone for staff without a usable number. Server: `auth/otp/start|verify`
     and a QR pairing route (short-lived, single-use token minted by a manager, bound to the invite / user);
     app: OTP and scan screens. Goes with MOB-7 (+ MOB-14 for the manager side).
   - **Shared devices are needed in v1.** Design them as *switching people on one device*, never as signing out:
     each person keeps their own device session; the shared install holds several, with a quick switcher (PIN or
     biometric per person), an idle return to the switcher, and per-person caches / push routing. Needs a design pass
     and a server change (several sessions per installation, `MobileDeviceSession.@@unique([userId, installationId])`
     already allows it). Plan §2.2 / §4 to update; new ticket, after MOB-8 and before the persona features.
     MOB-8 note: the persisted cache and the offline queue are per install today (one MMKV instance `pms-cache`,
     owner check `userId:orgId`); shared devices need one per person.
   - **Housekeeping is assigned per attendant, ordered by priority**, not a shared board. Sandarbh's description:
     assignment answers *who will do it*, priority answers *how urgent it is*. Workflow: (1) identify the work —
     checkout clean, stayover service, deep clean, turndown, extra guest request; (2) allocate it to attendants —
     **auto-allocate, then a supervisor reviews and adjusts** (by floor, staff on shift and workload; effort in
     **credits**, e.g. a suite or checkout clean counts more than a light service — cf. Oracle OPERA task-sheet
     generation); (3) order each attendant's list by priority — a room a guest is waiting for is urgent, and a
     guest's requested service time also sets when it happens; exact ordering follows hotel policy / settings.
     New data model (housekeeping task sheet / assignment per day, credits per task type, property policy for
     priority) + supervisor screen + attendant "my rooms" list. Replaces the shared-board default in MOB-19 (and
     MOB-13's board feed); was phase 3, now v1.
   - **Critical alerts: apply to Apple now** (Sandarbh, from the Apple developer org account; lead time). Design still
     works without it (time-sensitive + repeat until acknowledged).
   - **More approval types in v1: refunds, cancellations inside the penalty window, discounts / comps** (on top of
     early check-in, stay-shortening credit, late checkout and FAQ revisions). Server: create approval requests at
     those decision points (new `ApprovalRequest` types and thresholds per property), producers notify managers;
     app inbox already planned to be type-generic. MOB-11 / MOB-13 / MOB-14 / MOB-17.
   - **Lock screen shows full details always** (guest names and request text), overriding the plan's room +
     category default. Push payloads carry them; Android channels use `lockscreenVisibility: PUBLIC`; widgets /
     Live Activities may show names. Update plan §2.5 / §2.7 / §6 and the store privacy labels.
   - **Keep two repos** (pms-app + pms-mobile); contract and tokens stay vendored with drift checks.
   - **Remove content sync's read-only (masked) suggestions route and the panel's read-only mode** (pms-app, unused
     now that owners are admins) — small cleanup task.
   - **Branding: keep the placeholders** ("PMS Staff", `in.pmsbeds.staff`) until store submission.

## 3. What's built

**pms-app (`feat/mobile-app`)**
- **MOB-0** (pre-requisite fixes from the plan's audit):
  - OWNER removed.
  - Org context verified (master's helper).
  - Platform-admin flag.
  - Invite accept with password at `/auth/invite/[token]`.
  - Early check-in approval goes through `applyStatusChangeInTx`.
- **MOB-1:**
  - **Data:** the `MobileDeviceSession` table (migration `20261019130000`).
  - **Access token:** a NextAuth JWE with `typ:"mobile"` and `sid`, valid 10 min.
  - **Middleware bridge:** turns `Authorization: Bearer` into the session cookie.
  - **Routes:** `/api/mobile/v1/auth/{login,refresh,logout}`.
  - **Refresh token:** rotates on every use, and its successor can be replayed.
  - **Device proof:** a P-256 signature of `"pms-refresh-v1\n{installationId}\n{ts}\n{refreshToken}"`.
  - **Unbound dev devices:** allowed only when `MOBILE_ALLOW_UNBOUND_DEVICES=true` and not in production.
- **MOB-2:**
  - **Revocation:** `src/lib/mobile/auth/revoke.ts` is hooked into user remove, deactivate/reactivate
    (`PATCH {isActive}` plus the Staff menu), the user edit and `/api/user-properties*`. Open tasks go back to the queue.
  - **Session callback:** checks the device session on every request (15 s cache), so revocation takes effect in seconds.
  - **Device lists and sign-out:** `/api/mobile/v1/sessions[/:id]` and `/api/admin/users/[id]/devices[/:sessionId]`.
- **MOB-3:**
  - `src/lib/mobile/capabilities.ts` maps roles to personas: ORG_ADMIN→owner, PROPERTY_MGR→manager,
    FRONT_DESK/GUEST_SERVICES→frontDesk, HOUSEKEEPING, MAINTENANCE, others→basic.
  - Routes: `GET /api/mobile/v1/me` (ETag), `GET /config`, `POST /session/org`.
  - Requests with `x-app-version` below `MOBILE_MIN_APP_VERSION` get 426.
- **MOB-6 (`8183548`):** `npm run tokens:export` / `tokens:check` (`scripts/export-design-tokens.ts`, logic in
  `src/lib/design-tokens/css-tokens.ts`). Resolves `var()` per theme, evaluates `color-mix(in srgb)` (premultiplied,
  verified against theme.css's documented `--attention-ink` values), reads radii / type scale / `.badge-tint` recipe.
  `src/lib/design-tokens/__tests__` fails if `design-tokens/cellar.json` is stale — re-export after CSS changes.
- **MOB-4 (`1a71a7c`):** API contract + facade.
  - `src/lib/mobile/contracts/`: Zod 4 schemas (`import { z } from "zod/v4"` — zod 3.25 bundles it; the rest of the app stays on Zod 3),
    registered with `named()`; `endpoints.ts` lists every `/api/mobile/v1` route (id, method, path, auth, propertyScoped,
    idempotent, forwardsTo, request, responses). `openapi.ts` builds `contracts/mobile-v1.openapi.json`
    (`npm run mobile:openapi`; on the VM: `UPDATE_MOBILE_OPENAPI=1 jest src/lib/mobile/contracts`).
  - **Adding a mobile route:** add the schemas + a registry entry, write the route, regenerate the JSON, then
    `pnpm api:sync` in pms-mobile and commit both. The route-sync test fails if a route file and the registry disagree;
    forwarders must `export { GET } from "@/app/<web route>/route"` with the same dynamic segment names.
  - Forwarders: `/api/mobile/v1/tasks` (GET, POST) and `/tasks/[id]` (GET, PATCH) → `/api/guest-requests*`.
  - `src/lib/mobile/idempotency.ts` `withIdempotency(operationId, handler)` on mutations; `MobileIdempotencyRecord` table.
    Answers: same key + same request → stored answer with `Idempotent-Replayed: true`; still running → 409
    `idempotency_in_progress`; other request → 422 `idempotency_key_reused`. The fingerprint includes `x-property-id`
    and the body — which is why the app captures both when an action is queued.
  - Contract tests: `src/lib/mobile/contracts/__tests__/{openapi,tasks.contract,session.contract}.test.ts`.
  - Persona / capability names now live in `src/lib/mobile/capability-names.ts`.
- `npm run dev:set-password -- <email>` gives a local dev account a password (default `PmsDev@2026`, local only). The seed
  (`prisma/seed.ts`) creates `admin@example.com` (ORG_ADMIN), `manager@example.com`, … **without passwords** (dev login
  only) — run this before signing in on the app. `admin@example.com` has `PmsDev@2026` as of 10-29.

**pms-mobile (`main`)**
- **MOB-5 scaffold:**
  - Setup: EAS profiles, CI (tokens:check, api:check, typecheck, lint, test) and Cellar tokens in `src/theme/`.
  - Session core: `src/auth/session-manager.ts` (single-flight signed refresh), `auth-errors.ts` (sign-out
    rules), `secure-token-store.ts`, `device-signer.ts` (unbound dev signer) and `SessionProvider.tsx`.
  - Routing: `Stack.Protected` with `redirectTo` (`src/navigation/SessionStack.tsx`).
- **MOB-9 first pass:**
  - `/me` → current property (remembered) → that persona's route group with `NativeTabs`
    (`src/app/(app)/(owner|manager|frontdesk|housekeeping|maintenance|basic)/`, guarded by `src/navigation/PersonaStack.tsx`).
  - Account, More and Me work: org switch, property sheet, devices with remote sign-out, Sign out.
    Tabs other than Tasks are placeholders naming their ticket.
- **MOB-6 (`bd0222d`):**
  - Tokens: `design-tokens/cellar.json` (vendored) → `scripts/sync-tokens.mjs` → `src/theme/tokens.generated.ts`;
    `src/theme/tokens.ts` re-exports with types. `src/theme/color.ts` = the web's `.badge-tint` maths + contrast.
  - Kit, import from `@/ui`: Text (tones; `attention` uses the readable ink, `money` is clay), Card, Section,
    ListRow (44 / 56 pt), StatusChip + `tints.ts` (web's reservation-status / role / property-status tables),
    Badge, Button (primary / secondary / ghost / destructive, `lg` = 56 pt), KpiTile, Banner, EmptyState,
    Skeleton(Lines), Avatar, SwipeRow (actions also as accessibility actions), TextField, GlassCapsule,
    and since MOB-8 `Segmented` (JS stand-in until Expo UI segmented controls) and `UpdatedStamp`.
    `ScreenHeader` (title + property switcher) is exported from `src/ui/Screen.tsx`.
  - `src/native`: `haptic.*`, `floatingMaterial()` (glass iOS 26+ / blur / solid), `useReduceTransparency()`.
  - `src/layout`: `SplitView` (list-detail from 600 dp, list pane 320–400 dp), `AdaptiveSheet`
    (`useAdaptiveSheetOptions()` for the Stack.Screen + `<SheetSurface>` in the route — bottom sheet on phones,
    iPad form sheet, centred card on Android tablets). Property switcher uses it.
  - Dev-only `/kit-gallery` (`pmsstaff://kit-gallery`) shows every component in both themes; redirects home in release.
  - Tests: AA contrast for every chip tint (task status / priority tints included) and text tone in both themes; Jest uses
    `jest.resolver.js` (pnpm-safe copy of Reanimated's rules) + `jest.setup.js`; RNTL 14 is async (`await render`, `await fireEvent`).
- **pnpm (`0122603`):** `packageManager: pnpm@10.28.1`, CI on pnpm, `jest.resolver.js`. Import types only from direct
  dependencies (pnpm's isolated layout hides transitive ones — `sf-symbols-typescript` bit us).
- **Android tab bar (`a1a52b2`, fixed in `66c3994`):** see decision 8. The selected destination comes from
  `useTabTrigger().getTrigger(name).isFocused` — **not** `TabsStateContext`, which expo-router 58's `<Tabs>` never
  provides (the bar used to stay on the first tab). `src/features/shell/__tests__/persona-tabs-android.test.tsx`
  renders the real Android tabs under the router. `62580a3`: without NitroModules in the build, storage falls back
  to memory with one warning instead of a red error. Error classes restore their prototype (`src/lib/errors.ts`, `b8a0b1b`) —
  `instanceof` on Error subclasses failed on the device bundle; behaviour checks use `isErrorNamed()`. The load
  screen names the cause (`2588386`).
- **MOB-8 data layer (`6321236`):**
  - **Contract:** `contracts/mobile-v1.openapi.json` vendored. `pnpm api:sync` copies it from `../pms-app` (or `$PMS_APP_DIR`)
    and runs orval (`orval.config.mjs`, orval **7.21.0**, `client: react-query`, `mode: tags-split`) into
    `src/api/generated/` (committed, ESLint-ignored). `pnpm api:check` (CI, `pnpm check`) regenerates into a scratch copy
    under `node_modules/.cache` and fails on any difference, or when a sibling pms-app's spec differs from the vendored one.
    It also fails if orval silently dropped the mutator's options argument (it does when it can't bundle the mutator).
  - **Mutator:** `src/api/mutator.ts` `apiFetch<T>(config, { propertyId, idempotencyKey })` → `ApiClient.request`
    (Bearer, refresh-and-retry once, `x-app-version`, `x-platform`, `x-property-id`, `Idempotency-Key`). Non-2xx throws
    `ApiError` (`src/api/errors.ts`: `status`, `code`, `replayed`; helpers `isConflict`, `isIdempotencyInProgress`, `isRetryable`).
    Header params are left out of generated signatures on purpose.
  - **One client:** `SessionProvider` creates the `SessionManager` and the `ApiClient` together and binds the client in
    `src/api/runtime.ts` (`apiClient()` waits until bound), so generated fetchers and the queue work outside React.
    `useApi()` reads it from the session; the `ApiProvider` component is gone. `/me` types (`Me`, `MeProperty`,
    `Persona`, `Capability`) are re-exported from the generated model.
  - **Property scoping:** `propertyQueryOptions(propertyId, (request) => get…QueryOptions(params, { request }))` puts
    `["property", propertyId, …]` in the key and the same id in `x-property-id`. Every property query must use it.
    The Tasks screen is keyed by property id as well (selection / view reset on switch).
  - **Persistence:** `PersistQueryClientProvider` in `src/app/_layout.tsx`; `src/api/persistence.ts` uses
    `@tanstack/query-async-storage-persister` over MMKV (`src/api/storage.ts`, instance `pms-cache`; the sync persister is
    deprecated in 5.105). `maxAge` 24 h = `gcTime`; `buster` = `CACHE_SCHEMA_VERSION` (bump only for incompatible persisted
    shapes — it also drops unsent queued actions). `meta: { persist: false }` keeps a query in memory. `wipeLocalCache()` on
    sign-out, revocation and sign-in; `CurrentPropertyProvider` drops leftovers whose `userId:orgId` owner differs from `/me`.
  - **Online / focus:** `src/api/network.ts` wires `onlineManager` to `expo-network` (unknown = online) and `focusManager` to
    `AppState`; `useOnline()`. `src/features/shell/SyncBanners.tsx` = offline banner (with unsent count) + queue notices.
  - **Offline queue:** `src/api/offline-queue.ts` (generic: `QUEUED` key prefix, `shouldPersistMutation`, `newIdempotencyKey`,
    `resumeQueuedActions` — also continues requests that were in flight when the app died, `settleQueuedActions`,
    `useUnsyncedActionCount`) and `src/features/tasks/queue.ts` (`TASK_STATUS_ACTION`, `registerTaskQueue` defaults,
    `queueTaskStatus`, `usePendingTaskStatuses`). Variables carry `propertyId`, `idempotencyKey`, `queuedAt`, `label`
    captured at queue time. `networkMode: "online"`, retried while `isRetryable`, backoff ≤ 60 s (2 s for
    `idempotency_in_progress`), scope `task:<id>` so steps replay in order. 409 → "Already updated by someone else"
    notice (`src/api/notices.ts`); other refusals → "Change not saved". All other mutations default to
    `networkMode: "always"`, `retry: 0` (online-only). Sign-out warns about unsent changes; org switch first waits
    for the queue and refuses (`UnsyncedChangesError`) if something is still unsent.
  - **Tasks tab:** `src/features/tasks/` (`TasksScreen`, `TaskDetailView`, `TaskRow`, `TasksStack`, `hooks`, `model`, `routes`).
    Routes `src/app/(app)/(manager|frontdesk|housekeeping|basic)/tasks/{_layout,index,[id]}.tsx`. Now / Active / Upcoming
    (counts), Everyone / Mine, one page of 100. Steps: buttons + swipe from `allowedStatuses`; after a queued step the next
    ones follow the web's `STATUS_TRANSITIONS` (mirrored in `model.ts`). Chips mirror `tasks.css`. Detail opens from
    the list's cached copy (placeholder) before `tasks.get` answers. Instants in the property timezone (`src/lib/time.ts`),
    stay dates without shift. Links use `taskHref(persona, id)` (group-qualified).
  - **Bug found on the way:** jest-expo's `expo-crypto` stub returns `undefined` from `randomUUID()` — keys are now
    validated against the server's pattern with fallbacks.
- **MOB-7 (`6e919e7`, server `cc4b7b7`):**
  - **Server:** `POST /api/mobile/v1/auth/invite/lookup {token}` (the web's `loadInvitation` view) and
    `POST /api/mobile/v1/auth/invite/accept {token, name, password?, installationId, devicePublicKey, device}` in
    `src/lib/mobile/auth/invite.ts`. Tokens in the body, never the URL. The device key is checked **before** the token is
    used. Accounts that already had a password are not signed in by the invitation (`signIn: "password"`). Session creation
    is `startMobileSession()` + `deviceKeyProblem()` in `sessions.ts`, shared with login.
  - **`modules/device-key`** (local Expo module, autolinked from `modules/`; classic `Module` / `ModuleDefinition` DSL like
    SDK 58's expo-network — the `@ExpoModule`/`@JS` macro API is beta): `hasKey`, `getOrCreatePublicKey` (base64 SPKI; iOS adds
    the 26-byte P-256 SPKI header to `SecKeyCopyExternalRepresentation`), `sign` (DER base64), `deleteKey`, `describeKey`.
    iOS: Secure Enclave, `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` + `.privateKeyUsage` (simulator: keychain key).
    Android: Keystore `secp256r1`, StrongBox when present (falls back to TEE), no user auth. Package `com.pmsbeds.devicekey`
    (not `in.…` — `in` is a Kotlin keyword). Alias `session-v1`. JS: `modules/device-key/index.ts` via
    `requireOptionalNativeModule` from `expo` (null in Jest / old dev clients).
  - **`src/auth/device-signer.ts`:** `DeviceIdentity` = signer + `publicKey()` + `keyState()`; `selectDeviceIdentity` → hardware
    whenever the module exists (dev too), else unbound only for opted-in dev builds, else `unavailable` (sign-in refused on the
    device with a clear message). A keyless (pre-MOB-7 dev) session keeps refreshing with the empty proof.
  - **Restored install:** `secure-token-store.ts` keeps a "bound" marker. At launch, refresh token + bound marker + `hasKey` ===
    false → local sign-out, `resetInstallationId()`, cache wipe, sign-in says why. Only a definite "missing" counts; key-store
    errors keep the session. Tested in `src/auth/__tests__/session-provider.test.tsx`.
  - **Screens** (`src/features/auth/`): `InviteScreen` (`/invite/[token]`; `+native-intent.tsx` maps `/m/invite/<token>`),
    password rules mirrored from pms-app (`password-rules.ts`), `RevokedScreen` (`/revoked`, copy per reason in
    `src/auth/revocation-copy.ts`; `SessionStack` redirects there while `revocation` is set), `UpdateRequiredScreen` (root, on
    any 426 — `ApiClient.onUpdateRequired` and the refresh — session kept; "Check again" calls `/config`). Sign-in takes
    `email` / `joined` params. Account → Devices shows where the key lives.
  - **Not done:** biometric lock (plan MOB-7), SMS / QR sign-in (decision 9), opening an invite link while signed in (the
    (auth) guard sends it home), an iOS App Store link on the update screen (needs the store id).
- **Routing rule:** personas share tab paths (`/tasks`, `/today`), and a bare path resolves to the first group that has it.
  So **every link must be group-qualified via `personaHref()`** in `src/features/session/personas.ts`.
- `pnpm typecheck` first runs `scripts/typed-routes.cjs`, which regenerates `.expo/types/router.d.ts`
  (it was stale locally and missing in CI, so bad route strings type-checked).
- Verified 2026-10-20 on an Android emulator (Pixel 9 Pro), signing in through ngrok to a local pms-app.
  MOB-6 and the bottom bar were checked visually through a web export of `/kit-gallery`; Sandarbh ran the app on the
  emulator on 2026-10-28. **MOB-8 is verified by tests only** (`pnpm check` green on a clean `--frozen-lockfile` install);
  not yet run on a device.

## 4. Running it locally

**pms-app** (`.env` already has `MOBILE_ALLOW_UNBOUND_DEVICES=true`):
1. `npm run dev` (pms.local).
2. Expose it with ngrok. The middleware allows `*.ngrok-free.dev/.app`, `ngrok.app` and `ngrok.io`.
   If sign-in on the phone says "Can't reach the server", the tunnel is down.
3. Optional env: `MOBILE_ACCESS_TTL_SECONDS` (default 600), `MOBILE_MIN_APP_VERSION`, `MOBILE_LATEST_APP_VERSION`,
   `MOBILE_MAINTENANCE_MESSAGE`, `MOBILE_SUPPORT_EMAIL`.

**pms-mobile:**
1. `corepack enable` once, then `pnpm install`. Never `npm install` (it writes a package-lock and a hoisted tree).
2. Set `.env.local` from `.env.example`: `EXPO_PUBLIC_API_URL=<ngrok URL>`, `APP_VARIANT=development`,
   `EXPO_PUBLIC_ALLOW_UNBOUND_DEVICE=true`.
3. Run `pnpm expo run:android` (needs an AVD; start the emulator first) or `pnpm expo run:ios`. Then use `pnpm start:dev-client` for later JS-only runs.
   Rebuild the dev client whenever a native module is added (**MOB-8 added MMKV, nitro modules, expo-network, expo-crypto**) or fonts change.
   Emulator without ngrok: `adb reverse tcp:4001 tcp:4001` and `EXPO_PUBLIC_API_URL=http://localhost:4001`, then `--clear`.
4. `pnpm check` runs tokens:check, api:check, typecheck, lint and tests (what CI runs). Add Expo packages with `pnpm expo install`;
   non-native dev tools with `pnpm add -D`.

**MOB-8 acceptance still to run on the emulator** (needs the idempotency migration applied):
- Kill pms-app (or stop ngrok), cold-start the app: last task list shows at once with "Updated N min ago"; still signed in.
- Airplane mode → Acknowledge a task → row shows "Waiting to sync" → reconnect → it replays once: one `STATUS_CHANGED`
  on the web task timeline. Replaying the same key again (e.g. kill the app mid-request) answers `Idempotent-Replayed: true`.
- Switch property: no flash of the previous property's tasks.

**One-time local DB fix-up, if not done yet.** Your dev DB ran our dropped `20261019110000_platform_admin_flag` migration:
```sql
ALTER TABLE "User" ADD COLUMN IF NOT EXISTS "platformAdminGrantedAt" TIMESTAMP(3);
DELETE FROM "_prisma_migrations" WHERE migration_name = '20261019110000_platform_admin_flag';
```
```bash
npx prisma migrate resolve --applied 20261024060000_user_platform_admin_flag
npx prisma migrate deploy
```
Run the SQL **before** `migrate resolve`. On 2026-10-29 the resolve had been done without the `ALTER TABLE`, so
`migrate deploy` said "no pending migrations" while every `User` write failed with "column
`User.platformAdminGrantedAt` does not exist". Fix: `echo 'ALTER TABLE "User" ADD COLUMN IF NOT EXISTS
"platformAdminGrantedAt" TIMESTAMP(3);' | npx prisma db execute --stdin`. Drift check:
`npx prisma migrate diff --from-config-datasource --to-schema prisma/schema.prisma --script` (empty = in sync).

**Deploy notes:**
- When `feat/mobile-app` reaches master, Railway runs `remove_owner_role` and `mobile_device_sessions` at startup.
- After master's platform-admin migration, grant platform admins with
  `SUPERADMIN_PASSWORD=... npx tsx scripts/setup-platform-superadmin.ts --email <addr>`.

## 5. What to build next

Server work stays on pms-app `feat/mobile-app`; app work on pms-mobile `main`.

1. **Finish the device checks** (Android emulator; the dev client is current and the session is key-bound):
   - MOB-8 (§4 acceptance): cached task list with the server down; Acknowledge in airplane mode → reconnect → one
     `STATUS_CHANGED` on the web timeline; property switch without a flash.
   - MOB-7: a key-bound refresh succeeds (stay in the app > 10 min, or restart after the access token expires); accept a
     fresh invitation (`adb shell am start -a android.intent.action.VIEW -d "pmsstaff://invite/<token>"`); remove the
     user on the web → `/revoked`; `MOBILE_MIN_APP_VERSION=9.9.9` → update screen, still signed in after clearing it.
   - iOS: first build of `modules/device-key` (Swift) on a simulator / device when Xcode is available (`pnpm expo run:ios`).
2. **MOB-13 / MOB-14** server feeds and actions as screens need them — every new route goes through the contract
   registry (`src/lib/mobile/contracts/endpoints.ts`), then `pnpm api:sync` in the app. When housekeeping status,
   cleaning events and room notes routes land (MOB-14), register them as queued actions like `features/tasks/queue.ts`
   (key + property captured at queue time, own scope per room). Everything else stays online-only.
3. **MOB-7 remaining:** optional biometric lock (plan §2.2 "Optional device lock"), with SMS / QR sign-in from decision 9.
4. **MOB-9 spikes:** S3 lists inside native tabs (FlashList / LegendList — Tasks uses a ScrollView for now, one page of 100);
   S5 Android tablet navigation rail (a rail in the same style as `CellarBottomBar` for medium / expanded windows — needs a design, see §8).
5. **MOB-D** phone + tablet designs before the persona features (MOB-16…20) — `/kit-gallery` is the component
   reference. Then notifications (MOB-10/11/12; badges feed `CellarBottomBar`) and the calendar (MOB-28/29/30).
6. MOB-6 leftovers when first needed: Expo UI (SwiftUI / Compose) segmented controls (replace `ui/Segmented`), pickers and menus.
7. **New v1 scope from decision 9** (each needs a short design / plan pass first):
   - SMS code + QR pairing sign-in (with MOB-7).
   - Shared devices as person switching (server: several sessions per installation; app: switcher, per-person cache and queue).
   - Housekeeping allocation: task sheets per attendant, credits, auto-allocate + supervisor review, priority order
     (replaces the shared board in MOB-13 / MOB-19).
   - Approval producers for refunds, late cancellations and discounts / comps (MOB-11 / MOB-14).
   - Lock-screen full detail in push payloads and widgets (MOB-10 / MOB-21 / MOB-22 / MOB-23).
   - pms-app cleanup: remove content sync's read-only suggestions route and panel mode.

## 6. Working conventions (for Claude in the next chat)

**Git**
- **Don't push.** Sandarbh pushes from his terminal. Commit with
  `git -c user.name=sandarbh -c user.email=sandarbh88@gmail.com` and the attribution trailers the session provides.
- In pms-app: never run `npm install` (Sandarbh does), never commit `src/generated`, and never commit his own uncommitted changes.
  Commits there run lint-staged (`eslint --fix`) — re-run affected tests after committing.
- For merges or big changes, work in a **git worktree** on a temp branch, then `--ff-only` / `reset --keep` the checked-out branch.
- `git merge -F -` fails on the VM's git 2.34: write the message to a file.

**Mac bridge (`device_bash`)**
- The repos are at `$HOME/mnt/pms-app` and `$HOME/mnt/pms-mobile` when those folders are connected (ask for
  `~/Desktop/Study/pms-mobile` if it isn't).
- Request delete permission once per session (git needs it for lock files). **It resets when the bridge reconnects** —
  if a commit warns "unable to unlink", ask again and remove only the `.lock` / `tmp_obj_*` files that commit left.
- The bridge can drop for minutes; nothing is lost on the Mac. Retry once, then wait for Sandarbh.
- `prisma generate` works on the VM:
  `PRISMA_ENGINES_CHECKSUM_IGNORE_MISSING=1 PRISMA_SCHEMA_ENGINE_BINARY=/bin/true DIRECT_URL=postgresql://x:x@localhost:5432/x DATABASE_URL=postgresql://x:x@localhost:5432/x ./node_modules/.bin/prisma generate`.
  A new worktree needs it (and a `node_modules` symlink). `migrate` can't run there; Sandarbh runs migrations on the Mac.
- pms-app: `tsc --noEmit -p tsconfig.json` needs `NODE_OPTIONS=--max-old-space-size=6144` on the VM; jest runs fine.
  **`tsx` does not run on the VM** (macOS esbuild binary) — to run a pms-app script, transpile with
  `./node_modules/.bin/tsc --outDir $HOME/x --module commonjs …` and run it with node from the repo root.
- pms-mobile: the Mac's `node_modules` (macOS binaries) can't run lint or the RNTL suites on the VM. Verify in a clean Linux copy:
  copy the repo without the **root-level** `node_modules/.git/.expo/android/ios/.env.local` (anchor the excludes — `--exclude /android`;
  an unanchored `android` also drops `modules/device-key/android`) into `$HOME/pmsm-fresh`, then
  `npx -y pnpm@10.28.1 install --frozen-lockfile` (~2 min cold, seconds with the store warm) and `npx -y pnpm@10.28.1 check`
  (run `api:check` with `PMS_APP_DIR=$HOME/mnt/pms-app` to also compare with pms-app). Re-sync with
  `rsync -a --delete --exclude node_modules --exclude .git --exclude .expo --exclude android --exclude ios --exclude dist` before each run.
- **Adding packages to pms-mobile:** run `pnpm add` / `pnpm expo install` **in the Linux copy**, then copy only
  `package.json` + `pnpm-lock.yaml` back to the Mac — never install into the Mac's `node_modules` from the VM.
  For native modules take versions from `node_modules/expo/bundledNativeModules.json` when listed.
- Jest uses `pms-mobile/jest.resolver.js`, not Reanimated's: the upstream resolvers break under pnpm (see the file).
- Jest hygiene: a test that creates a `QueryClient` must `clear()` it (and `destroy()` its mutations first) or a worker
  hangs ("failed to exit gracefully" only shows with several workers, never in a single-file run). RNTL 14's `act` is
  async — never call it un-awaited; set `onlineManager` before render instead. Rows appear a tick after render: use `findBy…`.
- Visual check: `npx expo export --platform web --dev` in that copy, with `secure-token-store.ts` stubbed **in the copy only**
  (SecureStore has no web build), then stage the tarred `dist` and screenshot `/kit-gallery` with Playwright in the cloud workspace.
- Background processes die when a call ends, so run jest in chunks of about 26 files (fast once the cache is warm).
- `device_commit_files` reuses content per staged path, so use a new staged filename each time.
- The VM can't reach the Mac's localhost database.

**Tests and safety**
- pms-app suites that fail on master too (not regressions): `dashboard/unified`, `property-secrets/export-matcher`,
  `reservations/reschedule-inventory`.
- Don't fetch Sandarbh's ngrok URL.
- Several Prisma test stubs compare `where` keys with `===`. Relation filters (`user: {isActive}`) need the stub taught,
  for example `state.inactiveUsers` / `state.inactiveOrgs` in `src/test-utils/org-scope-test-fixtures.ts`.

## 7. Key files

- **pms-app:**
  - Mobile server code: `src/lib/mobile/auth/{tokens,bridge,device-proof,sessions,access,revoke,session-check,devices,device-admin,http}.ts`,
    `src/lib/mobile/{capabilities,me,version}.ts`, `src/app/api/mobile/v1/**`.
  - Middleware and auth: `src/middleware.ts` (the bridge runs first), `src/lib/auth.ts` (session callback mobile check),
    `src/lib/auth/org-scope.ts`, `src/lib/property-context.ts`.
  - Tokens: `src/lib/design-tokens/css-tokens.ts`, `scripts/export-design-tokens.ts`, `design-tokens/cellar.json`.
  - Contract: `src/lib/mobile/contracts/*`, `contracts/mobile-v1.openapi.json`, `src/lib/mobile/idempotency.ts`.
- **pms-mobile:** `src/auth/*`, `src/api/{client,mutator,runtime,errors,property-scope,persistence,storage,network,offline-queue,notices,query-client,ApiProvider}.ts`,
  `src/api/generated/` (orval), `src/features/session/*`, `src/features/shell/*`, `src/features/tasks/*`,
  `src/navigation/*` (incl. `bottom-bar/`), `src/theme/*`, `src/ui/*`, `src/native/*`, `src/layout/*`, `src/lib/{errors,time}.ts`,
  `src/test-utils/api.ts` (real ApiClient over a fake fetch, bound like the app), `orval.config.mjs`,
  `scripts/{sync-tokens.mjs,sync-api.mjs,typed-routes.cjs,subset-tab-icons.py}`, `jest.resolver.js`.

## 8. Open questions

All of plan §12 was answered on 2026-10-28 (decision 9). Still to do / decide:

1. **Store accounts** — who owns the Apple Developer and Google Play organisation accounts (needed before the critical-alerts
   request and store submission). App name / bundle id stay placeholders until then.
2. **Tell Ujjwal that owners = admins** (his 10-21 OWNER + property-manager ruling no longer applies) — Sandarbh's action.
3. Designs needed before building: shared-device switching (decision 9), the Android tablet rail (decision 8), the
   housekeeping supervisor allocation screen and attendant list (decision 9).
4. Update `claude/mobile-app-plan.md` sections that decision 9 changes (§2.2 sessions on shared devices, §2.5 / §2.7 / §6
   lock-screen detail, §3 housekeeping, §8 tickets and estimates) — the plan's §12 already records the answers.
5. The persisted cache is not encrypted (MMKV supports a key from SecureStore). With full lock-screen detail chosen it was
   left plain; say if the cache should be encrypted at rest.
