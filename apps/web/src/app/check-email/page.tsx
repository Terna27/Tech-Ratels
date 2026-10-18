"use client";

import React, { useState, useEffect, Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Mail, ArrowRight, RefreshCw, CheckCircle2, AlertCircle, ArrowLeft, Shield } from "lucide-react";
import { apiRequest } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/Spinner";

function CheckEmailContent() {
  const searchParams = useSearchParams();
  const emailParam = searchParams.get("email") || "";
  const initialStatus = searchParams.get("status") || "sent";

  const [email, setEmail] = useState(emailParam);
  const [resending, setResending] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(
    initialStatus === "failed"
      ? "We encountered a delivery provider delay sending your initial email. Please click Resend Verification Email below to try again."
      : null
  );

  useEffect(() => {
    if (emailParam) {
      setEmail(emailParam);
    }
  }, [emailParam]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => {
      setCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  const handleResend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || cooldown > 0 || resending) return;

    setErrorMsg(null);
    setSuccessMsg(null);
    setResending(true);

    try {
      const data = await apiRequest<{ message: string; sent: boolean; already_verified?: boolean }>(
        "/auth/resend-verification",
        {
          method: "POST",
          body: JSON.stringify({ email: email.trim() }),
        }
      );

      if (data.already_verified) {
        setSuccessMsg("This account is already verified. You can proceed to log in.");
      } else {
        setSuccessMsg(data.message || "A fresh verification link has been dispatched to your email address.");
        setCooldown(60); // 60s cooldown
      }
    } catch (err: any) {
      const message = err.message || "Failed to resend verification email.";
      setErrorMsg(message);
      // If cooldown was active, extract cooldown duration or default to 30
      if (message.includes("wait")) {
        setCooldown(30);
      }
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="min-h-[80vh] flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8 bg-white p-8 sm:p-10 rounded-2xl border border-gray-200 shadow-sm">
        {/* Header Icon */}
        <div className="text-center space-y-3">
          <div className="w-16 h-16 bg-emerald-50 text-emerald-800 rounded-full flex items-center justify-center mx-auto shadow-inner border border-emerald-100">
            <Mail className="w-8 h-8 text-emerald-700" />
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
            Check Your Email
          </h1>
          <p className="text-base sm:text-lg text-gray-600 leading-relaxed font-normal">
            We sent an activation link to{" "}
            {email ? (
              <span className="font-bold text-gray-900 break-all">{email}</span>
            ) : (
              "your registered email address"
            )}
            .
          </p>
        </div>

        {/* Status Alerts */}
        {errorMsg && (
          <Alert type="error">
            <span className="text-sm font-medium">{errorMsg}</span>
          </Alert>
        )}

        {successMsg && (
          <Alert type="success">
            <span className="text-sm font-medium">{successMsg}</span>
          </Alert>
        )}

        {/* Instructions Card */}
        <div className="bg-emerald-50/60 rounded-xl p-4 border border-emerald-100 space-y-2 text-xs sm:text-sm text-emerald-950">
          <p className="font-semibold flex items-center gap-1.5 text-emerald-900">
            <Shield className="w-4 h-4 text-emerald-700 shrink-0" />
            Verification Security Steps:
          </p>
          <ul className="list-disc list-inside space-y-1 text-emerald-800/90 leading-relaxed">
            <li>Open the email from Martins Vincent Otse Foundation.</li>
            <li>Click the verification link within 24 hours.</li>
            <li>Press the confirmation button to activate your account.</li>
          </ul>
        </div>

        {/* Resend Form */}
        <form onSubmit={handleResend} className="space-y-4 pt-2">
          {!emailParam && (
            <div>
              <label htmlFor="email" className="block text-xs font-semibold uppercase text-gray-600 mb-1">
                Registered Email Address
              </label>
              <input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="volunteer@example.org"
                required
                className="w-full px-4 py-2.5 text-sm rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:border-transparent transition"
              />
            </div>
          )}

          <button
            type="submit"
            disabled={resending || cooldown > 0 || !email}
            className="w-full flex items-center justify-center gap-2 py-3 px-4 border border-gray-300 rounded-xl text-sm font-bold text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-emerald-600 disabled:opacity-50 disabled:cursor-not-allowed transition shadow-sm"
          >
            {resending ? (
              <>
                <Spinner size="sm" />
                <span>Sending Verification Email...</span>
              </>
            ) : cooldown > 0 ? (
              <>
                <RefreshCw className="w-4 h-4 animate-spin text-gray-400" />
                <span>Resend available in {cooldown}s</span>
              </>
            ) : (
              <>
                <RefreshCw className="w-4 h-4 text-emerald-700" />
                <span>Resend Verification Email</span>
              </>
            )}
          </button>
        </form>

        {/* Action Links */}
        <div className="pt-4 border-t border-gray-100 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs sm:text-sm">
          <Link
            href="/login"
            className="font-bold text-emerald-800 hover:text-emerald-950 inline-flex items-center gap-1 transition"
          >
            <ArrowLeft className="w-4 h-4" />
            Proceed to Login
          </Link>
          <Link
            href="/register"
            className="font-medium text-gray-500 hover:text-gray-700 transition"
          >
            Wrong email address?
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function CheckEmailPage() {
  return (
    <Suspense fallback={<Spinner text="Loading..." />}>
      <CheckEmailContent />
    </Suspense>
  );
}
