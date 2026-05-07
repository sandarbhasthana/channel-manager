# WorkOS SSO Setup — Google & Apple

> **Scope:** Configure Google OAuth and Sign in with Apple as social login providers
> through WorkOS AuthKit for the Channel Manager platform.

---

## Prerequisites

| What                                     | Where                                                        |
| ---------------------------------------- | ------------------------------------------------------------ |
| WorkOS account                           | [dashboard.workos.com](https://dashboard.workos.com)         |
| Google account with Cloud Console access | [console.cloud.google.com](https://console.cloud.google.com) |
| Apple Developer account ($99/yr)         | [developer.apple.com](https://developer.apple.com)           |
| Channel Manager API running              | `http://localhost:8080`                                      |
| `.env` values already set                | `WORKOS_CLIENT_ID`, `WORKOS_API_KEY`                         |

---

## 1 — WorkOS Project Baseline

1. Log in to [dashboard.workos.com](https://dashboard.workos.com).
2. Select your environment (**Staging** for local dev, **Production** for live).
3. Go to **Configuration** → note your **Client ID** (already in `.env`).
4. Go to **Redirects** → add `http://localhost:8080/auth/callback` as an
   allowed redirect URI.

---

## 2 — Google OAuth Setup

### 2.1 Create a Google Cloud Project

1. Open [console.cloud.google.com](https://console.cloud.google.com).
2. Click the project dropdown (top bar) → **New Project**.
3. Name: `channel-manager` → **Create**.
4. Make sure the new project is selected in the dropdown.

### 2.2 Enable the Google People API

1. Navigate to **APIs & Services → Library**.
2. Search for **"Google People API"** → click it → **Enable**.

### 2.3 Configure the OAuth Consent Screen

1. Go to **APIs & Services → OAuth consent screen**.
2. Choose **External** (for any Google account) → **Create**.
3. Fill in the required fields:
   - **App name**: `Channel Manager`
   - **User support email**: your email
   - **Developer contact email**: your email
4. Click **Save and Continue** through Scopes (no extra scopes needed).
5. Under **Test users**, add any Google accounts you want to test with.
6. Click **Save and Continue** → **Back to Dashboard**.

> ⚠️ While the app is in **Testing** mode only test users can sign in.
> Publish the app (verify domain) before go-live.

### 2.4 Create OAuth Credentials

1. Go to **APIs & Services → Credentials → Create Credentials → OAuth client ID**.
2. Application type: **Web application**.
3. Name: `channel-manager-workos`.
4. Under **Authorised redirect URIs** click **Add URI** and enter the
   WorkOS callback URL for Google.

   > ⚠️ **The redirect URI is connection-specific.** Do **not** use a generic URL.
   > Find the exact URI in the WorkOS Dashboard:
   > **Authentication → Google OAuth → (your connection)** — it is shown on that
   > settings page and has the form:
   >
   > ```
   > https://auth.workos.com/sso/oauth/google/<connection_id>/callback
   > ```
   >
   > Example (your connection ID will differ):
   >
   > ```
   > https://auth.workos.com/sso/oauth/google/2UZE1PdUiMXfbxr7QmAZhTgff/callback
   > ```
   >
   > If you accidentally used the wrong URI and get a Google error saying
   > _"redirect_uri is not registered"_, add the exact URI shown in the
   > error's `redirect_uri=` parameter to the Google Cloud Console and save.

5. Click **Create**.
6. Copy the **Client ID** and **Client Secret** — you will paste these into WorkOS.

### 2.5 Add Google to WorkOS

1. In the WorkOS dashboard go to **Authentication → Social Login**.
2. Find **Google OAuth** → click **Configure**.
3. Paste the **Client ID** and **Client Secret** from step 2.4.
4. Toggle the provider to **Enabled** → **Save**.

---

## 3 — Sign in with Apple Setup

Apple requires more steps because it uses a private key instead of a secret.

### 3.1 Register an App ID

1. Go to [developer.apple.com/account](https://developer.apple.com/account) →
   **Certificates, Identifiers & Profiles → Identifiers**.
2. Click **+** → choose **App IDs** → **App** → **Continue**.
3. Fill in:
   - **Description**: `Channel Manager`
   - **Bundle ID** (Explicit): `com.channelmanager.app`
4. Scroll to **Capabilities** → enable **Sign In with Apple** → **Continue** → **Register**.

### 3.2 Create a Services ID

1. Back in **Identifiers** → click **+** → choose **Services IDs** → **Continue**.
2. Fill in:
   - **Description**: `Channel Manager Web`
   - **Identifier**: `com.channelmanager.web` ← this becomes your Apple `client_id`
3. Click **Continue** → **Register**.
4. Click the newly created Services ID to edit it.
5. Enable **Sign In with Apple** → click **Configure** next to it.
6. Under **Primary App ID** select the App ID created in 3.1.
7. Under **Domains and Subdomains** add:
   ```
   auth.workos.com
   ```
8. Under **Return URLs** add the WorkOS Apple callback.

   > ⚠️ **The return URL is connection-specific.** Find the exact URL in the WorkOS
   > Dashboard: **Authentication → Apple** → click the connection — the return URL
   > is displayed on that page and has the form:
   >
   > ```
   > https://auth.workos.com/sso/oauth/apple/<connection_id>/callback
   > ```
   >
   > Example (your connection ID will differ):
   >
   > ```
   > https://auth.workos.com/sso/oauth/apple/1mEz0aG50rikyCHztB0TgrYjs/callback
   > ```
   >
   > If Apple shows _"Invalid web redirect url"_, add the exact `redirect_uri=` value
   > from the error URL in your browser to the Services ID Return URLs and save.

9. Click **Next** → **Done** → **Continue** → **Save**.

### 3.3 Create a Private Key

1. Go to **Keys** → click **+**.
2. Name: `channel-manager-apple-key`.
3. Enable **Sign In with Apple** → click **Configure** → select the App ID from 3.1.
4. Click **Save** → **Continue** → **Register**.
5. **Download the `.p8` key file immediately** — Apple only lets you download it once.
6. Note the **Key ID** shown on screen.

### 3.4 Gather Apple Credentials

You need four values for WorkOS:

| WorkOS Field    | Where to find it                                        |
| --------------- | ------------------------------------------------------- |
| **Services ID** | The identifier from step 3.2 (`com.channelmanager.web`) |
| **Team ID**     | Top-right of developer.apple.com (10-char alphanumeric) |
| **Key ID**      | Shown after key creation in step 3.3                    |
| **Private Key** | Contents of the downloaded `.p8` file                   |

### 3.5 Add Apple to WorkOS

1. In the WorkOS dashboard go to **Authentication → Social Login**.
2. Find **Apple** → click **Configure**.
3. Fill in all four values from step 3.4.
4. Toggle the provider to **Enabled** → **Save**.

---

## 4 — Verify Providers Are Active in AuthKit

There is no separate AuthKit toggle — once a provider is configured and enabled
inside its own settings page (steps 2.5 and 3.5), AuthKit picks it up automatically.

**To confirm everything is wired:**

1. Go to **Authentication** in the WorkOS Dashboard.
2. Click **Google OAuth** — the toggle should show **Enabled**.
3. Click **Apple** — the toggle should show **Enabled**.
4. Visit your AuthKit hosted login URL (shown under **Configuration → Redirect URIs**)
   and verify both social buttons appear on the page.

> **Redirect URI reminder:** The callback URL registered in §1
> (`http://localhost:8080/auth/callback`) is the only redirect URI that needs
> to be configured. It lives under **Redirects** in the WorkOS Dashboard sidebar,
> not inside each provider's settings.

---

## 5 — Test the Flow

### 5.1 Start the stack

```bash
# Terminal 1 — API
make api

# Terminal 2 — Dashboard
make dev
```

### 5.2 Trigger login

Open `http://localhost:3000` — it redirects to `/login`.
Click **Sign in with Google** (or the form submit button).
You should land on the WorkOS-hosted AuthKit page showing both Google and Apple buttons.

### 5.3 Verify callback

After authenticating, WorkOS calls `GET /auth/callback` on the API.
The handler exchanges the code, sets `access_token` and `refresh_token`
HttpOnly cookies, and redirects to `/me`.

Check `/me` returns your user info:

```bash
curl -s http://localhost:8080/me --cookie "access_token=<token>"
```

---

## 6 — Production Checklist

| Step                                           | Detail                                                |
| ---------------------------------------------- | ----------------------------------------------------- |
| Publish Google consent screen                  | Requires domain verification in Google Search Console |
| Set production redirect URI in WorkOS          | `https://api.yourdomain.com/auth/callback`            |
| Set production redirect URI in Google Cloud    | Same URL as above                                     |
| Set production return URL in Apple Services ID | Same URL as above                                     |
| Rotate `WORKOS_COOKIE_PASSWORD`                | `openssl rand -base64 32`                             |
| Set `WORKOS_WEBHOOK_SECRET`                    | From WorkOS dashboard → Webhooks                      |

---

## 7 — Troubleshooting

| Error                                               | Cause                                          | Fix                                                                                                                            |
| --------------------------------------------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `redirect_uri_mismatch` (Google)                    | URI not registered in Google Cloud             | Add exact `redirect_uri=` value from error to Google Cloud → OAuth client → Authorised redirect URIs                           |
| `invalid_request: Invalid web redirect url` (Apple) | Return URL not registered in Apple Services ID | Add exact `redirect_uri=` value from error URL to Apple Developer → Services ID → Return URLs, then Save → Continue → Register |
| `invalid_client` (Apple)                            | Wrong Team/Key/Services ID                     | Double-check all four Apple values in WorkOS                                                                                   |
| WorkOS shows "provider not configured"              | Provider not enabled in AuthKit                | Toggle it on in Authentication → Social Login                                                                                  |
| Cookie not set after callback                       | `WORKOS_REDIRECT_URI` mismatch                 | Must match what's registered in WorkOS dashboard                                                                               |
| Apple button missing on AuthKit page                | Services ID not saved                          | Re-save the Services ID configuration in Apple portal                                                                          |
