"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Shield, User, Mail, Phone, Lock, MapPin, CheckCircle2 } from "lucide-react";
import { apiRequest } from "@/lib/api";
import { State, LGA } from "@/lib/types";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function RegisterPage() {
  const router = useRouter();

  const [step, setStep] = useState<"register" | "verify">("register");
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  // Form fields
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [homeStateId, setHomeStateId] = useState("");
  const [homeLgaId, setHomeLgaId] = useState("");
  const [address, setAddress] = useState("");
  const [acceptRules, setAcceptRules] = useState(false);

  // Verification step fields
  const [verificationCode, setVerificationCode] = useState("");
  const [devCodeHint, setDevCodeHint] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  useEffect(() => {
    async function loadStates() {
      try {
        const data = await apiRequest<State[]>("/geography/states");
        setStates(data || []);
      } catch (err) {
        console.error("Failed to load states", err);
      }
    }
    loadStates();
  }, []);

  useEffect(() => {
    if (!homeStateId) {
      setLgas([]);
      setHomeLgaId("");
      return;
    }
    async function loadLgas() {
      try {
        const data = await apiRequest<LGA[]>(`/geography/states/${homeStateId}/lgas`);
        setLgas(data || []);
      } catch (err) {
        console.error("Failed to load LGAs", err);
      }
    }
    loadLgas();
  }, [homeStateId]);

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!acceptRules) {
      setError("You must accept the community guidelines and privacy notice to register.");
      return;
    }

    setError(null);
    setLoading(true);

    try {
      const payload = {
        display_name: displayName,
        email: email || undefined,
        phone: phone || undefined,
        password,
        home_state_id: homeStateId || undefined,
        home_lga_id: homeLgaId || undefined,
        address: address || undefined,
        accept_rules: true,
      };

      const res = await apiRequest("/auth/register", {
        method: "POST",
        body: JSON.stringify(payload),
      });

      if (res._dev_verification_code) {
        setDevCodeHint(res._dev_verification_code);
      }

      setStep("verify");
      setSuccessMsg("Account created! Please enter your 6-digit contact verification code.");
    } catch (err: any) {
      setError(err.message || "Registration failed. Please check inputs.");
    } finally {
      setLoading(false);
    }
  };

  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      await apiRequest("/auth/verify", {
        method: "POST",
        body: JSON.stringify({
          identifier: email || phone,
          code: verificationCode.trim(),
        }),
      });

      setSuccessMsg("Contact successfully verified! Redirecting to sign in...");
      setTimeout(() => {
        router.push("/login");
      }, 1500);
    } catch (err: any) {
      setError(err.message || "Invalid or expired verification code.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-lg mx-auto px-4 py-12 space-y-6">
      <div className="text-center space-y-2">
        <div className="w-12 h-12 rounded-xl bg-emerald-900 text-white flex items-center justify-center mx-auto shadow-sm">
          <Shield className="w-6 h-6 text-emerald-400" />
        </div>
        <h1 className="text-2xl font-bold text-gray-900">Join the Nigerian Ratels</h1>
        <p className="text-xs text-gray-600">Register as a movement volunteer member</p>
      </div>

      <div className="bg-amber-50 border border-amber-200 rounded-xl p-3.5 text-xs text-amber-900 leading-relaxed">
        <strong>Anti-Privilege Guard:</strong> All new accounts strictly register as volunteer members.
        Coordinator, Zonal, and State leadership roles are granted exclusively through verified official appointments.
      </div>

      {error && <Alert type="error">{error}</Alert>}
      {successMsg && <Alert type="success">{successMsg}</Alert>}

      <div className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm">
        {step === "register" ? (
          <form onSubmit={handleRegister} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Full Display Name *
              </label>
              <div className="relative">
                <User className="w-4 h-4 text-gray-400 absolute left-3 top-3.5" />
                <input
                  type="text"
                  required
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="e.g. Terna Peter"
                  className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Email Address</label>
                <div className="relative">
                  <Mail className="w-4 h-4 text-gray-400 absolute left-3 top-3.5" />
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="name@example.com"
                    className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Phone Number</label>
                <div className="relative">
                  <Phone className="w-4 h-4 text-gray-400 absolute left-3 top-3.5" />
                  <input
                    type="tel"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                    placeholder="+2348000000000"
                    className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                  />
                </div>
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Password *</label>
              <div className="relative">
                <Lock className="w-4 h-4 text-gray-400 absolute left-3 top-3.5" />
                <input
                  type="password"
                  required
                  minLength={8}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Minimum 8 characters"
                  className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Home State / FCT</label>
                <select
                  value={homeStateId}
                  onChange={(e) => setHomeStateId(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
                >
                  <option value="">Select State or Territory</option>
                  {states.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} {s.is_fct ? "(Territory)" : ""}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Local Government / Area Council</label>
                <select
                  value={homeLgaId}
                  onChange={(e) => setHomeLgaId(e.target.value)}
                  disabled={!homeStateId || lgas.length === 0}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white disabled:bg-gray-100"
                >
                  <option value="">Select Local Area</option>
                  {lgas.map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="pt-2">
              <label className="flex items-start gap-2 cursor-pointer text-xs text-gray-600">
                <input
                  type="checkbox"
                  checked={acceptRules}
                  onChange={(e) => setAcceptRules(e.target.checked)}
                  className="mt-0.5 rounded border-gray-300 text-emerald-800 focus:ring-emerald-700"
                />
                <span>
                  I agree to the Ratels community sanitation charter and acknowledge that membership data is
                  processed under NDPA 2023 safeguarding principles.
                </span>
              </label>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-3 px-4 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition disabled:opacity-50 mt-4"
            >
              {loading ? "Creating Account..." : "Complete Registration"}
            </button>
          </form>
        ) : (
          <form onSubmit={handleVerify} className="space-y-4">
            <div className="text-center space-y-1">
              <h2 className="text-lg font-bold text-gray-900">Enter Verification Code</h2>
              <p className="text-xs text-gray-500">
                A 6-digit confirmation code was sent to {email || phone}.
              </p>
            </div>

            {devCodeHint && (
              <div className="p-3 bg-emerald-50 rounded-lg border border-emerald-200 text-xs text-emerald-900 text-center font-medium">
                [Dev Sandbox Code]: <strong>{devCodeHint}</strong>
              </div>
            )}

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1 text-center">
                6-Digit Code
              </label>
              <input
                type="text"
                required
                maxLength={6}
                value={verificationCode}
                onChange={(e) => setVerificationCode(e.target.value)}
                placeholder="123456"
                className="w-full text-center tracking-widest text-xl font-bold py-2 border border-gray-300 rounded-lg focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <button
              type="submit"
              disabled={loading || verificationCode.length < 6}
              className="w-full py-3 px-4 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition disabled:opacity-50"
            >
              {loading ? "Verifying..." : "Verify & Activate Account"}
            </button>
          </form>
        )}

        <div className="text-center text-xs text-gray-600 pt-6 border-t border-gray-100 mt-6">
          Already registered?{" "}
          <Link href="/login" className="text-emerald-800 font-semibold hover:underline">
            Sign In here
          </Link>
        </div>
      </div>
    </div>
  );
}
