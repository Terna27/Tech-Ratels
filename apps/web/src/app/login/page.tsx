"use client";

import React, { useState, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Shield, KeyRound, Mail, ArrowRight } from "lucide-react";
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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const data = await apiRequest<TokenResponse>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ identifier, password }),
      });

      login(data.access_token, data.user);

      if (redirect) {
        router.push(redirect);
      } else if (
        ["national_admin", "zonal_coordinator", "state_coordinator", "lga_representative"].includes(
          data.user.primary_role
        )
      ) {
        router.push("/admin");
      } else {
        router.push("/member");
      }
    } catch (err: any) {
      setError(err.message || "Invalid login credentials. Please check and try again.");
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
        <h1 className="text-2xl font-bold text-gray-900">Sign in to Nigerian Ratels</h1>
        <p className="text-xs text-gray-600">Access your member portal or administrative workspace</p>
      </div>

      {error && <Alert type="error">{error}</Alert>}

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
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full pl-9 pr-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
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
