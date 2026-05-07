import type { Metadata } from "next";
import LoginForm from "@/components/auth/login-form";

export const metadata: Metadata = {
  title: "Sign In — Channel Manager",
};

function SecurityIllustration() {
  return (
    <svg
      viewBox="0 0 520 400"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className="w-full max-w-lg"
      aria-hidden="true"
    >
      {/* ── Floor line ── */}
      <line x1="60" y1="330" x2="460" y2="330" stroke="#CBD5E1" strokeWidth="2" />
      <line x1="90" y1="332" x2="400" y2="332" stroke="#3B82F6" strokeWidth="3" strokeLinecap="round" />

      {/* ── Desk ── */}
      <rect x="100" y="272" width="320" height="12" rx="4" fill="#94A3B8" />
      <rect x="115" y="284" width="10" height="46" rx="3" fill="#78909C" />
      <rect x="395" y="284" width="10" height="46" rx="3" fill="#78909C" />

      {/* ── Laptop base ── */}
      <rect x="170" y="258" width="180" height="16" rx="5" fill="#64748B" />

      {/* ── Laptop screen ── */}
      <path d="M176 258 L162 178 L358 178 L344 258 Z" fill="#1E293B" />
      <rect x="168" y="182" width="184" height="72" rx="3" fill="#0F172A" />
      {/* screen glow */}
      <rect x="174" y="188" width="100" height="7" rx="3" fill="#60A5FA" opacity="0.5" />
      <rect x="174" y="201" width="72" height="7" rx="3" fill="#60A5FA" opacity="0.3" />
      <rect x="174" y="214" width="84" height="7" rx="3" fill="#60A5FA" opacity="0.3" />
      {/* laptop logo dot */}
      <circle cx="260" cy="268" r="7" fill="#475569" />

      {/* ── Chair ── */}
      <rect x="210" y="270" width="100" height="10" rx="5" fill="#475569" />
      <rect x="248" y="280" width="24" height="48" rx="5" fill="#475569" />
      <rect x="226" y="320" width="68" height="10" rx="5" fill="#475569" />

      {/* ── Person — body (orange sweater) ── */}
      <rect x="228" y="216" width="64" height="58" rx="14" fill="#F59E0B" />

      {/* ── Person — head ── */}
      <circle cx="260" cy="204" r="24" fill="#FDE68A" />
      {/* hair */}
      <path d="M236 198 Q260 176 284 198" fill="#1E293B" />
      {/* face details */}
      <circle cx="253" cy="207" r="2.5" fill="#92400E" />
      <circle cx="267" cy="207" r="2.5" fill="#92400E" />
      <path d="M252 216 Q260 221 268 216" stroke="#92400E" strokeWidth="2" strokeLinecap="round" fill="none" />

      {/* ── Person — arms ── */}
      <path d="M228 232 Q200 248 192 262" stroke="#F59E0B" strokeWidth="16" strokeLinecap="round" />
      <path d="M292 232 Q316 248 328 262" stroke="#F59E0B" strokeWidth="16" strokeLinecap="round" />
      {/* hands */}
      <ellipse cx="191" cy="264" rx="12" ry="9" fill="#FDE68A" />
      <ellipse cx="329" cy="264" rx="12" ry="9" fill="#FDE68A" />

      {/* ── Person — legs (dark blue trousers) ── */}
      <path d="M242 274 L234 330" stroke="#1E3A8A" strokeWidth="20" strokeLinecap="round" />
      <path d="M278 274 L286 330" stroke="#1E3A8A" strokeWidth="20" strokeLinecap="round" />
      {/* shoes */}
      <ellipse cx="230" cy="330" rx="18" ry="8" fill="#0F172A" />
      <ellipse cx="290" cy="330" rx="18" ry="8" fill="#0F172A" />

      {/* ── Plant (left) ── */}
      {/* pot */}
      <path d="M68 290 L52 330 L88 330 Z" fill="#78909C" />
      <rect x="56" y="284" width="28" height="10" rx="3" fill="#64748B" />
      {/* leaves */}
      <ellipse cx="70" cy="260" rx="20" ry="28" fill="#16A34A" />
      <ellipse cx="52" cy="272" rx="16" ry="22" fill="#15803D" />
      <ellipse cx="88" cy="269" rx="16" ry="22" fill="#166534" />

      {/* ── Floating: green checkmark badge ── */}
      <rect x="66" y="118" width="50" height="50" rx="10" fill="#DCFCE7" />
      <rect x="70" y="122" width="42" height="42" rx="8" fill="#22C55E" opacity="0.7" />
      <path d="M80 142 L89 151 L110 130" stroke="white" strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" />

      {/* ── Floating: padlock ── */}
      <rect x="188" y="88" width="68" height="62" rx="10" fill="#E2E8F0" />
      <rect x="196" y="108" width="52" height="42" rx="7" fill="#64748B" />
      <path d="M206 108 Q222 74 238 108" stroke="#CBD5E1" strokeWidth="9" fill="none" strokeLinecap="round" />
      <circle cx="222" cy="126" r="7" fill="#CBD5E1" />
      <rect x="219" y="129" width="6" height="10" rx="3" fill="#CBD5E1" />

      {/* ── Floating: key ── */}
      <circle cx="340" cy="118" r="18" stroke="#F59E0B" strokeWidth="5" fill="none" />
      <rect x="355" y="115" width="52" height="6" rx="3" fill="#F59E0B" />
      <rect x="396" y="120" width="6" height="12" rx="3" fill="#F59E0B" />
      <rect x="384" y="120" width="6" height="10" rx="3" fill="#F59E0B" />

      {/* ── Floating: password dots card ── */}
      <rect x="322" y="68" width="120" height="38" rx="9" fill="white" stroke="#E2E8F0" strokeWidth="1.5" />
      <circle cx="348" cy="87" r="6" fill="#475569" />
      <circle cx="368" cy="87" r="6" fill="#475569" />
      <circle cx="388" cy="87" r="6" fill="#475569" />
      <circle cx="408" cy="87" r="6" fill="#475569" />
      <circle cx="428" cy="87" r="6" fill="#475569" />

      {/* ── Decorative + / – ── */}
      <text x="145" y="170" fill="#94A3B8" fontSize="20" fontWeight="700">+</text>
      <text x="370" y="175" fill="#94A3B8" fontSize="20" fontWeight="700">+</text>
      <text x="424" y="210" fill="#94A3B8" fontSize="18" fontWeight="700">–</text>
    </svg>
  );
}

