"use client";

import React, { useEffect, useState, useTransition } from "react";
import Link from "next/link";
import {
  BarChart3,
  Calendar,
  MapPin,
  CheckCircle2,
  FileText,
  Download,
  ShieldCheck,
  TrendingUp,
  DollarSign,
  Heart,
  FolderKanban,
  Trash2,
  Info,
  RefreshCw,
  Search,
  ChevronRight,
  ExternalLink,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import {
  PublicOverviewData,
  PublicPublicationDTO,
  PaginatedPublicationsResponse,
  State,
  LGA,
  GeopoliticalZone,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

export default function PublicReportsPage() {
  const [overview, setOverview] = useState<PublicOverviewData | null>(null);
  const [publications, setPublications] = useState<PublicPublicationDTO[]>([]);
  const [zones, setZones] = useState<GeopoliticalZone[]>([]);
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  // Filters
  const [selectedZone, setSelectedZone] = useState<string>("");
  const [selectedState, setSelectedState] = useState<string>("");
  const [selectedLGA, setSelectedLGA] = useState<string>("");
  const [selectedPeriod, setSelectedPeriod] = useState<string>("All Time");

  const [loading, setLoading] = useState(true);
  const [filterLoading, startFilterTransition] = useTransition();

  // Load geography reference data
  useEffect(() => {
    async function loadGeography() {
      try {
        const [zonesRes, statesRes] = await Promise.all([
          apiRequest<GeopoliticalZone[]>("/geography/zones"),
          apiRequest<State[]>("/geography/states"),
        ]);
        if (zonesRes) setZones(zonesRes);
        if (statesRes) setStates(statesRes);
      } catch (err) {
        console.error("Failed loading geography", err);
      }
    }
    loadGeography();
  }, []);

  // When selected state changes, load LGAs
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
      } catch (err) {
        console.error("Failed loading LGAs", err);
      }
    }
    loadLGAs();
  }, [selectedState]);

  // Fetch report data on filter change
  useEffect(() => {
    startFilterTransition(async () => {
      try {
        const queryParams = new URLSearchParams();
        if (selectedZone) queryParams.set("zone_id", selectedZone);
        if (selectedState) queryParams.set("state_id", selectedState);
        if (selectedLGA) queryParams.set("lga_id", selectedLGA);
        if (selectedPeriod && selectedPeriod !== "All Time") {
          queryParams.set("period", selectedPeriod);
        }

        const [ovRes, pubsRes] = await Promise.all([
          apiRequest<PublicOverviewData>(`/public/reports/overview?${queryParams.toString()}`),
          apiRequest<PaginatedPublicationsResponse>(`/public/reports/publications?${queryParams.toString()}`),
        ]);

        if (ovRes) setOverview(ovRes);
        if (pubsRes && pubsRes.publications) {
          setPublications(pubsRes.publications);
        }
      } catch (err) {
        console.error("Failed loading report overview", err);
      } finally {
        setLoading(false);
      }
    });
  }, [selectedZone, selectedState, selectedLGA, selectedPeriod]);

  const handleDownload = (pubId: string, format: "pdf" | "csv") => {
    window.open(`/api/v1/public/reports/publications/${pubId}/${format}`, "_blank");
  };

  const filteredStates = selectedZone
    ? states.filter((s) => s.zone_id === selectedZone)
    : states;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-10">
      {/* Header Banner */}
      <div className="bg-gradient-to-r from-emerald-950 via-emerald-900 to-gray-900 text-white rounded-3xl p-6 sm:p-10 shadow-lg border border-emerald-800">
        <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
          <div className="space-y-2 max-w-2xl">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-800/80 border border-emerald-600/50 text-emerald-300 text-xs font-semibold">
              <ShieldCheck className="w-4 h-4 text-emerald-400" />
              <span>Certified Public Transparency & Accountability Portal</span>
            </div>
            <h1 className="text-3xl sm:text-4xl font-extrabold tracking-tight">
              Ratels Worldwide Impact & Performance Reports
            </h1>
            <p className="text-sm text-emerald-100/90 leading-relaxed">
              Real-time, audited figures covering environmental cleanups, civic volunteer participation,
              segregated charitable donations, and community infrastructure across Nigeria’s 6 Geopolitical Zones,
              36 States, FCT, and 774 Local Government Areas.
            </p>
          </div>

          <div className="flex flex-col sm:flex-row gap-3">
            <Link
              href="/reports/methodology"
              className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-white/10 hover:bg-white/20 text-white text-xs font-bold border border-white/20 transition"
            >
              <Info className="w-4 h-4 text-emerald-300" />
              <span>Data Methodology</span>
            </Link>
          </div>
        </div>

        {/* Sub-portal Quick Navigation Links */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2.5 pt-8 mt-8 border-t border-emerald-800/80">
          <Link
            href="/reports/cleanups"
            className="p-3 rounded-xl bg-emerald-900/60 hover:bg-emerald-800/80 border border-emerald-700/50 flex flex-col justify-between transition group"
          >
            <Trash2 className="w-4 h-4 text-emerald-400 mb-1 group-hover:scale-110 transition" />
            <span className="text-xs font-bold text-white">Cleanups</span>
            <span className="text-[10px] text-emerald-300">Waste & Venues</span>
          </Link>

          <Link
            href="/reports/participation"
            className="p-3 rounded-xl bg-emerald-900/60 hover:bg-emerald-800/80 border border-emerald-700/50 flex flex-col justify-between transition group"
          >
            <TrendingUp className="w-4 h-4 text-emerald-400 mb-1 group-hover:scale-110 transition" />
            <span className="text-xs font-bold text-white">Participation</span>
            <span className="text-[10px] text-emerald-300">Verified Volunteers</span>
          </Link>

          <Link
            href="/reports/finance"
            className="p-3 rounded-xl bg-emerald-900/60 hover:bg-emerald-800/80 border border-emerald-700/50 flex flex-col justify-between transition group"
          >
            <DollarSign className="w-4 h-4 text-emerald-400 mb-1 group-hover:scale-110 transition" />
            <span className="text-xs font-bold text-white">Finances</span>
            <span className="text-[10px] text-emerald-300">Approved Ledger</span>
          </Link>

          <Link
            href="/reports/foundation"
            className="p-3 rounded-xl bg-emerald-900/60 hover:bg-emerald-800/80 border border-emerald-700/50 flex flex-col justify-between transition group"
          >
            <Heart className="w-4 h-4 text-emerald-400 mb-1 group-hover:scale-110 transition" />
            <span className="text-xs font-bold text-white">Foundation</span>
            <span className="text-[10px] text-emerald-300">Segregated Aid</span>
          </Link>

          <Link
            href="/reports/projects"
            className="p-3 rounded-xl bg-emerald-900/60 hover:bg-emerald-800/80 border border-emerald-700/50 flex flex-col justify-between transition group"
          >
            <FolderKanban className="w-4 h-4 text-emerald-400 mb-1 group-hover:scale-110 transition" />
            <span className="text-xs font-bold text-white">Projects</span>
            <span className="text-[10px] text-emerald-300">Infrastructure</span>
          </Link>

          <Link
            href="/reports/methodology"
            className="p-3 rounded-xl bg-emerald-900/60 hover:bg-emerald-800/80 border border-emerald-700/50 flex flex-col justify-between transition group"
          >
            <Info className="w-4 h-4 text-emerald-400 mb-1 group-hover:scale-110 transition" />
            <span className="text-xs font-bold text-white">Methodology</span>
            <span className="text-[10px] text-emerald-300">Counting Rules</span>
          </Link>
        </div>
      </div>

      {/* Interactive Filter Bar */}
      <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <MapPin className="w-4 h-4 text-emerald-700" />
            <h2 className="text-sm font-bold text-gray-900">Filter Scope & Reporting Period</h2>
          </div>
          {(selectedZone || selectedState || selectedLGA || selectedPeriod !== "All Time") && (
            <button
              onClick={() => {
                setSelectedZone("");
                setSelectedState("");
                setSelectedLGA("");
                setSelectedPeriod("All Time");
              }}
              className="text-xs text-emerald-800 font-semibold hover:underline flex items-center gap-1"
            >
              <RefreshCw className="w-3 h-3" />
              <span>Reset Filters</span>
            </button>
          )}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <div>
            <label className="block text-[11px] font-semibold text-gray-600 mb-1 uppercase tracking-wider">
              Geopolitical Zone
            </label>
            <select
              value={selectedZone}
              onChange={(e) => {
                setSelectedZone(e.target.value);
                setSelectedState("");
                setSelectedLGA("");
              }}
              className="w-full text-xs rounded-lg border-gray-300 focus:border-emerald-600 focus:ring-emerald-600 py-2"
            >
              <option value="">All 6 Zones</option>
              {zones.map((z) => (
                <option key={z.id} value={z.id}>
                  {z.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-[11px] font-semibold text-gray-600 mb-1 uppercase tracking-wider">
              State / FCT
            </label>
            <select
              value={selectedState}
              onChange={(e) => {
                setSelectedState(e.target.value);
                setSelectedLGA("");
              }}
              className="w-full text-xs rounded-lg border-gray-300 focus:border-emerald-600 focus:ring-emerald-600 py-2"
            >
              <option value="">All 36 States + FCT</option>
              {filteredStates.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} {s.is_fct ? "(FCT)" : ""}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-[11px] font-semibold text-gray-600 mb-1 uppercase tracking-wider">
              LGA / Area Council
            </label>
            <select
              value={selectedLGA}
              onChange={(e) => setSelectedLGA(e.target.value)}
              disabled={!selectedState}
              className="w-full text-xs rounded-lg border-gray-300 focus:border-emerald-600 focus:ring-emerald-600 py-2 disabled:bg-gray-100 disabled:text-gray-400"
            >
              <option value="">{selectedState ? "All LGAs in State" : "Select State First"}</option>
              {lgas.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-[11px] font-semibold text-gray-600 mb-1 uppercase tracking-wider">
              Reporting Period
            </label>
            <select
              value={selectedPeriod}
              onChange={(e) => setSelectedPeriod(e.target.value)}
              className="w-full text-xs rounded-lg border-gray-300 focus:border-emerald-600 focus:ring-emerald-600 py-2"
            >
              <option value="All Time">All Time</option>
              <option value="2026">2026 Annual</option>
              <option value="2026-10">October 2026</option>
              <option value="2026-09">September 2026</option>
              <option value="2026-08">August 2026</option>
              <option value="2025">2025 Annual</option>
            </select>
          </div>
        </div>
      </div>

      {loading ? (
        <Spinner text="Loading verified transparency metrics..." />
      ) : (
        <>
          {/* Headline Metric Cards */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
                <BarChart3 className="w-5 h-5 text-emerald-800" />
                <span>Verified Impact Indicators ({overview?.scope || "Nigeria Nationwide"})</span>
              </h2>
              <span className="text-xs text-gray-500 font-medium">
                Period: {overview?.reporting_period || selectedPeriod}
              </span>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
              <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
                <div className="flex items-center justify-between text-gray-500 text-xs">
                  <span>Completed Cleanups</span>
                  <Trash2 className="w-4 h-4 text-emerald-600" />
                </div>
                <div className="text-2xl font-extrabold text-gray-900">
                  {overview?.completed_cleanups_count ?? 0}
                </div>
                <div className="text-[10px] text-gray-500">Verified last Saturday venues</div>
              </div>

              <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
                <div className="flex items-center justify-between text-gray-500 text-xs">
                  <span>Verified Attendees</span>
                  <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                </div>
                <div className="text-2xl font-extrabold text-gray-900">
                  {overview?.verified_attendees_count ?? 0}
                </div>
                <div className="text-[10px] text-emerald-700 font-semibold">
                  {overview?.unique_participants_count ?? 0} unique volunteers
                </div>
              </div>

              <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
                <div className="flex items-center justify-between text-gray-500 text-xs">
                  <span>Active LGAs</span>
                  <MapPin className="w-4 h-4 text-emerald-600" />
                </div>
                <div className="text-2xl font-extrabold text-gray-900">
                  {overview?.active_lgas_count ?? 0}
                  <span className="text-xs font-normal text-gray-400"> / 774</span>
                </div>
                <div className="text-[10px] text-gray-500">
                  {overview?.active_states_count ?? 0} States covered
                </div>
              </div>

              <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1">
                <div className="flex items-center justify-between text-gray-500 text-xs">
                  <span>Completed Projects</span>
                  <FolderKanban className="w-4 h-4 text-emerald-600" />
                </div>
                <div className="text-2xl font-extrabold text-gray-900">
                  {overview?.completed_projects_count ?? 0}
                </div>
                <div className="text-[10px] text-gray-500">
                  {overview?.approved_projects_count ?? 0} approved in total
                </div>
              </div>

              <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-1 col-span-2 md:col-span-1">
                <div className="flex items-center justify-between text-gray-500 text-xs">
                  <span>Waste Collected</span>
                  <span className="text-[10px] font-bold text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded">
                    Field Data
                  </span>
                </div>
                <div className="text-2xl font-extrabold text-gray-900">
                  {overview?.total_waste_collected_kg ? `${overview.total_waste_collected_kg.toLocaleString()} kg` : "Recorded"}
                </div>
                <div className="text-[10px] text-gray-500">Documented cleanup tonnage</div>
              </div>
            </div>
          </div>

          {/* Financial Transparency Summary Cards */}
          <div className="bg-emerald-50/50 rounded-2xl border border-emerald-200 p-6 space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h3 className="text-base font-bold text-emerald-950 flex items-center gap-2">
                  <DollarSign className="w-4 h-4 text-emerald-700" />
                  <span>Approved Financial Accountability (Reconciled Ledger Figures)</span>
                </h3>
                <p className="text-xs text-emerald-900/80">
                  Separation of operational funds and foundation funds maintained under double-entry accounting.
                </p>
              </div>
              <Link
                href="/reports/finance"
                className="text-xs font-bold text-emerald-800 hover:text-emerald-950 underline self-start sm:self-auto"
              >
                View Full Financial Report &rarr;
              </Link>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="bg-white p-4 rounded-xl border border-emerald-100 shadow-xs">
                <div className="text-xs text-gray-500 font-medium">Reconciled Operational Income</div>
                <div className="text-xl font-extrabold text-emerald-900 mt-1">
                  {overview?.approved_income_formatted || "₦0.00"}
                </div>
                <div className="text-[10px] text-gray-400 mt-0.5">Authoritative ledger income</div>
              </div>

              <div className="bg-white p-4 rounded-xl border border-emerald-100 shadow-xs">
                <div className="text-xs text-gray-500 font-medium">Reconciled Operational Expenses</div>
                <div className="text-xl font-extrabold text-gray-900 mt-1">
                  {overview?.approved_expenditure_formatted || "₦0.00"}
                </div>
                <div className="text-[10px] text-gray-400 mt-0.5">Payment voucher cleared disbursements</div>
              </div>

              <div className="bg-white p-4 rounded-xl border border-emerald-100 shadow-xs">
                <div className="text-xs text-gray-500 font-medium">Foundation Segregated Donations</div>
                <div className="text-xl font-extrabold text-emerald-700 mt-1">
                  {overview?.foundation_donations_formatted || "₦0.00"}
                </div>
                <div className="text-[10px] text-gray-400 mt-0.5">Martinsvincent Ortse Foundation restricted fund</div>
              </div>
            </div>
          </div>

          {/* Published Reports & Certified Disclosures Table */}
          <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm space-y-4 p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h3 className="text-base font-bold text-gray-900 flex items-center gap-2">
                  <FileText className="w-4 h-4 text-emerald-800" />
                  <span>Certified Public Publications & Official Downloads</span>
                </h3>
                <p className="text-xs text-gray-500">
                  Immutable frozen reports signed off by National Governance. Downloadable in PDF and CSV format.
                </p>
              </div>
            </div>

            {publications.length === 0 ? (
              <EmptyState
                icon={<FileText className="w-6 h-6" />}
                title="No Published Reports in Current Scope"
                description="Official public reports are prepared by coordinators, verified by governance, and certified prior to publication. Live database metrics are shown above."
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                    <tr>
                      <th className="py-3 px-4">Report Reference</th>
                      <th className="py-3 px-4">Title</th>
                      <th className="py-3 px-4">Type</th>
                      <th className="py-3 px-4">Scope</th>
                      <th className="py-3 px-4">Period</th>
                      <th className="py-3 px-4">Published Date</th>
                      <th className="py-3 px-4">Version</th>
                      <th className="py-3 px-4 text-right">Downloads</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {publications.map((pub) => (
                      <tr key={pub.publication_id} className="hover:bg-gray-50/50 transition">
                        <td className="py-3.5 px-4 font-mono font-bold text-emerald-950">
                          {pub.public_reference}
                        </td>
                        <td className="py-3.5 px-4 font-medium text-gray-900">
                          {pub.title}
                        </td>
                        <td className="py-3.5 px-4">
                          <Badge variant="primary" size="sm">
                            {pub.report_type.replace(/_/g, " ").toUpperCase()}
                          </Badge>
                        </td>
                        <td className="py-3.5 px-4 text-gray-600">
                          {pub.state_name ? `${pub.state_name} State` : "Nationwide"}
                        </td>
                        <td className="py-3.5 px-4 text-gray-600 font-medium">
                          {pub.reporting_period}
                        </td>
                        <td className="py-3.5 px-4 text-gray-500">
                          {new Date(pub.published_at).toLocaleDateString("en-GB", {
                            day: "2-digit",
                            month: "short",
                            year: "numeric",
                          })}
                        </td>
                        <td className="py-3.5 px-4">
                          <span className="font-semibold text-gray-700">v{pub.publication_version}</span>
                        </td>
                        <td className="py-3.5 px-4 text-right">
                          <div className="inline-flex items-center gap-1.5">
                            <button
                              onClick={() => handleDownload(pub.public_reference, "pdf")}
                              title="Download Certified PDF Report"
                              className="px-2.5 py-1.5 rounded-lg bg-emerald-50 hover:bg-emerald-100 text-emerald-800 text-[11px] font-bold border border-emerald-200 flex items-center gap-1 transition"
                            >
                              <Download className="w-3 h-3" />
                              <span>PDF</span>
                            </button>
                            <button
                              onClick={() => handleDownload(pub.public_reference, "csv")}
                              title="Download Machine-Readable CSV"
                              className="px-2.5 py-1.5 rounded-lg bg-gray-50 hover:bg-gray-100 text-gray-700 text-[11px] font-bold border border-gray-200 flex items-center gap-1 transition"
                            >
                              <Download className="w-3 h-3" />
                              <span>CSV</span>
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Privacy & Safeguarding Non-Disclosure Statement */}
          <Alert type="info" title="Privacy Protection by Architecture">
            In compliance with the Nigeria Data Protection Act (NDPA) 2023, personal volunteer names,
            phone numbers, home addresses, private donation records, and confidential safeguarding incident
            reports are permanently excluded from all public transparency dashboards and downloadable exports.
          </Alert>
        </>
      )}
    </div>
  );
}
