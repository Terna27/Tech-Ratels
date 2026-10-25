"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  Trash2,
  Calendar,
  MapPin,
  CheckCircle2,
  Clock,
  ArrowLeft,
  Download,
  Filter,
  BarChart3,
  ShieldCheck,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicCleanupsData, State } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function CleanupsReportPage() {
  const [data, setData] = useState<PublicCleanupsData | null>(null);
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
        const res = await apiRequest<PublicCleanupsData>(`/public/reports/cleanups?${qp.toString()}`);
        if (res) setData(res);
      } catch (err) {
        console.error("Failed loading cleanups report", err);
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
        <span className="text-gray-900 font-bold">Environmental Cleanups</span>
      </div>

      {/* Header */}
      <div className="bg-emerald-950 text-white rounded-3xl p-6 sm:p-8 shadow-sm border border-emerald-800 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div className="space-y-1 max-w-xl">
          <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-800 text-emerald-300 text-xs font-semibold">
            <Trash2 className="w-3.5 h-3.5" />
            <span>Monthly Environmental Sanitation</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold">Cleanup Operations & Waste Metric Reports</h1>
          <p className="text-xs text-emerald-200 leading-relaxed">
            Every last Saturday of the month, Ratels Worldwide volunteers mobilize across 774 LGAs to reclaim public spaces,
            clear drainage channels, and prevent urban flooding.
          </p>
        </div>

        {/* Filter Controls */}
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
        <Spinner text="Loading cleanup metrics..." />
      ) : (
        <>
          {/* Top Metric Cards */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Completed Cleanups</span>
              <div className="text-3xl font-extrabold text-emerald-900">
                {data?.completed_cleanups_count ?? 0}
              </div>
              <span className="text-[10px] text-gray-400">Verified coordinator venues</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Total Waste Collected</span>
              <div className="text-3xl font-extrabold text-gray-900">
                {data?.total_waste_collected_kg ? `${data.total_waste_collected_kg.toLocaleString()} kg` : "0 kg"}
              </div>
              <span className="text-[10px] text-emerald-700 font-semibold">
                {data?.total_waste_bags ?? 0} bags disposed safely
              </span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Verified Attendees</span>
              <div className="text-3xl font-extrabold text-emerald-900">
                {data?.verified_attendees_count ?? 0}
              </div>
              <span className="text-[10px] text-gray-400">
                {data?.unique_participants_count ?? 0} unique volunteers
              </span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Active States & LGAs</span>
              <div className="text-3xl font-extrabold text-gray-900">
                {data?.active_states_count ?? 0}
                <span className="text-sm font-normal text-gray-400"> / {data?.active_lgas_count ?? 0} LGAs</span>
              </div>
              <span className="text-[10px] text-gray-400">Geographic footprint</span>
            </div>
          </div>

          {/* State Activity Breakdown */}
          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm p-6 space-y-4">
            <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
              <MapPin className="w-4 h-4 text-emerald-800" />
              <span>State Activity & Waste Breakdown</span>
            </h2>

            {!data?.state_breakdown || data.state_breakdown.length === 0 ? (
              <div className="p-8 text-center text-xs text-gray-500">
                No state-level cleanups recorded in this filter period. Check other time boundaries.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                    <tr>
                      <th className="py-3 px-4">State</th>
                      <th className="py-3 px-4">Cleanups Completed</th>
                      <th className="py-3 px-4">Waste Collected (kg)</th>
                      <th className="py-3 px-4">Verified Attendees</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {data.state_breakdown.map((row) => (
                      <tr key={row.state_id} className="hover:bg-gray-50/50 transition">
                        <td className="py-3.5 px-4 font-bold text-gray-900">{row.state_name}</td>
                        <td className="py-3.5 px-4 font-mono font-medium">{row.cleanups_count}</td>
                        <td className="py-3.5 px-4 font-mono font-semibold text-emerald-800">
                          {row.waste_collected_kg.toLocaleString()} kg
                        </td>
                        <td className="py-3.5 px-4 font-mono text-gray-700">{row.verified_attendees}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <Alert type="info" title="Scientific Evidence Rules">
            Waste tonnage is recorded exclusively from confirmed scale weights or certified dump receipt manifests.
            Volunteer headcounts are never converted into synthetic kilogram estimates.
          </Alert>
        </>
      )}
    </div>
  );
}
