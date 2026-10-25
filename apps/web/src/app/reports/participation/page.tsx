"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  TrendingUp,
  Users,
  CheckCircle2,
  XCircle,
  ArrowLeft,
  ShieldCheck,
  AlertCircle,
  UserCheck,
  EyeOff,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicParticipationData, State } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function ParticipationReportPage() {
  const [data, setData] = useState<PublicParticipationData | null>(null);
  const [states, setStates] = useState<State[]>([]);
  const [selectedState, setSelectedState] = useState<string>("");
  const [selectedPeriod, setSelectedPeriod] = useState<string>("All Time");
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    async function loadStates() {
      try {
        const res = await apiRequest<State[]>("/geography/states");
        if (res) setStates(res);
      } catch (e) {
        console.error(e);
      }
    }
    loadStates();
  }, []);

  useEffect(() => {
    async function loadData() {
      setLoading(true);
      try {
        const qp = new URLSearchParams();
        if (selectedState) qp.set("state_id", selectedState);
        if (selectedPeriod && selectedPeriod !== "All Time") qp.set("period", selectedPeriod);
        const res = await apiRequest<PublicParticipationData>(`/public/reports/participation?${qp.toString()}`);
        if (res) setData(res);
      } catch (err) {
        console.error("Failed loading participation report", err);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [selectedState, selectedPeriod]);

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Back button and breadcrumb */}
      <div className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/reports" className="flex items-center gap-1 hover:text-emerald-800 transition">
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>All Public Reports</span>
        </Link>
        <span>/</span>
        <span className="text-gray-900 font-bold">Verified Civic Participation</span>
      </div>

      {/* Header */}
      <div className="bg-emerald-950 text-white rounded-3xl p-6 sm:p-8 shadow-sm border border-emerald-800 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div className="space-y-1 max-w-xl">
          <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-800 text-emerald-300 text-xs font-semibold">
            <UserCheck className="w-3.5 h-3.5" />
            <span>Civic Engagement & Deduplicated Attendance</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold">Verified Volunteer Participation Reports</h1>
          <p className="text-xs text-emerald-200 leading-relaxed">
            Attendance at Nigerian Ratels community activities is verified through independent coordinator review,
            anti-self-approval validation, and strict volunteer deduplication rules.
          </p>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-2 bg-emerald-900/80 p-2 rounded-xl border border-emerald-700/60 text-xs">
          <select
            value={selectedState}
            onChange={(e) => setSelectedState(e.target.value)}
            className="rounded-lg bg-emerald-950 border-emerald-700 text-white text-xs py-1.5 px-2.5 focus:ring-emerald-500"
          >
            <option value="">All 36 States + FCT</option>
            {states.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>

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
        <Spinner text="Loading participation statistics..." />
      ) : (
        <>
          {/* Top Metric Cards */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Claims Submitted</span>
              <div className="text-3xl font-extrabold text-gray-900">
                {data?.total_claims_submitted ?? 0}
              </div>
              <span className="text-[10px] text-gray-400">Total volunteer claims</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Approved Claims</span>
              <div className="text-3xl font-extrabold text-emerald-900">
                {data?.approved_claims_count ?? 0}
              </div>
              <span className="text-[10px] text-emerald-700 font-semibold">
                {data?.approval_rate_percent ? `${data.approval_rate_percent.toFixed(1)}%` : "0%"} approval rate
              </span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Unique Volunteers</span>
              <div className="text-3xl font-extrabold text-emerald-800">
                {data?.unique_members_suppressed
                  ? data.unique_members_suppressed
                  : (data?.unique_verified_members ?? 0)}
              </div>
              <span className="text-[10px] text-gray-400">Deduplicated participants</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Rejected / Incomplete</span>
              <div className="text-3xl font-extrabold text-gray-500">
                {data?.rejected_claims_count ?? 0}
              </div>
              <span className="text-[10px] text-gray-400">Failed evidence check</span>
            </div>
          </div>

          {/* Privacy & Anti-Deanonymization Callout */}
          <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-3 shadow-sm">
            <div className="flex items-center gap-2">
              <EyeOff className="w-5 h-5 text-emerald-700" />
              <h3 className="font-bold text-gray-900 text-sm">Counting Distinctions & Privacy Safeguards</h3>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs text-gray-600">
              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="font-bold text-gray-900">1. Attendance Claims vs People</span>
                <p>
                  A volunteer who attends 5 cleanups generates 5 approved attendance claims, but is counted as exactly 1
                  unique participant in annual summaries.
                </p>
              </div>

              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="font-bold text-gray-900">2. Small-Cell Suppression</span>
                <p>
                  To prevent re-identification in rural LGAs, any cell with fewer than 5 participants is suppressed
                  from granular public display.
                </p>
              </div>

              <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                <span className="font-bold text-gray-900">3. Anti-Self-Approval Enforcement</span>
                <p>
                  Coordinators are architecturally blocked by the database from reviewing or approving their own
                  participation records.
                </p>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
