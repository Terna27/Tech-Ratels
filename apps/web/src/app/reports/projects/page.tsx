"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  FolderKanban,
  CheckCircle2,
  Clock,
  MapPin,
  ArrowLeft,
  DollarSign,
  ExternalLink,
  ChevronRight,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicCommunityProjectsData, State, LGA } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function ProjectsReportPage() {
  const [data, setData] = useState<PublicCommunityProjectsData | null>(null);
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);
  const [selectedState, setSelectedState] = useState<string>("");
  const [selectedLGA, setSelectedLGA] = useState<string>("");
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
    if (!selectedState) {
      setLgas([]);
      setSelectedLGA("");
      return;
    }
    async function loadLGAs() {
      try {
        const res = await apiRequest<LGA[]>(`/geography/states/${selectedState}/lgas`);
        if (res) setLgas(res);
      } catch (e) {
        console.error(e);
      }
    }
    loadLGAs();
  }, [selectedState]);

  useEffect(() => {
    async function loadData() {
      setLoading(true);
      try {
        const qp = new URLSearchParams();
        if (selectedState) qp.set("state_id", selectedState);
        if (selectedLGA) qp.set("lga_id", selectedLGA);
        const res = await apiRequest<PublicCommunityProjectsData>(`/public/reports/projects?${qp.toString()}`);
        if (res) setData(res);
      } catch (err) {
        console.error("Failed loading community projects report", err);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [selectedState, selectedLGA]);

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "completed":
        return <Badge variant="success" size="sm">COMPLETED</Badge>;
      case "in_progress":
        return <Badge variant="primary" size="sm">IN PROGRESS</Badge>;
      case "awaiting_verification":
        return <Badge variant="warning" size="sm">AWAITING VERIFY</Badge>;
      default:
        return <Badge variant="neutral" size="sm">{status.toUpperCase()}</Badge>;
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-xs text-gray-500 font-medium">
        <Link href="/reports" className="flex items-center gap-1 hover:text-emerald-800 transition">
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>All Public Reports</span>
        </Link>
        <span>/</span>
        <span className="text-gray-900 font-bold">Community Projects</span>
      </div>

      {/* Header */}
      <div className="bg-emerald-950 text-white rounded-3xl p-6 sm:p-8 shadow-sm border border-emerald-800 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div className="space-y-1 max-w-xl">
          <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-emerald-800 text-emerald-300 text-xs font-semibold">
            <FolderKanban className="w-3.5 h-3.5" />
            <span>Community Infrastructure & Sanitation</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold">Community Projects Progress Reports</h1>
          <p className="text-xs text-emerald-200 leading-relaxed">
            Independent physical inspection records, milestone verifications, and reconciled financial disbursements
            for approved community development projects across Nigerian wards and LGAs.
          </p>
        </div>

        {/* Geographic Filter */}
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
            value={selectedLGA}
            onChange={(e) => setSelectedLGA(e.target.value)}
            disabled={!selectedState}
            className="rounded-lg bg-emerald-950 border-emerald-700 text-white text-xs py-1.5 px-2.5 focus:ring-emerald-500 disabled:opacity-50"
          >
            <option value="">{selectedState ? "All LGAs" : "Select State First"}</option>
            {lgas.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      {loading ? (
        <Spinner text="Loading community project records..." />
      ) : (
        <>
          {/* Top Metric Cards */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Total Approved Projects</span>
              <div className="text-3xl font-extrabold text-gray-900">
                {data?.total_projects_count ?? 0}
              </div>
              <span className="text-[10px] text-gray-400">Formal community charters</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Physically Completed</span>
              <div className="text-3xl font-extrabold text-emerald-900">
                {data?.completed_projects_count ?? 0}
              </div>
              <span className="text-[10px] text-emerald-700 font-semibold">100% milestone sign-off</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Total Approved Budget</span>
              <div className="text-2xl font-extrabold text-gray-900">
                {data?.total_approved_budget_formatted || "₦0.00"}
              </div>
              <span className="text-[10px] text-gray-400">Allocated project capital</span>
            </div>

            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-medium">Recorded Expenditure</span>
              <div className="text-2xl font-extrabold text-emerald-800">
                {data?.total_recorded_expenditure_formatted || "₦0.00"}
              </div>
              <span className="text-[10px] text-gray-400">Verified voucher spending</span>
            </div>
          </div>

          {/* Projects Table */}
          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm p-6 space-y-4">
            <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
              <FolderKanban className="w-4 h-4 text-emerald-800" />
              <span>Approved Public Projects Status</span>
            </h2>

            {!data?.projects || data.projects.length === 0 ? (
              <div className="p-8 text-center text-xs text-gray-500">
                No approved public projects found for this location.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                    <tr>
                      <th className="py-3 px-4">Reference</th>
                      <th className="py-3 px-4">Project Title</th>
                      <th className="py-3 px-4">Location</th>
                      <th className="py-3 px-4">Category</th>
                      <th className="py-3 px-4">Progress</th>
                      <th className="py-3 px-4">Status</th>
                      <th className="py-3 px-4 text-right">Budget (NGN)</th>
                      <th className="py-3 px-4 text-right">Details</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {data.projects.map((pr) => (
                      <tr key={pr.id} className="hover:bg-gray-50/50 transition">
                        <td className="py-3.5 px-4 font-mono font-bold text-emerald-950">{pr.reference}</td>
                        <td className="py-3.5 px-4 font-bold text-gray-900">{pr.title}</td>
                        <td className="py-3.5 px-4 text-gray-600">
                          {pr.state_name} • {pr.lga_name} ({pr.community_ward})
                        </td>
                        <td className="py-3.5 px-4">
                          <span className="capitalize">{pr.category.replace(/_/g, " ")}</span>
                        </td>
                        <td className="py-3.5 px-4">
                          <div className="flex items-center gap-2">
                            <div className="w-16 bg-gray-200 rounded-full h-1.5 overflow-hidden">
                              <div
                                className="bg-emerald-700 h-1.5 rounded-full"
                                style={{ width: `${pr.progress_percentage}%` }}
                              />
                            </div>
                            <span className="font-mono text-[10px] text-gray-600 font-semibold">
                              {pr.progress_percentage}%
                            </span>
                          </div>
                        </td>
                        <td className="py-3.5 px-4">{getStatusBadge(pr.execution_status)}</td>
                        <td className="py-3.5 px-4 text-right font-mono font-bold text-gray-900">
                          {pr.approved_budget_formatted}
                        </td>
                        <td className="py-3.5 px-4 text-right">
                          <Link
                            href={`/projects/${pr.id}`}
                            className="p-1.5 rounded-lg bg-gray-50 hover:bg-emerald-50 text-emerald-800 inline-flex items-center gap-1 font-semibold transition"
                          >
                            <span>View</span>
                            <ChevronRight className="w-3.5 h-3.5" />
                          </Link>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
