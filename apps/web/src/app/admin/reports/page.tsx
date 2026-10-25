"use client";

import React, { useEffect, useState, useTransition } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  BarChart3,
  Plus,
  Filter,
  CheckCircle2,
  XCircle,
  FileText,
  Download,
  AlertTriangle,
  History,
  ShieldCheck,
  MapPin,
  Calendar,
  Eye,
  RefreshCw,
  Send,
  Layers,
  ArrowRight,
  ShieldAlert,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  AdminSnapshotDetailDTO,
  PaginatedSnapshotsResponse,
  ReportAuditItem,
  State,
  Zone,
  LGA,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

export default function AdminReportsWorkspacePage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<
    "all" | "pending" | "published" | "withdrawn" | "audit"
  >("all");
  const [snapshots, setSnapshots] = useState<AdminSnapshotDetailDTO[]>([]);
  const [totalCount, setTotalCount] = useState<number>(0);
  const [audits, setAudits] = useState<ReportAuditItem[]>([]);
  const [zones, setZones] = useState<Zone[]>([]);
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  // Generation Modal State
  const [showGenModal, setShowGenModal] = useState<boolean>(false);
  const [genReportType, setGenReportType] = useState<string>("cleanups");
  const [genTitle, setGenTitle] = useState<string>("");
  const [genPeriod, setGenPeriod] = useState<string>("2026");
  const [genScopeType, setGenScopeType] = useState<string>("national");
  const [genZoneID, setGenZoneID] = useState<string>("");
  const [genStateID, setGenStateID] = useState<string>("");
  const [genLgaID, setGenLgaID] = useState<string>("");
  const [isGenerating, setIsGenerating] = useState<boolean>(false);

  // Detail / Action Modal State
  const [selectedSnapshot, setSelectedSnapshot] = useState<AdminSnapshotDetailDTO | null>(null);
  const [actionReason, setActionReason] = useState<string>("");
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [isActionPending, setIsActionPending] = useState<boolean>(false);

  const [loading, setLoading] = useState<boolean>(true);

  // Authentication guard
  useEffect(() => {
    if (!isLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/reports");
    }
  }, [user, isCoordinator, isLoading, router]);

  // Load geography reference
  useEffect(() => {
    async function loadGeo() {
      try {
        const [zRes, sRes] = await Promise.all([
          apiRequest<Zone[]>("/geography/zones").catch(() => []),
          apiRequest<State[]>("/geography/states").catch(() => []),
        ]);
        if (zRes) setZones(zRes);
        if (sRes) setStates(sRes);
      } catch (e) {
        console.error(e);
      }
    }
    loadGeo();
  }, []);

  // Load LGAs dynamically when state is selected in LGA scope
  useEffect(() => {
    async function loadLGAs() {
      if (genScopeType === "lga" && genStateID) {
        try {
          const res = await apiRequest<LGA[]>(`/geography/states/${genStateID}/lgas`);
          if (res) setLgas(res);
        } catch (e) {
          console.error(e);
        }
      } else {
        setLgas([]);
      }
    }
    loadLGAs();
  }, [genScopeType, genStateID]);

  const loadSnapshots = async () => {
    setLoading(true);
    try {
      const qp = new URLSearchParams();
      if (activeTab === "pending") qp.set("status", "pending_review");
      if (activeTab === "published") qp.set("status", "published");
      if (activeTab === "withdrawn") qp.set("status", "withdrawn");

      const res = await apiRequest<PaginatedSnapshotsResponse>(
        `/admin/reports?${qp.toString()}`
      );
      if (res && res.snapshots) {
        setSnapshots(res.snapshots);
        setTotalCount(res.total_count);
      }
    } catch (err) {
      console.error("Failed loading snapshots", err);
    } finally {
      setLoading(false);
    }
  };

  const loadAudits = async () => {
    setLoading(true);
    try {
      const res = await apiRequest<ReportAuditItem[]>("/admin/reports/audit?page_size=50");
      if (res) setAudits(res);
    } catch (err) {
      console.error("Failed loading audit trail", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (user && isCoordinator) {
      if (activeTab === "audit") {
        loadAudits();
      } else {
        loadSnapshots();
      }
    }
  }, [activeTab, user, isCoordinator]);

  const handleGenerate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsGenerating(true);
    setActionError(null);
    try {
      const payload: any = {
        report_type: genReportType,
        title: genTitle || `${genReportType.toUpperCase()} - ${genPeriod}`,
        reporting_period: genPeriod,
        period_start: `${genPeriod.slice(0, 4)}-01-01`,
        period_end: `${genPeriod.slice(0, 4)}-12-31`,
        scope_type: genScopeType,
      };
      if (genScopeType === "zone" && genZoneID) {
        payload.zone_id = genZoneID;
      }
      if (genScopeType === "state" && genStateID) {
        payload.state_id = genStateID;
      }
      if (genScopeType === "lga" && genLgaID) {
        payload.state_id = genStateID;
        payload.lga_id = genLgaID;
      }

      const res = await apiRequest<AdminSnapshotDetailDTO>(
        "/admin/reports/generate",
        {
          method: "POST",
          body: JSON.stringify(payload),
        }
      );
      setShowGenModal(false);
      setGenTitle("");
      loadSnapshots();
      if (res) {
        setSelectedSnapshot(res);
      }
    } catch (err: any) {
      setActionError(err.message || "Failed generating report snapshot");
    } finally {
      setIsGenerating(false);
    }
  };

  const handleAction = async (actionType: "submit-review" | "approve" | "reject" | "publish" | "withdraw" | "revise") => {
    if (!selectedSnapshot) return;
    setIsActionPending(true);
    setActionError(null);
    setActionSuccess(null);

    try {
      let body: any = undefined;
      if (actionType === "approve") {
        body = JSON.stringify({ decision_reason: actionReason || "Approved by authorized governance reviewer" });
      } else if (actionType === "reject") {
        if (!actionReason) throw new Error("Rejection reason is required");
        body = JSON.stringify({ reason: actionReason });
      } else if (actionType === "withdraw") {
        if (!actionReason) throw new Error("Withdrawal reason is required");
        body = JSON.stringify({ reason: actionReason });
      } else if (actionType === "revise") {
        if (!actionReason) throw new Error("Correction notes are required for revision");
        body = JSON.stringify({ correction_notes: actionReason });
      }

      await apiRequest(
        `/admin/reports/${selectedSnapshot.snapshot_id}/${actionType}`,
        {
          method: "POST",
          body,
        }
      );

      setActionSuccess(`Successfully performed action: ${actionType.replace(/-/g, " ")}`);
      setActionReason("");

      // Refresh detail
      const updated = await apiRequest<AdminSnapshotDetailDTO>(
        `/admin/reports/${selectedSnapshot.snapshot_id}`
      );
      if (updated) setSelectedSnapshot(updated);
      loadSnapshots();
    } catch (err: any) {
      setActionError(err.message || `Action ${actionType} failed`);
    } finally {
      setIsActionPending(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "published":
        return <Badge variant="success" size="sm">PUBLISHED</Badge>;
      case "approved":
        return <Badge variant="primary" size="sm">APPROVED</Badge>;
      case "pending_review":
        return <Badge variant="warning" size="sm">PENDING REVIEW</Badge>;
      case "draft":
        return <Badge variant="neutral" size="sm">DRAFT</Badge>;
      case "withdrawn":
        return <Badge variant="danger" size="sm">WITHDRAWN</Badge>;
      case "rejected":
        return <Badge variant="danger" size="sm">REJECTED</Badge>;
      default:
        return <Badge variant="neutral" size="sm">{status.toUpperCase()}</Badge>;
    }
  };

  if (isLoading || !user) return <Spinner text="Loading administration workspace..." />;

  const isCreator = selectedSnapshot?.created_by_user_id === user.id;
  const isSuperAdmin = user.primary_role === "super_admin";

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Workspace Header */}
      <div className="bg-emerald-950 text-white rounded-3xl p-6 sm:p-8 shadow-sm border border-emerald-800 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-xs uppercase font-bold tracking-wider text-emerald-400">
              Administrative Governance
            </span>
            <Badge variant="primary" size="sm">PHASE D</Badge>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold flex items-center gap-2">
            <BarChart3 className="w-6 h-6 text-emerald-400" />
            <span>Public Reports & Transparency Management</span>
          </h1>
          <p className="text-xs text-emerald-200">
            Generate, review, approve, publish, and audit certified transparency disclosures across Nigeria.
          </p>
        </div>

        <button
          onClick={() => {
            setActionError(null);
            setShowGenModal(true);
          }}
          className="px-4 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold flex items-center gap-1.5 shadow-sm transition"
        >
          <Plus className="w-4 h-4" />
          <span>Generate Report Snapshot</span>
        </button>
      </div>

      {/* Tabs */}
      <div className="flex flex-wrap gap-2 border-b border-gray-200 pb-2">
        <button
          onClick={() => setActiveTab("all")}
          className={`px-4 py-2 rounded-lg text-xs font-bold transition ${
            activeTab === "all"
              ? "bg-emerald-800 text-white"
              : "bg-gray-100 text-gray-700 hover:bg-gray-200"
          }`}
        >
          All Snapshots
        </button>
        <button
          onClick={() => setActiveTab("pending")}
          className={`px-4 py-2 rounded-lg text-xs font-bold transition ${
            activeTab === "pending"
              ? "bg-amber-800 text-white"
              : "bg-gray-100 text-gray-700 hover:bg-gray-200"
          }`}
        >
          Pending Review Queue
        </button>
        <button
          onClick={() => setActiveTab("published")}
          className={`px-4 py-2 rounded-lg text-xs font-bold transition ${
            activeTab === "published"
              ? "bg-emerald-700 text-white"
              : "bg-gray-100 text-gray-700 hover:bg-gray-200"
          }`}
        >
          Published Live
        </button>
        <button
          onClick={() => setActiveTab("withdrawn")}
          className={`px-4 py-2 rounded-lg text-xs font-bold transition ${
            activeTab === "withdrawn"
              ? "bg-rose-800 text-white"
              : "bg-gray-100 text-gray-700 hover:bg-gray-200"
          }`}
        >
          Withdrawn & Revisions
        </button>
        <button
          onClick={() => setActiveTab("audit")}
          className={`px-4 py-2 rounded-lg text-xs font-bold transition ${
            activeTab === "audit"
              ? "bg-gray-800 text-white"
              : "bg-gray-100 text-gray-700 hover:bg-gray-200"
          }`}
        >
          Governance Audit Trail
        </button>
      </div>

      {/* Tab Content */}
      {loading ? (
        <Spinner text="Loading reporting records..." />
      ) : activeTab === "audit" ? (
        <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm p-6 space-y-4">
          <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
            <History className="w-4 h-4 text-emerald-800" />
            <span>Immutable Governance Audit History</span>
          </h2>

          {audits.length === 0 ? (
            <div className="p-8 text-center text-xs text-gray-500">No audit records logged yet.</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                  <tr>
                    <th className="py-3 px-4">Timestamp</th>
                    <th className="py-3 px-4">Action</th>
                    <th className="py-3 px-4">Actor</th>
                    <th className="py-3 px-4">IP Address</th>
                    <th className="py-3 px-4">Details</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {audits.map((a) => (
                    <tr key={a.id} className="hover:bg-gray-50/50 transition">
                      <td className="py-3 px-4 text-gray-500 font-mono">
                        {new Date(a.created_at).toLocaleString("en-GB")}
                      </td>
                      <td className="py-3 px-4 font-bold text-emerald-950">
                        {a.action.replace(/_/g, " ").toUpperCase()}
                      </td>
                      <td className="py-3 px-4">
                        <span className="font-semibold text-gray-900">{a.actor_name || "System"}</span>
                        {a.actor_email && (
                          <div className="text-[10px] text-gray-400">{a.actor_email}</div>
                        )}
                      </td>
                      <td className="py-3 px-4 font-mono text-gray-500">{a.ip_address || "Internal"}</td>
                      <td className="py-3 px-4 font-mono text-[10px] text-gray-600 max-w-xs truncate">
                        {JSON.stringify(a.details)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : (
        <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm p-6 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-bold text-gray-900">
              Report Snapshots ({totalCount} Total)
            </h2>
          </div>

          {snapshots.length === 0 ? (
            <EmptyState
              icon={<FileText className="w-6 h-6" />}
              title="No Snapshots Found"
              description="Click 'Generate Report Snapshot' above to create a certified report draft from live verified database records."
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-gray-50 text-gray-600 font-semibold uppercase tracking-wider border-b border-gray-200">
                  <tr>
                    <th className="py-3 px-4">Reference</th>
                    <th className="py-3 px-4">Title</th>
                    <th className="py-3 px-4">Type</th>
                    <th className="py-3 px-4">Scope</th>
                    <th className="py-3 px-4">Period</th>
                    <th className="py-3 px-4">Status</th>
                    <th className="py-3 px-4">Created By</th>
                    <th className="py-3 px-4 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {snapshots.map((snap) => (
                    <tr key={snap.snapshot_id} className="hover:bg-gray-50/50 transition">
                      <td className="py-3.5 px-4 font-mono font-bold text-emerald-950">
                        {snap.public_reference}
                      </td>
                      <td className="py-3.5 px-4 font-bold text-gray-900">{snap.title}</td>
                      <td className="py-3.5 px-4">
                        <span className="capitalize">{snap.report_type.replace(/_/g, " ")}</span>
                      </td>
                      <td className="py-3.5 px-4 text-gray-600">
                        {snap.state_name ? `${snap.state_name} State` : "Nationwide"}
                      </td>
                      <td className="py-3.5 px-4 font-medium text-gray-700">{snap.reporting_period}</td>
                      <td className="py-3.5 px-4">{getStatusBadge(snap.status)}</td>
                      <td className="py-3.5 px-4 text-gray-600">{snap.creator_name}</td>
                      <td className="py-3.5 px-4 text-right">
                        <button
                          onClick={() => {
                            setActionError(null);
                            setActionSuccess(null);
                            setActionReason("");
                            setSelectedSnapshot(snap);
                          }}
                          className="px-2.5 py-1.5 rounded-lg bg-emerald-50 hover:bg-emerald-100 text-emerald-800 text-[11px] font-bold border border-emerald-200 transition"
                        >
                          Manage & Inspect
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Snapshot Generation Modal */}
      {showGenModal && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-xl border border-gray-200">
            <div className="flex items-center justify-between border-b pb-3">
              <h3 className="font-bold text-gray-900 text-base flex items-center gap-2">
                <BarChart3 className="w-5 h-5 text-emerald-800" />
                <span>Generate Verified Report Snapshot</span>
              </h3>
              <button
                onClick={() => setShowGenModal(false)}
                className="text-gray-400 hover:text-gray-600 text-lg font-bold"
              >
                &times;
              </button>
            </div>

            {actionError && <Alert type="error">{actionError}</Alert>}

            <form onSubmit={handleGenerate} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Report Type</label>
                <select
                  value={genReportType}
                  onChange={(e) => setGenReportType(e.target.value)}
                  className="w-full rounded-lg border-gray-300 py-2 text-xs"
                >
                  <option value="cleanups">Environmental Cleanups & Impact</option>
                  <option value="participation">Verified Civic Participation</option>
                  <option value="finance">Financial Accountability & Ledger Disclosure</option>
                  <option value="foundation">Martinsvincent Ortse Foundation Charitable Report</option>
                  <option value="community_projects">Community Development Projects</option>
                  <option value="national_overview">National Overview Performance Dashboard</option>
                  <option value="annual_impact">Certified Annual Comprehensive Report</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Report Title</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Q4 2026 Nationwide Environmental Cleanup Impact"
                  value={genTitle}
                  onChange={(e) => setGenTitle(e.target.value)}
                  className="w-full rounded-lg border-gray-300 py-2 text-xs"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Reporting Period</label>
                  <select
                    value={genPeriod}
                    onChange={(e) => setGenPeriod(e.target.value)}
                    className="w-full rounded-lg border-gray-300 py-2 text-xs"
                  >
                    <option value="2026">2026 Annual</option>
                    <option value="2026-10">October 2026</option>
                    <option value="2026-09">September 2026</option>
                    <option value="2026-08">August 2026</option>
                    <option value="2025">2025 Annual</option>
                    <option value="All Time">All Time Baseline</option>
                  </select>
                </div>

                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Geographic Scope</label>
                  <select
                    value={genScopeType}
                    onChange={(e) => setGenScopeType(e.target.value)}
                    className="w-full rounded-lg border-gray-300 py-2 text-xs"
                  >
                    <option value="national">Nigeria Nationwide</option>
                    <option value="zone">Geopolitical Zone</option>
                    <option value="state">Specific State</option>
                    <option value="lga" disabled={genReportType === "foundation"}>Local Government Area (LGA)</option>
                  </select>
                </div>
              </div>

              {genReportType === "foundation" && genScopeType === "lga" && (
                <div className="bg-amber-50 p-3 rounded-xl border border-amber-200 text-amber-900 text-[11px] leading-relaxed flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 text-amber-600 flex-shrink-0" />
                  <span>Martinsvincent Ortse Foundation reports operate at National, Zonal, or State levels. LGA scope is not supported for Foundation donations.</span>
                </div>
              )}

              {genScopeType === "zone" && (
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Target Geopolitical Zone</label>
                  <select
                    required
                    value={genZoneID}
                    onChange={(e) => setGenZoneID(e.target.value)}
                    className="w-full rounded-lg border-gray-300 py-2 text-xs"
                  >
                    <option value="">Select Zone</option>
                    {zones.map((z) => (
                      <option key={z.id} value={z.id}>
                        {z.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {genScopeType === "state" && (
                <div>
                  <label className="block font-semibold text-gray-700 mb-1">Target State</label>
                  <select
                    required
                    value={genStateID}
                    onChange={(e) => setGenStateID(e.target.value)}
                    className="w-full rounded-lg border-gray-300 py-2 text-xs"
                  >
                    <option value="">Select State</option>
                    {states.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {genScopeType === "lga" && (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block font-semibold text-gray-700 mb-1">State</label>
                    <select
                      required
                      value={genStateID}
                      onChange={(e) => {
                        setGenStateID(e.target.value);
                        setGenLgaID("");
                      }}
                      className="w-full rounded-lg border-gray-300 py-2 text-xs"
                    >
                      <option value="">Select State</option>
                      {states.map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block font-semibold text-gray-700 mb-1">Local Government Area (LGA)</label>
                    <select
                      required
                      disabled={!genStateID || lgas.length === 0}
                      value={genLgaID}
                      onChange={(e) => setGenLgaID(e.target.value)}
                      className="w-full rounded-lg border-gray-300 py-2 text-xs disabled:opacity-50"
                    >
                      <option value="">{genStateID ? "Select LGA" : "Select State first"}</option>
                      {lgas.map((l) => (
                        <option key={l.id} value={l.id}>
                          {l.name}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
              )}

              <div className="bg-emerald-50 p-3 rounded-xl border border-emerald-200 text-emerald-950 text-[11px] leading-relaxed">
                Snapshot generation computes deterministic SQL aggregates from verified records and freezes them
                with an immutable SHA-256 data hash. The draft remains unlisted publicly until certified and published.
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowGenModal(false)}
                  className="px-4 py-2 border rounded-lg text-gray-700 font-semibold hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isGenerating || (genReportType === "foundation" && genScopeType === "lga")}
                  className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg flex items-center gap-1.5 disabled:opacity-50"
                >
                  {isGenerating ? <Spinner size="sm" /> : <BarChart3 className="w-3.5 h-3.5" />}
                  <span>Generate Snapshot</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Snapshot Management Modal / Detail Drawer */}
      {selectedSnapshot && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-2xl w-full p-6 space-y-5 shadow-2xl border border-gray-200 max-h-[90vh] overflow-y-auto">
            <div className="flex items-start justify-between border-b pb-4">
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-mono font-bold text-sm text-emerald-950">
                    {selectedSnapshot.public_reference}
                  </span>
                  {getStatusBadge(selectedSnapshot.status)}
                  <span className="text-[10px] font-bold text-gray-500 bg-gray-100 px-2 py-0.5 rounded">
                    v{selectedSnapshot.publication_version}
                  </span>
                </div>
                <h3 className="font-bold text-gray-900 text-base mt-1">
                  {selectedSnapshot.title}
                </h3>
              </div>
              <button
                onClick={() => setSelectedSnapshot(null)}
                className="text-gray-400 hover:text-gray-600 text-xl font-bold"
              >
                &times;
              </button>
            </div>

            {actionError && <Alert type="error">{actionError}</Alert>}
            {actionSuccess && <Alert type="success">{actionSuccess}</Alert>}

            {/* Metadata Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-gray-50 p-3.5 rounded-xl text-xs">
              <div>
                <span className="text-gray-400 font-medium">Report Type:</span>
                <div className="font-bold text-gray-900 capitalize">
                  {selectedSnapshot.report_type.replace(/_/g, " ")}
                </div>
              </div>
              <div>
                <span className="text-gray-400 font-medium">Scope:</span>
                <div className="font-bold text-gray-900">
                  {selectedSnapshot.state_name ? `${selectedSnapshot.state_name} State` : "Nationwide"}
                </div>
              </div>
              <div>
                <span className="text-gray-400 font-medium">Period:</span>
                <div className="font-bold text-gray-900">{selectedSnapshot.reporting_period}</div>
              </div>
              <div>
                <span className="text-gray-400 font-medium">Prepared By:</span>
                <div className="font-bold text-gray-900 truncate">{selectedSnapshot.creator_name}</div>
              </div>
            </div>

            {/* Cryptographic Hash */}
            <div className="p-2.5 bg-gray-100 rounded-lg text-[10px] font-mono text-gray-600 break-all">
              <span className="font-bold text-gray-800">SHA-256 Hash: </span>
              {selectedSnapshot.data_hash}
            </div>

            {/* Aggregated Data Preview */}
            <div className="space-y-1.5">
              <span className="text-xs font-bold text-gray-700">Frozen Aggregated Content Preview:</span>
              <pre className="p-3 bg-gray-900 text-emerald-400 rounded-xl text-[11px] font-mono overflow-x-auto max-h-48">
                {JSON.stringify(selectedSnapshot.aggregated_data, null, 2)}
              </pre>
            </div>

            {/* Approvals History */}
            {selectedSnapshot.approvals && selectedSnapshot.approvals.length > 0 && (
              <div className="space-y-2">
                <span className="text-xs font-bold text-gray-700">Governance Review History:</span>
                <div className="divide-y divide-gray-100 border border-gray-200 rounded-xl overflow-hidden text-xs">
                  {selectedSnapshot.approvals.map((ap) => (
                    <div key={ap.id} className="p-3 flex items-center justify-between bg-gray-50/50">
                      <div>
                        <span className="font-bold text-gray-900">{ap.reviewer_name}</span>
                        <p className="text-[11px] text-gray-500 italic mt-0.5">&ldquo;{ap.decision_reason}&rdquo;</p>
                      </div>
                      <div className="text-right">
                        <Badge variant={ap.decision === "approved" ? "success" : "danger"} size="sm">
                          {ap.decision.toUpperCase()}
                        </Badge>
                        <div className="text-[10px] text-gray-400 mt-1">
                          {new Date(ap.created_at).toLocaleDateString()}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Action Form based on Status */}
            <div className="border-t pt-4 space-y-3">
              <span className="text-xs font-bold text-gray-900 block">Governance Workflow Actions:</span>

              {/* Status: DRAFT */}
              {selectedSnapshot.status === "draft" && (
                <div className="flex gap-2">
                  <button
                    onClick={() => handleAction("submit-review")}
                    disabled={isActionPending}
                    className="w-full py-2.5 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-xl text-xs flex items-center justify-center gap-1.5 disabled:opacity-50"
                  >
                    <Send className="w-4 h-4" />
                    <span>Submit for Governance Review</span>
                  </button>
                </div>
              )}

              {/* Status: PENDING REVIEW */}
              {selectedSnapshot.status === "pending_review" && (
                <div className="space-y-3">
                  {isCreator && (
                    <Alert type="warning" title="Separation of Duties (Anti-Self-Approval)">
                      You prepared this report snapshot. An independent coordinator or Super Admin must review and approve it.
                    </Alert>
                  )}
                  {(selectedSnapshot.report_type === "finance" || selectedSnapshot.report_type === "annual_impact") && !isSuperAdmin && (
                    <Alert type="warning" title="Financial Governance Authority">
                      Public disclosure of financial statements requires Super Admin certification.
                    </Alert>
                  )}

                  <div>
                    <label className="block text-[11px] font-semibold text-gray-700 mb-1">
                      Decision Reason / Inspection Notes:
                    </label>
                    <textarea
                      rows={2}
                      value={actionReason}
                      onChange={(e) => setActionReason(e.target.value)}
                      placeholder="e.g. Verified against state cleanup receipts and bank reconciliations."
                      className="w-full rounded-lg border-gray-300 text-xs p-2"
                    />
                  </div>

                  <div className="flex gap-2">
                    <button
                      onClick={() => handleAction("approve")}
                      disabled={isActionPending || isCreator}
                      className="flex-1 py-2.5 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-xl text-xs flex items-center justify-center gap-1.5 disabled:opacity-50"
                    >
                      <CheckCircle2 className="w-4 h-4" />
                      <span>Approve Snapshot</span>
                    </button>
                    <button
                      onClick={() => handleAction("reject")}
                      disabled={isActionPending}
                      className="flex-1 py-2.5 bg-rose-700 hover:bg-rose-800 text-white font-bold rounded-xl text-xs flex items-center justify-center gap-1.5 disabled:opacity-50"
                    >
                      <XCircle className="w-4 h-4" />
                      <span>Reject Snapshot</span>
                    </button>
                  </div>
                </div>
              )}

              {/* Status: APPROVED */}
              {selectedSnapshot.status === "approved" && (
                <div className="space-y-3">
                  <p className="text-xs text-emerald-900 font-medium">
                    This snapshot has received governance approval. Publishing will make it visible on public dashboards
                    and activate PDF and CSV downloads.
                  </p>
                  <button
                    onClick={() => handleAction("publish")}
                    disabled={isActionPending}
                    className="w-full py-2.5 bg-emerald-700 hover:bg-emerald-800 text-white font-bold rounded-xl text-xs flex items-center justify-center gap-1.5 disabled:opacity-50"
                  >
                    <ShieldCheck className="w-4 h-4" />
                    <span>Publish Report Snapshot to Public Portal</span>
                  </button>
                </div>
              )}

              {/* Status: PUBLISHED */}
              {selectedSnapshot.status === "published" && (
                <div className="space-y-3">
                  <div className="flex gap-2">
                    <button
                      onClick={() => window.open(`/api/v1/public/reports/publications/${selectedSnapshot.public_reference}/pdf`, "_blank")}
                      className="flex-1 py-2 bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-bold rounded-xl text-xs border border-emerald-200 flex items-center justify-center gap-1"
                    >
                      <Download className="w-3.5 h-3.5" />
                      <span>Certified PDF</span>
                    </button>
                    <button
                      onClick={() => window.open(`/api/v1/public/reports/publications/${selectedSnapshot.public_reference}/csv`, "_blank")}
                      className="flex-1 py-2 bg-gray-50 hover:bg-gray-100 text-gray-700 font-bold rounded-xl text-xs border border-gray-200 flex items-center justify-center gap-1"
                    >
                      <Download className="w-3.5 h-3.5" />
                      <span>RFC 4180 CSV</span>
                    </button>
                  </div>

                  <div className="pt-2 border-t space-y-2">
                    <label className="block text-[11px] font-semibold text-rose-800">
                      Withdraw Publication (Notice of Correction Required):
                    </label>
                    <input
                      type="text"
                      value={actionReason}
                      onChange={(e) => setActionReason(e.target.value)}
                      placeholder="e.g. Audit revision required for Benue cleanup records."
                      className="w-full rounded-lg border-gray-300 text-xs p-2"
                    />
                    <button
                      onClick={() => handleAction("withdraw")}
                      disabled={isActionPending || !actionReason}
                      className="w-full py-2 bg-rose-600 hover:bg-rose-700 text-white font-bold rounded-xl text-xs flex items-center justify-center gap-1.5 disabled:opacity-50"
                    >
                      <AlertTriangle className="w-3.5 h-3.5" />
                      <span>Withdraw from Public Portal</span>
                    </button>
                  </div>
                </div>
              )}

              {/* Status: WITHDRAWN */}
              {selectedSnapshot.status === "withdrawn" && (
                <div className="space-y-3">
                  <label className="block text-[11px] font-semibold text-gray-700">
                    Correction Notes for Revision {selectedSnapshot.publication_version + 1}:
                  </label>
                  <input
                    type="text"
                    value={actionReason}
                    onChange={(e) => setActionReason(e.target.value)}
                    placeholder="e.g. Reconciled with updated bank statements and verified dump slips."
                    className="w-full rounded-lg border-gray-300 text-xs p-2"
                  />
                  <button
                    onClick={() => handleAction("revise")}
                    disabled={isActionPending || !actionReason}
                    className="w-full py-2.5 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-xl text-xs flex items-center justify-center gap-1.5 disabled:opacity-50"
                  >
                    <RefreshCw className="w-4 h-4" />
                    <span>Generate Corrected Version {selectedSnapshot.publication_version + 1}</span>
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