function CMLogo() {
  return (
    <div className="flex flex-col items-center gap-2 mb-6">
      {/* Circular icon with coloured arcs */}
      <svg width="56" height="56" viewBox="0 0 56 56" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="28" cy="28" r="27" stroke="#E2E8F0" strokeWidth="2" />
        {/* arcs */}
        <path d="M28 4 A24 24 0 0 1 52 28" stroke="#2563EB" strokeWidth="5" strokeLinecap="round" fill="none" />
        <path d="M52 28 A24 24 0 0 1 28 52" stroke="#22C55E" strokeWidth="5" strokeLinecap="round" fill="none" />
        <path d="M28 52 A24 24 0 0 1 4 28" stroke="#F59E0B" strokeWidth="5" strokeLinecap="round" fill="none" />
        <path d="M4 28 A24 24 0 0 1 28 4" stroke="#EF4444" strokeWidth="5" strokeLinecap="round" fill="none" />
        {/* inner white circle */}
        <circle cx="28" cy="28" r="16" fill="white" />
        <text x="28" y="33" textAnchor="middle" fontSize="11" fontWeight="700" fill="#1E293B" fontFamily="Inter, sans-serif">CM</text>
      </svg>
      <span className="text-2xl font-bold text-slate-800 tracking-tight">Channel Manager</span>
    </div>
  );
}

export default function LoginPage() {
  return (
    <div className="flex min-h-screen font-sans">
      {/* ── Left panel — illustration ── */}
      <div className="hidden lg:flex lg:w-[56%] bg-slate-100 items-center justify-center px-12 py-10 relative overflow-hidden">
        <SecurityIllustration />
      </div>

      {/* ── Right panel — form ── */}
      <div className="flex flex-1 flex-col bg-white">
        {/* language selector */}
        <div className="flex justify-end px-8 pt-6">
          <button
            type="button"
            className="flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 transition-colors"
          >
            English
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className="mt-px">
              <path d="M2 4l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        </div>

        {/* centred form area */}
        <div className="flex flex-1 flex-col items-center justify-center px-8 pb-16">
          <div className="w-full max-w-sm">
            <CMLogo />
            <h1 className="text-2xl font-bold text-center text-slate-800 mb-1">Sign In</h1>
            <p className="text-sm text-center text-slate-500 mb-8">
              Welcome back. You&apos;ve been missed!
            </p>
            <LoginForm />
          </div>
        </div>
      </div>
    </div>
  );
}
