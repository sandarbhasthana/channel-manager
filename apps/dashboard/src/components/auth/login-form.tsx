"use client";

import { useState, useCallback } from "react";
import { Mail, Lock, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export default function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [trust, setTrust] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
    <form onSubmit={handleSignIn} className="flex flex-col gap-4">
      {/* ── OAuth provider buttons ── */}
      <div className="flex flex-col gap-2">
        {/* Google */}
        <a
          href={`${API_URL}/auth/login?provider=GoogleOAuth`}
          className={cn(
            "flex items-center justify-center gap-3 w-full",
            "rounded-lg border border-slate-200 bg-white px-4 py-2.5",
            "text-sm font-medium text-slate-700",
            "hover:bg-slate-50 transition-colors"
          )}
        >
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
            <path d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844c-.209 1.125-.843 2.078-1.796 2.716v2.259h2.908c1.702-1.567 2.684-3.875 2.684-6.615z" fill="#4285F4" />
            <path d="M9 18c2.43 0 4.467-.806 5.956-2.184l-2.908-2.259c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332A8.997 8.997 0 0 0 9 18z" fill="#34A853" />
            <path d="M3.964 10.706A5.41 5.41 0 0 1 3.682 9c0-.593.102-1.17.282-1.706V4.962H.957A8.996 8.996 0 0 0 0 9c0 1.452.348 2.827.957 4.038l3.007-2.332z" fill="#FBBC05" />
            <path d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0A8.997 8.997 0 0 0 .957 4.962L3.964 7.294C4.672 5.163 6.656 3.58 9 3.58z" fill="#EA4335" />
          </svg>
          Sign in with Google
        </a>

        {/* Apple */}
        <a
          href={`${API_URL}/auth/login?provider=AppleOAuth`}
          className={cn(
            "flex items-center justify-center gap-3 w-full",
            "rounded-lg border border-slate-200 bg-white px-4 py-2.5",
            "text-sm font-medium text-slate-700",
            "hover:bg-slate-50 transition-colors"
          )}
        >
          <svg width="16" height="18" viewBox="0 0 814 1000" aria-hidden="true" fill="currentColor">
            <path d="M788.1 340.9c-5.8 4.5-108.2 62.2-108.2 190.5 0 148.4 130.3 200.9 134.2 202.2-.6 3.2-20.7 71.9-68.7 141.9-42.8 61.6-87.5 123.1-155.5 123.1s-85.5-39.5-164-39.5c-76 0-103.7 40.8-165.9 40.8s-105-57.8-155.5-127.4C46 411.5 0 282.4 0 159.8 0 71.3 37.8 27.8 75.5 27.8c58.3 0 97.3 54.8 135.3 54.8 26.1 0 75.5-54.8 142.4-54.8 21.9 0 135.3 2.7 198.1 127.4zm-234-181.5c31.1-36.9 53.1-88.1 53.1-139.3 0-7.1-.6-14.3-1.9-20.1-50.6 1.9-110.8 33.7-147.1 75.8-28.5 32.4-55.1 83.6-55.1 135.5 0 7.8 1.3 15.6 1.9 18.1 3.2.6 8.4 1.3 13.6 1.3 45.4 0 102.5-30.4 135.5-71.3z" />
          </svg>
          Sign in with Apple
        </a>
      </div>

      {/* ── Divider ── */}
      <div className="flex items-center gap-3">
        <hr className="flex-1 border-slate-200" />
        <span className="text-xs text-slate-400 select-none">or</span>
        <hr className="flex-1 border-slate-200" />
      </div>

      {/* ── Error banner ── */}
      {error && (
        <p className="rounded-lg bg-red-50 border border-red-200 px-3 py-2 text-xs text-red-600">
          {error}
        </p>
      )}

      {/* ── Email ── */}
      <div className="relative">
        <Mail size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
        <input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          disabled={loading}
          className={cn(
            "w-full rounded-lg border border-slate-200 bg-white pl-9 pr-4 py-2.5",
            "text-sm text-slate-800 placeholder:text-slate-400",
            "outline-none focus:border-brand focus:ring-2 focus:ring-brand/20 transition",
            "disabled:opacity-60 disabled:cursor-not-allowed"
          )}
        />
      </div>

      {/* ── Password ── */}
      <div className="relative">
        <Lock size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          disabled={loading}
          className={cn(
            "w-full rounded-lg border border-slate-200 bg-white pl-9 pr-4 py-2.5",
            "text-sm text-slate-800 placeholder:text-slate-400",
            "outline-none focus:border-brand focus:ring-2 focus:ring-brand/20 transition",
            "disabled:opacity-60 disabled:cursor-not-allowed"
          )}
        />
      </div>

      {/* ── Trust device + forgot password row ── */}
      <div className="flex items-center justify-between">
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={trust}
            onChange={(e) => setTrust(e.target.checked)}
            disabled={loading}
            className="w-4 h-4 rounded border-slate-300 accent-blue-600 cursor-pointer"
          />
          <span className="text-xs text-slate-600">Trust this device</span>
        </label>
        <a href="#" className="text-xs text-brand hover:underline transition-colors">
          Forgot password?
        </a>
      </div>

      {/* ── Sign In button ── */}
      <button
        type="submit"
        disabled={loading}
        className={cn(
          "flex items-center justify-center gap-2 w-full",
          "rounded-lg bg-brand px-4 py-2.5",
          "text-sm font-semibold text-white",
          "hover:bg-brand-hover active:scale-[0.98] transition-all",
          "disabled:opacity-70 disabled:cursor-not-allowed disabled:active:scale-100"
        )}
      >
        {loading ? <Loader2 size={15} className="animate-spin" /> : null}
        {loading ? "Signing in…" : "Sign In"}
      </button>
    </form>
  );
}
