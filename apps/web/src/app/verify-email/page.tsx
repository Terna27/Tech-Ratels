"use client";

import React, { useState, useEffect, Suspense } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import {
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Mail,
  ShieldCheck,
  ArrowRight,
  RefreshCw,
  Lock,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/Spinner";

type VerificationStatus =
  | "ready" // Waiting for explicit human button press
  | "verifying" // Executing POST request
  | "success" // Successfully verified
  | "already_verified" // User already active
  | "expired" // Token expired (24h)
  | "invalid" // Invalid or consumed token
  | "no_token"; // No token query param

function VerifyEmailContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [status, setStatus] = useState<VerificationStatus>(token ? "ready" : "no_token");
  const [message, setMessage] = useState<string>("");
  const [resendEmail, setResendEmail] = useState("");
  const [resending, setResending] = useState(false);
  const [resendMsg, setResendMsg] = useState<string | null>(null);
  const [resendErr, setResendErr] = useState<string | null>(null);

  // Require an explicit button press so automated email prefetch crawlers / security scanners do NOT consume the single-use token on GET!
  const handleConfirmVerification = async () => {
    if (!token || status === "verifying") return;

    setStatus("verifying");
    setMessage("");

    try {
      const data = await apiRequest<{
        message: string;
        is_verified: boolean;
        already_verified: boolean;
      }>("/auth/verify-email", {
        method: "POST",
        body: JSON.stringify({ token: token.trim() }),
      });

      // Clear token from browser URL to prevent token leakage in history or referrer
      if (typeof window !== "undefined" && window.history && window.history.replaceState) {
        window.history.replaceState(null, "", window.location.pathname);
      }

      if (data.already_verified) {
        setStatus("already_verified");
        setMessage(data.message || "Your email address is already verified. You may proceed to login.");
      } else {
        setStatus("success");
        setMessage(data.message || "Your email address has been verified successfully. Your account is active.");
      }
    } catch (err: any) {
      const errMsg = err.message || "";
      if (errMsg.toLowerCase().includes("expired")) {
        setStatus("expired");
        setMessage("This verification link has expired (24-hour lifetime). Please request a new link below.");
      } else if (errMsg.toLowerCase().includes("already been used")) {
        setStatus("invalid");
        setMessage("This verification link has already been used. If your account is verified, you can log in directly.");
      } else {
        setStatus("invalid");
        setMessage(errMsg || "This verification link is invalid or malformed.");
      }
    }
  };

  const handleResend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resendEmail || resending) return;

    setResendErr(null);
    setResendMsg(null);
    setResending(true);

    try {
      const data = await apiRequest<{ message: string; sent: boolean; already_verified?: boolean }>(
        "/auth/resend-verification",
        {
          method: "POST",
          body: JSON.stringify({ email: resendEmail.trim() }),
        }
      );

      if (data.already_verified) {
        setResendMsg("This email is already verified. You can log in directly.");
      } else {
        setResendMsg(data.message || "A new verification link has been sent to your email.");
      }
    } catch (err: any) {
      setResendErr(err.message || "Failed to resend verification email.");
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="min-h-[85vh] flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
      {/* Strict referrer-policy to protect verification tokens */}
      <head>
        <meta name="referrer" content="no-referrer" />
      </head>

      <div className="max-w-lg w-full bg-white p-8 sm:p-10 rounded-2xl border border-gray-200 shadow-sm space-y-6">
        {/* STATE: READY FOR EXPLICIT CONFIRMATION */}
        {status === "ready" && (
          <div className="text-center space-y-6">
            <div className="w-16 h-16 bg-emerald-50 text-emerald-800 rounded-full flex items-center justify-center mx-auto border border-emerald-100">
              <ShieldCheck className="w-9 h-9 text-emerald-700" />
            </div>
            <div className="space-y-2">
              <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
                Confirm Email Verification
              </h1>
              <p className="text-base sm:text-lg text-gray-600 leading-relaxed font-normal">
                Click the confirmation button below to verify your email address and activate your Ratels Worldwide membership.
              </p>
            </div>

            <div className="bg-gray-50 rounded-xl p-4 text-xs sm:text-sm text-gray-600 text-left space-y-1.5 border border-gray-100">
              <p className="font-semibold text-gray-900 flex items-center gap-1.5">
                <Lock className="w-4 h-4 text-emerald-700" />
                Anti-Scanner Single-Use Protection
              </p>
              <p className="leading-relaxed">
                We require an explicit click so automated email security scanners and link prefetch crawlers cannot consume your single-use activation link.
              </p>
            </div>

            <button
              onClick={handleConfirmVerification}
              className="w-full py-3.5 px-6 rounded-xl text-base font-bold text-white bg-emerald-800 hover:bg-emerald-900 focus:outline-none focus:ring-4 focus:ring-emerald-200 shadow-sm transition flex items-center justify-center gap-2"
            >
              <ShieldCheck className="w-5 h-5" />
              <span>Verify My Email</span>
            </button>
          </div>
        )}

        {/* STATE: VERIFYING IN PROGRESS */}
        {status === "verifying" && (
          <div className="text-center py-8 space-y-4">
            <Spinner size="lg" text="Activating your account..." />
            <p className="text-base text-gray-600 font-normal">
              Communicating with security services to verify your token...
            </p>
          </div>
        )}

        {/* STATE: SUCCESS */}
        {status === "success" && (
          <div className="text-center space-y-6">
            <div className="w-16 h-16 bg-emerald-100 text-emerald-800 rounded-full flex items-center justify-center mx-auto">
              <CheckCircle2 className="w-10 h-10 text-emerald-700" />
            </div>
            <div className="space-y-2">
              <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
                Email Verified Successfully!
              </h1>
              <p className="text-base sm:text-lg text-gray-600 leading-relaxed font-normal">
                {message || "Your email address has been verified and your account is now fully active."}
              </p>
            </div>

            <div className="pt-4">
              <Link
                href="/login"
                rel="noreferrer noopener"
                className="w-full inline-flex items-center justify-center gap-2 py-3.5 px-6 rounded-xl text-base font-bold text-white bg-emerald-800 hover:bg-emerald-900 transition shadow-sm"
              >
                <span>Proceed to Login</span>
                <ArrowRight className="w-5 h-5" />
              </Link>
            </div>
          </div>
        )}

        {/* STATE: ALREADY VERIFIED */}
        {status === "already_verified" && (
          <div className="text-center space-y-6">
            <div className="w-16 h-16 bg-emerald-50 text-emerald-800 rounded-full flex items-center justify-center mx-auto border border-emerald-100">
              <CheckCircle2 className="w-9 h-9 text-emerald-700" />
            </div>
            <div className="space-y-2">
              <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
                Account Already Verified
              </h1>
              <p className="text-base sm:text-lg text-gray-600 leading-relaxed font-normal">
                {message || "This email address is already verified and active. You can log in directly."}
              </p>
            </div>

            <div className="pt-4">
              <Link
                href="/login"
                rel="noreferrer noopener"
                className="w-full inline-flex items-center justify-center gap-2 py-3.5 px-6 rounded-xl text-base font-bold text-white bg-emerald-800 hover:bg-emerald-900 transition shadow-sm"
              >
                <span>Go to Login</span>
                <ArrowRight className="w-5 h-5" />
              </Link>
            </div>
          </div>
        )}

        {/* STATE: EXPIRED LINK */}
        {status === "expired" && (
          <div className="space-y-6">
            <div className="text-center space-y-3">
              <div className="w-16 h-16 bg-amber-50 text-amber-700 rounded-full flex items-center justify-center mx-auto border border-amber-200">
                <AlertTriangle className="w-9 h-9 text-amber-600" />
              </div>
              <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
                Verification Link Expired
              </h1>
              <p className="text-base sm:text-lg text-gray-600 leading-relaxed font-normal">
                {message || "This verification link has expired. Links are valid for 24 hours for security."}
              </p>
            </div>

            {resendErr && <Alert type="error">{resendErr}</Alert>}
            {resendMsg && <Alert type="success">{resendMsg}</Alert>}

            {/* Inline Resend Form */}
            <form onSubmit={handleResend} className="space-y-3 pt-2">
              <label htmlFor="resendEmail" className="block text-xs font-bold uppercase text-gray-600">
                Enter your email to request a new link
              </label>
              <div className="flex gap-2">
                <input
                  id="resendEmail"
                  type="email"
                  value={resendEmail}
                  onChange={(e) => setResendEmail(e.target.value)}
                  placeholder="volunteer@example.org"
                  required
                  className="flex-1 px-4 py-2.5 text-sm rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-emerald-600"
                />
                <button
                  type="submit"
                  disabled={resending || !resendEmail}
                  className="px-5 py-2.5 bg-emerald-800 hover:bg-emerald-900 text-white text-sm font-bold rounded-lg disabled:opacity-50 transition"
                >
                  {resending ? "Sending..." : "Resend Link"}
                </button>
              </div>
            </form>

            <div className="pt-4 border-t border-gray-100 text-center">
              <Link href="/login" rel="noreferrer noopener" className="text-sm font-bold text-emerald-800 hover:underline">
                Return to Login
              </Link>
            </div>
          </div>
        )}

        {/* STATE: INVALID LINK OR NO TOKEN */}
        {(status === "invalid" || status === "no_token") && (
          <div className="space-y-6">
            <div className="text-center space-y-3">
              <div className="w-16 h-16 bg-red-50 text-red-700 rounded-full flex items-center justify-center mx-auto border border-red-200">
                <XCircle className="w-9 h-9 text-red-600" />
              </div>
              <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
                {status === "no_token" ? "Verification Link Required" : "Invalid Verification Link"}
              </h1>
              <p className="text-base sm:text-lg text-gray-600 leading-relaxed font-normal">
                {status === "no_token"
                  ? "No verification token was detected in your link. Please open the link sent to your email inbox."
                  : message || "This link is malformed or has already been consumed."}
              </p>
            </div>

            {resendErr && <Alert type="error">{resendErr}</Alert>}
            {resendMsg && <Alert type="success">{resendMsg}</Alert>}

            {/* Inline Resend Form */}
            <form onSubmit={handleResend} className="space-y-3 pt-2">
              <label htmlFor="invalidResendEmail" className="block text-xs font-bold uppercase text-gray-600">
                Request a fresh verification link
              </label>
              <div className="flex gap-2">
                <input
                  id="invalidResendEmail"
                  type="email"
                  value={resendEmail}
                  onChange={(e) => setResendEmail(e.target.value)}
                  placeholder="volunteer@example.org"
                  required
                  className="flex-1 px-4 py-2.5 text-sm rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-emerald-600"
                />
                <button
                  type="submit"
                  disabled={resending || !resendEmail}
                  className="px-5 py-2.5 bg-emerald-800 hover:bg-emerald-900 text-white text-sm font-bold rounded-lg disabled:opacity-50 transition"
                >
                  {resending ? "Sending..." : "Resend Link"}
                </button>
              </div>
            </form>

            <div className="pt-4 border-t border-gray-100 flex items-center justify-between text-xs sm:text-sm">
              <Link href="/login" rel="noreferrer noopener" className="font-bold text-emerald-800 hover:underline">
                Go to Login
              </Link>
              <Link href="/register" rel="noreferrer noopener" className="font-medium text-gray-500 hover:underline">
                Create an account
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={<Spinner text="Loading verification..." />}>
      <VerifyEmailContent />
    </Suspense>
  );
}
