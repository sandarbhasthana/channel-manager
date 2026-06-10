"use client";

import { useState, useCallback } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export default function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [trust, setTrust] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);

  const handleSignIn = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setError(null);
      setLoading(true);
      try {
        const res = await fetch(`${API_URL}/auth/password`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include",
          body: JSON.stringify({ email, password }),
        });
        if (!res.ok) {
          const body = await res.json().catch(() => ({}));
          setError((body as { error?: string }).error ?? "Login failed. Please try again.");
          return;
        }
        // Cookies are set by the API — navigate to the dashboard root.
        window.location.href = "/";
      } catch {
        setError("Network error. Please check your connection and try again.");
      } finally {
        setLoading(false);
      }
    },
    [email, password]
  );

  return (
    <form onSubmit={handleSignIn} className="login-form" noValidate>
      <div className="login-welcome-eyebrow">Welcome back</div>
      <h1>Sign in to your hub</h1>
      <p className="login-lede">Pick up right where you left off.</p>

      {/* ── OAuth provider buttons ── */}
      <div className="login-social">
        {/* Google */}
        <a href={`${API_URL}/auth/login?provider=GoogleOAuth`} className="login-sbtn">
          <svg width="18" height="18" xmlns="http://www.w3.org/2000/svg" xmlnsXlink="http://www.w3.org/1999/xlink" xmlSpace="preserve" overflow="hidden" viewBox="0 0 268.152 273.883" aria-hidden="true">
            <defs>
              <linearGradient id="a"><stop offset="0" stopColor="#0fbc5c" /><stop offset="1" stopColor="#0cba65" /></linearGradient>
              <linearGradient id="g"><stop offset=".231" stopColor="#0fbc5f" /><stop offset=".312" stopColor="#0fbc5f" /><stop offset=".366" stopColor="#0fbc5e" /><stop offset=".458" stopColor="#0fbc5d" /><stop offset=".54" stopColor="#12bc58" /><stop offset=".699" stopColor="#28bf3c" /><stop offset=".771" stopColor="#38c02b" /><stop offset=".861" stopColor="#52c218" /><stop offset=".915" stopColor="#67c30f" /><stop offset="1" stopColor="#86c504" /></linearGradient>
              <linearGradient id="h"><stop offset=".142" stopColor="#1abd4d" /><stop offset=".248" stopColor="#6ec30d" /><stop offset=".312" stopColor="#8ac502" /><stop offset=".366" stopColor="#a2c600" /><stop offset=".446" stopColor="#c8c903" /><stop offset=".54" stopColor="#ebcb03" /><stop offset=".616" stopColor="#f7cd07" /><stop offset=".699" stopColor="#fdcd04" /><stop offset=".771" stopColor="#fdce05" /><stop offset=".861" stopColor="#ffce0a" /></linearGradient>
              <linearGradient id="f"><stop offset=".316" stopColor="#ff4c3c" /><stop offset=".604" stopColor="#ff692c" /><stop offset=".727" stopColor="#ff7825" /><stop offset=".885" stopColor="#ff8d1b" /><stop offset="1" stopColor="#ff9f13" /></linearGradient>
              <linearGradient id="b"><stop offset=".231" stopColor="#ff4541" /><stop offset=".312" stopColor="#ff4540" /><stop offset=".458" stopColor="#ff4640" /><stop offset=".54" stopColor="#ff473f" /><stop offset=".699" stopColor="#ff5138" /><stop offset=".771" stopColor="#ff5b33" /><stop offset=".861" stopColor="#ff6c29" /><stop offset="1" stopColor="#ff8c18" /></linearGradient>
              <linearGradient id="d"><stop offset=".408" stopColor="#fb4e5a" /><stop offset="1" stopColor="#ff4540" /></linearGradient>
              <linearGradient id="c"><stop offset=".132" stopColor="#0cba65" /><stop offset=".21" stopColor="#0bb86d" /><stop offset=".297" stopColor="#09b479" /><stop offset=".396" stopColor="#08ad93" /><stop offset=".477" stopColor="#0aa6a9" /><stop offset=".568" stopColor="#0d9cc6" /><stop offset=".667" stopColor="#1893dd" /><stop offset=".769" stopColor="#258bf1" /><stop offset=".859" stopColor="#3086ff" /></linearGradient>
              <linearGradient id="e"><stop offset=".366" stopColor="#ff4e3a" /><stop offset=".458" stopColor="#ff8a1b" /><stop offset=".54" stopColor="#ffa312" /><stop offset=".616" stopColor="#ffb60c" /><stop offset=".771" stopColor="#ffcd0a" /><stop offset=".861" stopColor="#fecf0a" /><stop offset=".915" stopColor="#fecf08" /><stop offset="1" stopColor="#fdcd01" /></linearGradient>
              <linearGradient xlinkHref="#a" id="s" x1="219.7" x2="254.467" y1="329.535" y2="329.535" gradientUnits="userSpaceOnUse" />
              <radialGradient xlinkHref="#b" id="m" cx="109.627" cy="135.862" r="71.46" fx="109.627" fy="135.862" gradientTransform="matrix(-1.93688 1.043 1.45573 2.55542 290.525 -400.634)" gradientUnits="userSpaceOnUse" />
              <radialGradient xlinkHref="#c" id="n" cx="45.259" cy="279.274" r="71.46" fx="45.259" fy="279.274" gradientTransform="matrix(-3.5126 -4.45809 -1.69255 1.26062 870.8 191.554)" gradientUnits="userSpaceOnUse" />
              <radialGradient xlinkHref="#d" id="l" cx="304.017" cy="118.009" r="47.854" fx="304.017" fy="118.009" gradientTransform="matrix(2.06435 0 0 2.59204 -297.679 -151.747)" gradientUnits="userSpaceOnUse" />
              <radialGradient xlinkHref="#e" id="o" cx="181.001" cy="177.201" r="71.46" fx="181.001" fy="177.201" gradientTransform="matrix(-.24858 2.08314 2.96249 .33417 -255.146 -331.164)" gradientUnits="userSpaceOnUse" />
              <radialGradient xlinkHref="#f" id="p" cx="207.673" cy="108.097" r="41.102" fx="207.673" fy="108.097" gradientTransform="matrix(-1.2492 1.34326 -3.89684 -3.4257 880.501 194.905)" gradientUnits="userSpaceOnUse" />
              <radialGradient xlinkHref="#g" id="r" cx="109.627" cy="135.862" r="71.46" fx="109.627" fy="135.862" gradientTransform="matrix(-1.93688 -1.043 1.45573 -2.55542 290.525 838.683)" gradientUnits="userSpaceOnUse" />
              <radialGradient xlinkHref="#h" id="j" cx="154.87" cy="145.969" r="71.46" fx="154.87" fy="145.969" gradientTransform="matrix(-.0814 -1.93722 2.92674 -.11625 -215.135 632.86)" gradientUnits="userSpaceOnUse" />
              <filter id="q" width="1.097" height="1.116" x="-.048" y="-.058" colorInterpolationFilters="sRGB"><feGaussianBlur stdDeviation="1.701" /></filter>
              <filter id="k" width="1.033" height="1.02" x="-.017" y="-.01" colorInterpolationFilters="sRGB"><feGaussianBlur stdDeviation=".242" /></filter>
              <clipPath id="i" clipPathUnits="userSpaceOnUse"><path d="M371.378 193.24H237.083v53.438h77.167c-1.241 7.563-4.026 15.003-8.105 21.786-4.674 7.773-10.451 13.69-16.373 18.196-17.74 13.498-38.42 16.258-52.783 16.258-36.283 0-67.283-23.286-79.285-54.928-.484-1.149-.805-2.335-1.197-3.507a81.115 81.115 0 0 1-4.101-25.448c0-9.226 1.569-18.057 4.43-26.398 11.285-32.897 42.985-57.467 80.179-57.467 7.481 0 14.685.884 21.517 2.648a77.668 77.668 0 0 1 33.425 18.25l40.834-39.712c-24.839-22.616-57.219-36.32-95.844-36.32-30.878 0-59.386 9.553-82.748 25.7-18.945 13.093-34.483 30.625-44.97 50.985-9.753 18.879-15.094 39.8-15.094 62.294 0 22.495 5.35 43.633 15.103 62.337v.126c10.302 19.857 25.368 36.954 43.678 49.988 15.997 11.386 44.68 26.551 84.031 26.551 22.63 0 42.687-4.051 60.375-11.644 12.76-5.478 24.065-12.622 34.301-21.804 13.525-12.132 24.117-27.139 31.347-44.404 7.23-17.265 11.097-36.79 11.097-57.957 0-9.858-.998-19.87-2.689-28.968Z" /></clipPath>
            </defs>
            <g clipPath="url(#i)" transform="matrix(.95792 0 0 .98525 -90.174 -78.856)">
              <path fill="url(#j)" d="M92.076 219.958c.148 22.14 6.501 44.983 16.117 63.424v.127c6.949 13.392 16.445 23.97 27.26 34.452l65.327-23.67c-12.36-6.235-14.246-10.055-23.105-17.026-9.054-9.066-15.802-19.473-20.004-31.677h-.17l.17-.127c-2.765-8.058-3.037-16.613-3.14-25.503Z" filter="url(#k)" />
              <path fill="url(#l)" d="M237.083 79.025c-6.456 22.526-3.988 44.421 0 57.161 7.457.006 14.64.888 21.45 2.647a77.662 77.662 0 0 1 33.424 18.25l41.88-40.726c-24.81-22.59-54.667-37.297-96.754-37.332Z" filter="url(#k)" />
              <path fill="url(#m)" d="M236.943 78.847c-31.67 0-60.91 9.798-84.871 26.359a145.533 145.533 0 0 0-24.332 21.15c-1.904 17.744 14.257 39.551 46.262 39.37 15.528-17.936 38.495-29.542 64.056-29.542l.07.002-1.044-57.335c-.048 0-.093-.004-.14-.004Z" filter="url(#k)" />
              <path fill="url(#n)" d="m341.475 226.379-28.268 19.285c-1.24 7.562-4.028 15.002-8.107 21.786-4.674 7.772-10.45 13.69-16.373 18.196-17.702 13.47-38.328 16.244-52.687 16.255-14.842 25.102-17.444 37.675 1.043 57.934 22.877-.016 43.157-4.117 61.046-11.796 12.931-5.551 24.388-12.792 34.761-22.097 13.706-12.295 24.442-27.503 31.769-45 7.327-17.497 11.245-37.282 11.245-58.734Z" filter="url(#k)" />
              <path fill="#3086ff" d="M234.996 191.21v57.498h136.006c1.196-7.874 5.152-18.064 5.152-26.5 0-9.858-.996-21.899-2.687-30.998Z" filter="url(#k)" />
              <path fill="url(#o)" d="M128.39 124.327c-8.394 9.119-15.564 19.326-21.249 30.364-9.753 18.879-15.094 41.83-15.094 64.324 0 .317.026.627.029.944 4.32 8.224 59.666 6.649 62.456 0-.004-.31-.039-.613-.039-.924 0-9.226 1.57-16.026 4.43-24.367 3.53-10.289 9.056-19.763 16.123-27.926 1.602-2.031 5.875-6.397 7.121-9.016.475-.997-.862-1.557-.937-1.908-.083-.393-1.876-.077-2.277-.37-1.275-.929-3.8-1.414-5.334-1.845-3.277-.921-8.708-2.953-11.725-5.06-9.536-6.658-24.417-14.612-33.505-24.216Z" filter="url(#k)" />
              <path fill="url(#p)" d="M162.099 155.857c22.112 13.301 28.471-6.714 43.173-12.977l-25.574-52.664a144.74 144.74 0 0 0-26.543 14.504c-12.316 8.512-23.192 18.9-32.176 30.72Z" filter="url(#q)" />
              <path fill="url(#r)" d="M171.099 290.222c-29.683 10.641-34.33 11.023-37.062 29.29a144.806 144.806 0 0 0 16.792 13.984c15.996 11.386 46.766 26.551 86.118 26.551.046 0 .09-.004.137-.004v-59.157l-.094.002c-14.736 0-26.512-3.843-38.585-10.527-2.977-1.648-8.378 2.777-11.123.799-3.786-2.729-12.9 2.35-16.183-.938Z" filter="url(#k)" />
              <path fill="url(#s)" d="M219.7 299.023v59.996c5.506.64 11.236 1.028 17.247 1.028 6.026 0 11.855-.307 17.52-.872v-59.748a105.119 105.119 0 0 1-17.477 1.461c-5.932 0-11.7-.686-17.29-1.865Z" filter="url(#k)" opacity=".5" />
            </g>
          </svg>
          Google
        </a>
        
        {/* Apple */}
        <a href={`${API_URL}/auth/login?provider=AppleOAuth`} className="login-sbtn">
          <svg xmlns="http://www.w3.org/2000/svg" xmlSpace="preserve" width="19" height="19" viewBox="0 0 814 1000" aria-hidden="true" fill="currentColor">
            <path d="M788.1 340.9c-5.8 4.5-108.2 62.2-108.2 190.5 0 148.4 130.3 200.9 134.2 202.2-.6 3.2-20.7 71.9-68.7 141.9-42.8 61.6-87.5 123.1-155.5 123.1s-85.5-39.5-164-39.5c-76.5 0-103.7 40.8-165.9 40.8s-105.6-57-155.5-127C46.7 790.7 0 663 0 541.8c0-194.4 126.4-297.5 250.8-297.5 66.1 0 121.2 43.4 162.7 43.4 39.5 0 101.1-46 176.3-46 28.5 0 130.9 2.6 198.3 99.2zm-234-181.5c31.1-36.9 53.1-88.1 53.1-139.3 0-7.1-.6-14.3-1.9-20.1-50.6 1.9-110.8 33.7-147.1 75.8-28.5 32.4-55.1 83.6-55.1 135.5 0 7.8 1.3 15.6 1.9 18.1 3.2.6 8.4 1.3 13.6 1.3 45.4 0 102.5-30.4 135.5-71.3z" />
          </svg>
          Apple
        </a>
      </div>

      <div className="login-or">or sign in with email</div>

      {error && (
        <p className="rounded-lg bg-red-50 border border-red-200 px-3 py-2 text-xs text-red-600 mb-4">
          {error}
        </p>
      )}

      {/* Email */}
      <div className="login-field">
        <label htmlFor="email">Email address</label>
        <div className="login-inputwrap">
          <svg className="login-lead" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="4" width="20" height="16" rx="3"/><path d="m3 6 9 7 9-7"/></svg>
          <input 
            id="email" 
            type="email" 
            placeholder="you@property.com" 
            autoComplete="email" 
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            disabled={loading}
          />
        </div>
      </div>

      {/* Password */}
      <div className="login-field">
        <label htmlFor="password">Password</label>
        <div className="login-inputwrap">
          <svg className="login-lead" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>
          <input 
            id="password" 
            type={showPassword ? "text" : "password"}
            placeholder="Enter your password" 
            autoComplete="current-password" 
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={loading}
          />
          <button 
            type="button" 
            className="login-pw-toggle" 
            aria-label="Show password"
            onClick={() => setShowPassword(!showPassword)}
          >
            {showPassword ? (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9.9 4.24A9.1 9.1 0 0 1 12 4c6.5 0 10 7 10 7a18 18 0 0 1-2.16 3.19m-3.6 2.34A9.1 9.1 0 0 1 12 18c-6.5 0-10-7-10-7a18 18 0 0 1 4.06-4.94"/><path d="m1 1 22 22"/></svg>
            ) : (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg>
            )}
          </button>
        </div>
      </div>

      <div className="login-row-between">
        <label className="login-check">
          <input 
            type="checkbox" 
            checked={trust}
            onChange={(e) => setTrust(e.target.checked)}
            disabled={loading}
          />
          <span className="login-box"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3"><path d="M5 13l4 4L19 7"/></svg></span>
          Trust this device
        </label>
        <a href="#" className="login-link">Forgot password?</a>
      </div>

      <button type="submit" className="login-submit" disabled={loading}>
        {loading ? "Signing in…" : "Sign in"}
        {!loading && <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M5 12h14M13 6l6 6-6 6"/></svg>}
      </button>

      <div className="login-signup">New to Channel Manager? <a href="#" className="login-link">Create an account</a></div>
    </form>
  );
}
