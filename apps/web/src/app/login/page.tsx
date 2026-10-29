"use client";

import React, { useState, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Shield, KeyRound, Mail, ArrowRight, Eye, EyeOff } from "lucide-react";
import { apiRequest } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { User, TokenResponse } from "@/lib/types";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/Spinner";

function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const redirect = searchParams.get("redirect");
  const { login } = useAuth();

  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isUnverified, setIsUnverified] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsUnverified(false);
    setLoading(true);

    try {
      const trimmedIdentifier = identifier.trim();
      const data = await apiRequest<TokenResponse>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ username: trimmedIdentifier, identifier: trimmedIdentifier, password }),
      });

      login(data.user, (data as any).csrf_token);

      const hasPrivilegedRole =
        ["super_admin", "national_admin", "zonal_coordinator", "state_coordinator", "lga_representative"].includes(
          data.user.primary_role
        ) ||
        Boolean(
          data.user.appointments?.some((a) =>
            a.status === "active" &&
            ["super_admin", "national_admin", "zonal_coordinator", "state_coordinator", "lga_representative"].includes(a.role_type)
          )
        );

      if (redirect) {
        router.push(redirect);
      } else if (hasPrivilegedRole) {
        router.push("/admin");
      } else {
        router.push("/member");
      }
    } catch (err: any) {
      const status = typeof err?.status === "number" ? err.status : 0;
      const code = typeof err?.code === "string" ? err.code : "";
      const rawMsg = typeof err?.message === "string" ? err.message : "";

      // 1. Gateway & Infrastructure Errors (502 Bad Gateway, Cloudflare, Next.js rewrite proxy timeout)
      if (status === 502 || code === "BAD_GATEWAY" || rawMsg.includes("502") || rawMsg.toLowerCase().includes("bad gateway")) {
        setError(
          "Service temporarily unavailable (502 Bad Gateway). The backend server is restarting or unreachable. Please wait a moment and try again."
        );
        return;
      }

      // 2. Service Unavailable, Gateway Timeout, or Network Drops (503, 504, connection refused)
      if (
        status === 503 ||
        status === 504 ||
        code === "SERVICE_UNAVAILABLE" ||
        code === "GATEWAY_TIMEOUT" ||
        code === "NETWORK_ERROR" ||
        rawMsg.toLowerCase().includes("failed to fetch") ||
        rawMsg.toLowerCase().includes("network")
      ) {
        setError(
          rawMsg || "Authentication service is temporarily unavailable. Please wait a moment and try again."
        );
        return;
      }

      // 3. Other Internal Server Errors (500)
      if (status >= 500 || code === "SERVER_ERROR") {
        setError(
          "An unexpected server error occurred. Please try again in a few moments."
        );
        return;
      }

      // 4. Rate Limiting (429)
      if (status === 429 || code === "RATE_LIMIT_EXCEEDED") {
        setError(rawMsg || "Too many login attempts. Please wait a few minutes before trying again.");
        return;
      }

      // 5. Account Unverified (403)
      if (
        (status === 403 && code === "USER_UNVERIFIED") ||
        rawMsg.toLowerCase().includes("pending") ||
        rawMsg.toLowerCase().includes("verification") ||
        rawMsg.toLowerCase().includes("unverified")
      ) {
        setError(rawMsg || "Your account email is unverified. Please check your inbox or request a new verification link.");
        setIsUnverified(true);
        return;
      }

      // 6. Account Inactive or Suspended (403)
      if (status === 403 && (code === "ACCOUNT_INACTIVE" || rawMsg.toLowerCase().includes("inactive"))) {
        setError("Your account is currently inactive. Please contact support or your coordinator.");
        return;
      }

      // 7. Genuine Authentication Failure (Strictly HTTP 401 or INVALID_CREDENTIALS)
      if (status === 401 || code === "INVALID_CREDENTIALS") {
        setError("Invalid email or password. Please check your credentials and try again.");
        return;
      }

      // 8. Safe Fallback - NEVER display "Invalid email or password" unless status was 401
      setError(rawMsg || "Unable to complete login. Please check your connection and try again.");
    } finally {
      setLoading(false);
    }
  };

  const handleQuickFill = (emailVal: string, passVal: string) => {
    setIdentifier(emailVal);
    setPassword(passVal);
    setError(null);
  };

  return (
    <div className="max-w-md mx-auto px-4 py-16 space-y-6">
      <div className="text-center space-y-2">
        <div className="w-12 h-12 rounded-xl bg-emerald-900 text-white flex items-center justify-center mx-auto shadow-sm">
          <Shield className="w-6 h-6 text-emerald-400" />
        </div>
        <h1 className="text-2xl font-bold text-gray-900">Sign in to Ratels Worldwide</h1>
        <p className="text-xs text-gray-600">Access your member portal or administrative workspace</p>
      </div>

      {error && <Alert type="error">{error}</Alert>}

      {isUnverified && (
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-4 text-xs sm:text-sm text-amber-950 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 shadow-sm">
          <span>Need to activate your account or get a new link?</span>
          <Link
            href={`/check-email?email=${encodeURIComponent(identifier)}`}
            className="font-bold underline text-amber-900 hover:text-amber-950 inline-flex items-center gap-1 shrink-0"
          >
            Check Email / Resend &rarr;
          </Link>
        </div>
      )}

      <div className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-6">
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-gray-700 mb-1">
              Email, Phone, or Membership Reference
            </label>
            <div className="relative">
              <Mail className="w-4 h-4 text-gray-400 absolute left-3 top-3.5" />
              <input
                type="text"
                required
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
                placeholder="e.g. member1@ratels.org.ng or RATEL-2026-..."
                className="w-full pl-9 pr-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-700 mb-1">Password</label>
            <div className="relative">
              <KeyRound className="w-4 h-4 text-gray-400 absolute left-3 top-3.5" />
              <input
                type={showPassword ? "text" : "password"}
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full pl-9 pr-12 py-2.5 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
              <button
                type="button"
                onClick={() => setShowPassword((current) => !current)}
                aria-label={showPassword ? "Hide password" : "Show password"}
                aria-pressed={showPassword}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-emerald-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-700 rounded"
              >
                {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
              </button>
            </div>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full py-3 px-4 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition disabled:opacity-50"
          >
            {loading ? "Signing in..." : "Sign In"}
          </button>
        </form>

        {/* Development Quick-Fill Credentials */}
        <div className="pt-4 border-t border-gray-100 space-y-2.5">
          <div className="text-[11px] font-semibold text-gray-500 uppercase tracking-wider text-center">
            Development Quick Sign-In
          </div>
          <div className="grid grid-cols-1 gap-1.5">
            <button
              type="button"
              onClick={() => handleQuickFill("admin@ratels.example.org", "Admin1234!")}
              className="text-left px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-xs text-gray-700 flex justify-between items-center transition"
            >
              <span><strong>National Admin:</strong> admin@ratels.example.org</span>
              <ArrowRight className="w-3.5 h-3.5 text-gray-400" />
            </button>
            <button
              type="button"
              onClick={() => handleQuickFill("lagos_coord@ratels.example.org", "Coord1234!")}
              className="text-left px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-xs text-gray-700 flex justify-between items-center transition"
            >
              <span><strong>Lagos Coordinator:</strong> lagos_coord@ratels.example.org</span>
              <ArrowRight className="w-3.5 h-3.5 text-gray-400" />
            </button>
            <button
              type="button"
              onClick={() => handleQuickFill("member1@ratels.example.org", "Member1234!")}
              className="text-left px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-xs text-gray-700 flex justify-between items-center transition"
            >
              <span><strong>Sample Member:</strong> member1@ratels.example.org</span>
              <ArrowRight className="w-3.5 h-3.5 text-gray-400" />
            </button>
          </div>
        </div>

        <div className="text-center text-xs text-gray-600">
          Not yet a member?{" "}
          <Link href="/register" className="text-emerald-800 font-semibold hover:underline">
            Register an account
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<Spinner text="Loading sign-in interface..." />}>
      <LoginForm />
    </Suspense>
  );
}
