"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  Heart,
  ShieldCheck,
  ArrowLeft,
  DollarSign,
  AlertTriangle,
  Lock,
  Layers,
  Sparkles,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicFoundationData } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function FoundationReportPage() {
  const [data, setData] = useState<PublicFoundationData | null>(null);
  const [selectedPeriod, setSelectedPeriod] = useState<string>("All Time");
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    async function loadData() {
      setLoading(true);
      try {
        const qp = new URLSearchParams();
        if (selectedPeriod && selectedPeriod !== "All Time") qp.set("period", selectedPeriod);
        const res = await apiRequest<PublicFoundationData>(`/public/reports/foundation?${qp.toString()}`);
        if (res) setData(res);
      } catch (err) {
        console.error("Failed loading foundation report", err);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [selectedPeriod]);

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/reports" className="flex items-center gap-1 hover:text-emerald-800 transition">
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>All Public Reports</span>
        </Link>
        <span>/</span>
        <span className="text-gray-900 font-bold">Foundation Transparency</span>
      </div>

      {/* Header */}
      <div className="bg-gradient-to-r from-emerald-950 via-emerald-900 to-gray-900 text-white rounded-3xl p-6 sm:p-8 shadow-sm border border-emerald-800 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div className="space-y-1 max-w-xl">
          <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-800 text-emerald-300 text-xs font-semibold">
            <Heart className="w-3.5 h-3.5" />
            <span>Dedicated Charitable Trust Accounting</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold">Martinsvincent Ortse Foundation Reports</h1>
          <p className="text-xs text-emerald-200 leading-relaxed">
            Transparent reporting of charitable donations received, humanitarian outreaches, and independent foundation fund
            segregation in strict compliance with non-profit trust standards.
          </p>
        </div>

        {/* Period Filter */}
        <div className="flex items-center gap-2 bg-emerald-900/80 p-2 rounded-xl border border-emerald-700/60 text-xs">
          <select
            value={selectedPeriod}
            onChange={(e) => setSelectedPeriod(e.target.value)}
            className="rounded-lg bg-emerald-950 border-emerald-700 text-white text-xs py-1.5 px-2.5 focus:ring-emerald-500"
          >
            <option value="All Time">All Time</option>
            <option value="2026">2026 Annual</option>
            <option value="2026-10">October 2026</option>
            <option value="2026-09">September 2026</option>
            <option value="2025">2025 Annual</option>
          </select>
        </div>
      </div>

      {loading ? (
        <Spinner text="Loading foundation records..." />
      ) : (
        <>
          {/* Top Metric Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Total Verified Donations</span>
              <div className="text-3xl font-extrabold text-emerald-900">
                {data?.total_donations_formatted || "₦0.00"}
              </div>
              <span className="text-[10px] text-gray-400">Reconciled charitable gifts</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Completed Donations Count</span>
              <div className="text-3xl font-extrabold text-gray-900">
                {data?.verified_donations_count ?? 0}
              </div>
              <span className="text-[10px] text-emerald-700 font-semibold">Verified transactions</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Dual-Signatory Clearing</span>
              <div className="text-3xl font-extrabold text-emerald-800 flex items-center gap-2">
                <span>Active</span>
                <ShieldCheck className="w-6 h-6 text-emerald-600" />
              </div>
              <span className="text-[10px] text-gray-400">Restricted humanitarian account</span>
            </div>
          </div>

          {/* Institutional Governance Disclaimers */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="bg-amber-50/60 rounded-2xl border border-amber-200 p-6 space-y-3">
              <div className="flex items-center gap-2 text-amber-900">
                <AlertTriangle className="w-5 h-5 text-amber-700" />
                <h3 className="font-bold text-sm">Provisional Legal Entity Status</h3>
              </div>
              <p className="text-xs text-amber-950 leading-relaxed">
                {data?.provisional_entity_notice ||
                  "Martinsvincent Ortse Foundation registration is provisional pending final Corporate Affairs Commission (CAC) review. Dedicated charitable accounts operate with dual-signatory verification."}
              </p>
            </div>

            <div className="bg-emerald-50/60 rounded-2xl border border-emerald-200 p-6 space-y-3">
              <div className="flex items-center gap-2 text-emerald-900">
                <Layers className="w-5 h-5 text-emerald-700" />
                <h3 className="font-bold text-sm">Strict Financial Segregation</h3>
              </div>
              <p className="text-xs text-emerald-950 leading-relaxed">
                {data?.strict_segregation_notice ||
                  "Foundation humanitarian funds are strictly separated from Nigerian Ratels operational funds in double-entry ledgers. Zero cross-commingling of membership dues and charitable grants."}
              </p>
            </div>
          </div>

          {/* Donor Privacy & Live Switch Notice */}
          <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-3 shadow-sm">
            <div className="flex items-center gap-2 text-gray-900">
              <Lock className="w-5 h-5 text-emerald-800" />
              <h3 className="font-bold text-sm">Donor Privacy & Operational Safety Standards</h3>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs text-gray-600">
              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="font-bold text-gray-900">1. Permanent Donor Anonymity</span>
                <p>
                  Individual donor identities, card details, and personal contributions are completely redacted from
                  public portals to respect donor privacy. Only verified totals are published.
                </p>
              </div>

              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="font-bold text-gray-900">2. Payment Provider Safety Switch</span>
                <p>
                  To prevent unauthorized payment processing, live card checkout remains locked until production
                  merchant certificates are officially provisioned by the Board of Trustees.
                </p>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
