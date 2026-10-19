"use client";

import React, { useEffect, useState } from "react";
import { Heart, ShieldCheck, Lock, AlertCircle, FileCheck, Landmark } from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicFoundation, PublicTransparencySummary } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/Spinner";

export default function FoundationPage() {
  const [foundation, setFoundation] = useState<PublicFoundation | null>(null);
  const [transparency, setTransparency] = useState<PublicTransparencySummary | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function loadFoundation() {
      try {
        const [foundData, transData] = await Promise.all([
          apiRequest<PublicFoundation>("/foundation"),
          apiRequest<PublicTransparencySummary>("/finance/transparency").catch(() => null),
        ]);
        setFoundation(foundData);
        setTransparency(transData);
      } catch (err) {
        console.error("Failed to load foundation info", err);
      } finally {
        setLoading(false);
      }
    }
    loadFoundation();
  }, []);

  if (loading) return <Spinner text="Loading Foundation records..." />;

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-10">
      <div className="space-y-4">
        <Badge variant="warning" size="md">
          Awaiting Official Verification
        </Badge>
        <h1 className="text-3xl font-extrabold text-gray-900 tracking-tight">
          {foundation?.name || "Martins Vincent Otse Foundation"}
        </h1>
        <p className="text-base text-gray-600 leading-relaxed">
          The humanitarian relief and community development foundation affiliated with the movement.
        </p>
      </div>

      {/* Verification Notice */}
      <Alert type="warning" title="Regulatory Notice & Bank Details Protection">
        {foundation?.donation_notice ||
          "Official donation details and verified bank accounts will be published after regulatory and trustee verification."}
      </Alert>

      {/* Mission & Purpose */}
      <section className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-4">
        <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
          <Heart className="w-5 h-5 text-emerald-700" />
          Foundation Purpose & Focus
        </h2>
        <p className="text-sm text-gray-700 leading-relaxed">
          {foundation?.mission_statement ||
            "Dedicated to grassroots humanitarian relief, community empowerment, and environmental sanitation interventions across Nigeria."}
        </p>
      </section>

      {/* Public Financial Transparency Summary */}
      {transparency && (
        <section className="bg-gradient-to-br from-slate-900 to-slate-950 text-white p-6 sm:p-8 rounded-2xl border border-slate-800 shadow-lg space-y-5">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 border-b border-slate-800 pb-4">
            <div>
              <div className="text-emerald-400 text-xs font-semibold uppercase tracking-wider">Public Accountability</div>
              <h2 className="text-xl font-bold text-white flex items-center gap-2 mt-0.5">
                <ShieldCheck className="w-5 h-5 text-emerald-400" /> Verified Financial Stewardship
              </h2>
            </div>
            <span className="text-xs text-slate-400">Certified by {transparency.certified_by}</span>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-1">
            <div className="bg-slate-800/60 p-4 rounded-xl border border-slate-700/60">
              <div className="text-xs text-slate-400">Verified Income</div>
              <div className="text-lg font-bold text-emerald-400 mt-1 font-mono">
                ₦{(transparency.total_verified_income_kobo / 100).toLocaleString("en-NG", { minimumFractionDigits: 2 })}
              </div>
            </div>
            <div className="bg-slate-800/60 p-4 rounded-xl border border-slate-700/60">
              <div className="text-xs text-slate-400">Verified Projects</div>
              <div className="text-lg font-bold text-blue-400 mt-1 font-mono">
                ₦{(transparency.total_project_funding_kobo / 100).toLocaleString("en-NG", { minimumFractionDigits: 2 })}
              </div>
            </div>
            <div className="bg-slate-800/60 p-4 rounded-xl border border-slate-700/60">
              <div className="text-xs text-slate-400">Foundation Donations</div>
              <div className="text-lg font-bold text-purple-400 mt-1 font-mono">
                ₦{(transparency.foundation_donations_total_kobo / 100).toLocaleString("en-NG", { minimumFractionDigits: 2 })}
              </div>
            </div>
            <div className="bg-slate-800/60 p-4 rounded-xl border border-slate-700/60">
              <div className="text-xs text-slate-400">Available Reserves</div>
              <div className="text-lg font-bold text-amber-400 mt-1 font-mono">
                ₦{(transparency.available_reserve_balance_kobo / 100).toLocaleString("en-NG", { minimumFractionDigits: 2 })}
              </div>
            </div>
          </div>
        </section>
      )}

      {/* Official Bank Account Verification Framework */}
      <section className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-6">
        <div>
          <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
            <Landmark className="w-5 h-5 text-emerald-700" />
            Official Donation Account Verification Protocol
          </h2>
          <p className="text-xs text-gray-600 mt-1">
            To prevent fraud, impersonation, or unauthorized account substitution, the platform enforces strict dual-officer controls.
          </p>
        </div>

        <div className="p-6 bg-gray-50 rounded-xl border border-dashed border-gray-300 text-center space-y-3">
          <div className="w-12 h-12 mx-auto rounded-full bg-amber-100 text-amber-800 flex items-center justify-center">
            <Lock className="w-6 h-6" />
          </div>
          <h3 className="font-bold text-gray-900 text-base">
            Donation Collection Not Active in Phase 1
          </h3>
          <p className="text-xs text-gray-600 max-w-md mx-auto leading-relaxed">
            Bank account details will be published only after official documentation is verified by two independent
            trustee finance officers. No fictitious or unverified account numbers are published on this platform.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 pt-2 text-xs">
          <div className="p-4 bg-white rounded-xl border border-gray-200 space-y-1.5">
            <div className="font-semibold text-gray-900 flex items-center gap-1.5">
              <FileCheck className="w-4 h-4 text-emerald-700" />
              1. Document Review
            </div>
            <p className="text-gray-600">
              Registration certificate and official bank mandates are cross-checked against registered legal trustees.
            </p>
          </div>

          <div className="p-4 bg-white rounded-xl border border-gray-200 space-y-1.5">
            <div className="font-semibold text-gray-900 flex items-center gap-1.5">
              <ShieldCheck className="w-4 h-4 text-emerald-700" />
              2. Dual-Officer Sign-Off
            </div>
            <p className="text-gray-600">
              An account version requires two distinct authorized officers before any public display approval.
            </p>
          </div>

          <div className="p-4 bg-white rounded-xl border border-gray-200 space-y-1.5">
            <div className="font-semibold text-gray-900 flex items-center gap-1.5">
              <Lock className="w-4 h-4 text-emerald-700" />
              3. Versioned Instructions
            </div>
            <p className="text-gray-600">
              Published bank accounts are versioned and audited. Reversals and bank statements are balanced to avoid double counting.
            </p>
          </div>
        </div>
      </section>

      {/* Disclaimer */}
      <div className="p-4 bg-gray-100 rounded-xl text-xs text-gray-600 border border-gray-200 leading-relaxed">
        <strong>Transparency Disclaimer:</strong> {foundation?.disclaimer}
      </div>
    </div>
  );
}
