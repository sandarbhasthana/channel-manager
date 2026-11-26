# Handoff — PMS staff mobile app (pms-app + pms-mobile)

Updated 2026-10-27. Start a new chat in the **channel-manager** project with:
"Read `claude/handoff.md` and `claude/mobile-app-plan.md`, then continue the staff app."

The full plan (architecture, tickets, estimates) is the project doc `claude/mobile-app-plan.md`.
The same file is in `pms-app/docs/MOBILE_APP_PLAN.md`, which is git-ignored. This file covers
where things stand and how to keep working.

---

## 1. Where everything is

| What | Where | State |
|---|---|---|
| pms-app (Next.js 15.5.9, next-auth 4.24.13, Prisma 7.9.1) | Mac: `~/Desktop/Study/pms-app` · GitHub `sandarbhasthana/pms-app` | Branch **`feat/mobile-app` @ `7ff4211`**, pushed, clean. Contains GitHub `master` (`9e51f45`, PR #25) as an ancestor, so merging it into master is a **fast-forward**. |
| pms-app master | same | `9e51f45` = origin/master. Local `fix/access-hardening` (`9765fa7`) is superseded; safe to delete. |
| pms-mobile (Expo SDK 58, RN 0.88, React 19.3, TS 6, expo-router 58) | Mac: `~/Desktop/Study/pms-mobile` · GitHub `sandarbhasthana/pms-mobile` | `main` @ `1a7bafe`, pushed, clean. 55 tests across 9 files. |
| Plan | project `claude/mobile-app-plan.md` and `pms-app/docs/MOBILE_APP_PLAN.md` | Progress table updated through 2026-10-27. |
| Feature log | `pms-app/docs/APP_FEATURES.md`, `docs/APP_MAP.md` | Tracked in git even though `docs/` is ignored, so use `git add -f`. |

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
5. **Separate `pms-mobile` repo.** You asked earlier whether it could live inside pms-app as a module; that's still open (§8).
6. **Access hardening uses master's design** (PRs #26–#35):
   - `requireOrgContext` / `requirePropertyOrgContext` in `src/lib/auth/org-scope.ts`.
   - Platform admin is `User.isPlatformAdmin`, re-read from the database on every check.
   - The migration `20261024060000_user_platform_admin_flag` is fail-closed, so nobody is a platform admin
     until someone runs `scripts/setup-platform-superadmin.ts`.
   - Our additions on top: active-user and active-org checks (`cc3579a`).

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
- `npm run dev:set-password -- <email>` gives a local dev account a password (default `PmsDev@2026`, local only).

**pms-mobile (`main`)**
- **MOB-5 scaffold:**
  - Setup: EAS profiles, CI (typecheck, lint, test) and Cellar tokens in `src/theme/`.
  - Session core: `src/auth/session-manager.ts` (single-flight signed refresh), `auth-errors.ts` (sign-out
    rules), `secure-token-store.ts`, `device-signer.ts` (unbound dev signer) and `SessionProvider.tsx`.
  - Routing: `Stack.Protected` with `redirectTo` (`src/navigation/SessionStack.tsx`).
- **MOB-9 first pass:**
  - `/me` → current property (remembered) → that persona's route group with `NativeTabs`
    (`src/app/(app)/(owner|manager|frontdesk|housekeeping|maintenance|basic)/`, guarded by `src/navigation/PersonaStack.tsx`).
  - Account, More and Me work: org switch, property form sheet, devices with remote sign-out, Sign out.
    The other tabs are placeholders naming their ticket.
- **Routing rule:** personas share tab paths (`/tasks`, `/today`), and a bare path resolves to the first group that has it.
  So **every link must be group-qualified via `personaHref()`** in `src/features/session/personas.ts`.
- Verified 2026-10-20 on an Android emulator (Pixel 9 Pro), signing in through ngrok to a local pms-app.

## 4. Running it locally

**pms-app** (`.env` already has `MOBILE_ALLOW_UNBOUND_DEVICES=true`):
1. `npm run dev` (pms.local).
2. Expose it with ngrok. The middleware allows `*.ngrok-free.dev/.app`, `ngrok.app` and `ngrok.io`.
   If sign-in on the phone says "Can't reach the server", the tunnel is down.
3. Optional env: `MOBILE_ACCESS_TTL_SECONDS` (default 600), `MOBILE_MIN_APP_VERSION`, `MOBILE_LATEST_APP_VERSION`,
   `MOBILE_MAINTENANCE_MESSAGE`, `MOBILE_SUPPORT_EMAIL`.

**pms-mobile:**
1. Set `.env.local` from `.env.example`: `EXPO_PUBLIC_API_URL=<ngrok URL>`, `APP_VARIANT=development`,
   `EXPO_PUBLIC_ALLOW_UNBOUND_DEVICE=true`.
2. Run `npx expo run:android` (needs an AVD; start the emulator first) or `npx expo run:ios`. Then use `npm run start:dev-client` for later JS-only runs.
3. `npm run check` runs typecheck, lint and tests.

**One-time local DB fix-up, if not done yet.** Your dev DB ran our dropped `20261019110000_platform_admin_flag` migration:
```sql
ALTER TABLE "User" ADD COLUMN IF NOT EXISTS "platformAdminGrantedAt" TIMESTAMP(3);
DELETE FROM "_prisma_migrations" WHERE migration_name = '20261019110000_platform_admin_flag';
```
```bash
npx prisma migrate resolve --applied 20261024060000_user_platform_admin_flag
npx prisma migrate deploy
```

**Deploy notes:**
- When `feat/mobile-app` reaches master, Railway runs `remove_owner_role` and `mobile_device_sessions` at startup.
- After master's platform-admin migration, grant platform admins with
  `SUPERADMIN_PASSWORD=... npx tsx scripts/setup-platform-superadmin.ts --email <addr>`.

## 5. What to build next (suggested order)

Server work stays on pms-app `feat/mobile-app`; app work on pms-mobile `main` (or feature branches).

1. **MOB-6 Design tokens + UI kit:**
   - pms-app side: `scripts/export-design-tokens.mjs` writes `design-tokens/cellar.json` from
     `src/styles/palette.css` and `theme.css` (`:root` and `.dark`).
   - pms-mobile side: a sync script writes `src/theme/tokens.generated.ts`, plus a CI drift check.
   - UI kit: Cellar components (Text, Card, ListRow, StatusChip, Button, KpiTile, EmptyState, Skeleton, Banner, SwipeRow)
     and `layout/` (SplitView, AdaptiveSheet).
   - Jost font; Liquid Glass / blur / Material wrappers.
2. **MOB-4 Contracts + facade (pms-app):**
   - Zod schemas in `src/lib/mobile/contracts/*`, exported as OpenAPI 3.1 to `contracts/mobile-v1.openapi.json` (tracked).
   - Forwarder pattern (`export { GET } from "@/app/api/..."` plus segment config), contract tests and `withIdempotency`.
3. **MOB-8 Data layer (pms-mobile):** orval client generated from the OpenAPI file, TanStack Query + MMKV persistence,
   an offline mutation queue that replays with `Idempotency-Key`.
4. **MOB-7 remaining:** `modules/device-key` (Secure Enclave / Android Keystore P-256, non-exportable), which
   replaces the unbound dev signer. Plus the invite-accept screen and the revoked and update-required screens.
5. **MOB-9 spikes:**
   - **S3:** lists inside native tabs (try FlashList or LegendList).
   - **S5:** Android navigation rail on tablets by toggling `NativeTabs hidden` without remounting.
6. **MOB-D phone + tablet designs** before the persona features (MOB-16…20). Then notifications (MOB-10/11/12) and the calendar (MOB-28/29/30).

## 6. Working conventions (for Claude in the next chat)

**Git**
- **Don't push.** Sandarbh pushes from his terminal. Commit with
  `git -c user.name=sandarbh -c user.email=sandarbh88@gmail.com` and the attribution trailers the session provides.
- In pms-app: never run `npm install` (Sandarbh does), never commit `src/generated`, and never commit his own uncommitted changes.
- For merges or big changes, work in a **git worktree** on a temp branch, then `--ff-only` / `reset --keep` the checked-out branch.
- `git merge -F -` fails on the VM's git 2.34: write the message to a file.

**Mac bridge (`device_bash`)**
- The repos are at `$HOME/mnt/pms-app` and `$HOME/mnt/Study/pms-mobile`.
- Request delete permission once per session (git needs it for lock files). Clean only locks you created.
- `prisma generate` works on the VM:
  `PRISMA_ENGINES_CHECKSUM_IGNORE_MISSING=1 PRISMA_SCHEMA_ENGINE_BINARY=/bin/true DIRECT_URL=postgresql://x:x@localhost:5432/x DATABASE_URL=postgresql://x:x@localhost:5432/x ./node_modules/.bin/prisma generate`.
  A new worktree needs it (and a `node_modules` symlink). `migrate` can't run there; Sandarbh runs migrations on the Mac.
- `tsc --noEmit -p tsconfig.json` and jest run fine.
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
- **pms-mobile:** `src/auth/*`, `src/api/{client,ApiProvider,query-client}.ts`, `src/features/session/*`, `src/features/shell/*`,
  `src/navigation/*`, `src/theme/*`, `src/layout/{layoutClass,orientation}.ts`.

## 8. Open questions

1. Plan §12 is still open:
   - Sign-in for staff without email (SMS code / QR / email only).
   - Shared devices.
   - Housekeeping assignment model.
   - Applying for Apple critical alerts.
   - More approval types.
   - Lock-screen detail level.
   - App name / bundle id / store accounts (placeholders: "PMS Staff", `in.pmsbeds.staff`).
2. Move pms-mobile into pms-app as a module, or keep two repos?
3. Content sync's read-only (masked) suggestions route and the panel's read-only mode are unused now that owners are admins.
   Remove them?
4. Tell Ujjwal that owners = admins (his 10-21 OWNER + property-manager ruling no longer applies).
