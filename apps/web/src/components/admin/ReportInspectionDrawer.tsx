"use client";

import React, { useState, useEffect } from "react";
import {
  X,
  Copy,
  Check,
  Shield,
  ShieldAlert,
  ShieldCheck,
  FileText,
  FileSpreadsheet,
  Download,
  RefreshCw,
  Clock,
  Calendar,
  User,
  MapPin,
  Layers,
  ChevronDown,
  ChevronUp,
  AlertTriangle,
  Info,
  CheckCircle2,
  XCircle,
  Building,
  Coins,
  TrendingUp,
  Award,
  Trash2,
  FolderTree,
  Send,
  Eye,
  FileCode,
} from "lucide-react";
import {
  AdminSnapshotDetailDTO,
  AdminReportExportDTO,
  ReportApprovalItem,
  ReportAuditItem,
  User as AuthUser,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { Spinner } from "@/components/ui/EmptyState";
import { apiRequest } from "@/lib/api";

interface ReportInspectionDrawerProps {
  snapshotId: string;
  currentUser: AuthUser | null;
  isSuperAdmin: boolean;
  isNationalAdmin: boolean;
  onClose: () => void;
  onGovernanceActionExecuted?: () => void;
}

export function ReportInspectionDrawer({
  snapshotId,
  currentUser,
  isSuperAdmin,
  isNationalAdmin,
  onClose,
  onGovernanceActionExecuted,
}: ReportInspectionDrawerProps) {
  const [snapshot, setSnapshot] = useState<AdminSnapshotDetailDTO | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Active sub-tab in drawer
  const [activeTab, setActiveTab] = useState<"preview" | "details" | "integrity" | "governance" | "audit" | "exports">("preview");

  // Raw JSON collapse state
  const [showRawJson, setShowRawJson] = useState<boolean>(false);
  const [copiedHash, setCopiedHash] = useState<boolean>(false);
  const [copiedJson, setCopiedJson] = useState<boolean>(false);

  // Governance action form states
  const [governanceAction, setGovernanceAction] = useState<string | null>(null);
  const [governanceReason, setGovernanceReason] = useState<string>("");
  const [isSubmittingAction, setIsSubmittingAction] = useState<boolean>(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  // Export regeneration states
  const [regeneratingFormat, setRegeneratingFormat] = useState<"pdf" | "csv" | null>(null);

  // Fetch complete snapshot details
  const fetchSnapshotDetail = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiRequest<AdminSnapshotDetailDTO>(`/admin/reports/${snapshotId}`);
      setSnapshot(data);
    } catch (err: any) {
      setError(err.message || "Failed to retrieve snapshot contents");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (snapshotId) {
      fetchSnapshotDetail();
    }
  }, [snapshotId]);

  // Handle ESC key to close
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !governanceAction) {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, governanceAction]);

  // Copy hash helper
  const handleCopyHash = () => {
    if (!snapshot?.data_hash) return;
    navigator.clipboard.writeText(snapshot.data_hash);
    setCopiedHash(true);
    setTimeout(() => setCopiedHash(false), 2000);
  };

  // Copy raw JSON helper
  const handleCopyJson = () => {
    if (!snapshot?.aggregated_data) return;
    navigator.clipboard.writeText(JSON.stringify(snapshot.aggregated_data, null, 2));
    setCopiedJson(true);
    setTimeout(() => setCopiedJson(false), 2000);
  };

  // Date formatter
  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "N/A";
    try {
      return new Date(dateStr).toLocaleString("en-GB", {
        day: "2-digit",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      });
    } catch {
      return dateStr;
    }
  };

  // Status badge
  const getStatusBadge = (status: string) => {
    switch (status) {
      case "draft":
        return <Badge variant="neutral">Draft</Badge>;
      case "pending_review":
        return <Badge variant="warning">Pending Review</Badge>;
      case "approved":
        return <Badge variant="primary">Approved</Badge>;
      case "published":
        return <Badge variant="success">Published</Badge>;
      case "rejected":
        return <Badge variant="danger">Rejected</Badge>;
      case "withdrawn":
        return <Badge variant="danger">Withdrawn</Badge>;
      default:
        return <Badge variant="neutral">{status}</Badge>;
    }
  };

  // Check anti-self-approval rule
  const isCreator = Boolean(
    currentUser && snapshot?.created_by_user_id && snapshot.created_by_user_id === currentUser.id
  );

  // Financial formatter (Kobo to Naira)
  const formatKobo = (kobo?: number | null) => {
    if (kobo === undefined || kobo === null) return "₦0.00";
    const naira = kobo / 100;
    return "₦" + naira.toLocaleString("en-NG", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  };

  // Export Download
  const handleDownload = (format: "pdf" | "csv") => {
    if (!snapshot) return;
    const downloadUrl = `/api/v1/admin/reports/${snapshot.snapshot_id}/exports/${format}/download`;
    window.open(downloadUrl, "_blank");
  };

  // Export Regenerate
  const handleRegenerate = async (format: "pdf" | "csv") => {
    if (!snapshot) return;
    if (
      !confirm(
        `Are you sure you want to regenerate the ${format.toUpperCase()} export for "${snapshot.title}"? This will recompute the export document and store it in private Cloudflare R2.`
      )
    ) {
      return;
    }

    setRegeneratingFormat(format);
    try {
      await apiRequest(`/admin/reports/${snapshot.snapshot_id}/exports/${format}/regenerate`, {
        method: "POST",
      });
      await fetchSnapshotDetail();
    } catch (err: any) {
      alert(`Regeneration failed: ${err.message || "Unknown error"}`);
    } finally {
      setRegeneratingFormat(null);
    }
  };

  // Execute Governance Action
  const handleExecuteGovernance = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!snapshot || !governanceAction) return;

    setIsSubmittingAction(true);
    setActionError(null);
    setActionSuccess(null);

    let endpoint = `/admin/reports/${snapshot.snapshot_id}/${governanceAction}`;
    if (governanceAction === "submit_review") {
      endpoint = `/admin/reports/${snapshot.snapshot_id}/submit-review`;
    }

    try {
      await apiRequest(endpoint, {
        method: "POST",
        body: JSON.stringify({
          decision_reason: governanceReason,
          reason: governanceReason,
        }),
      });

      setActionSuccess(`Governance action "${governanceAction.replace("_", " ").toUpperCase()}" executed successfully.`);
      setTimeout(async () => {
        setGovernanceAction(null);
        setGovernanceReason("");
        setActionSuccess(null);
        await fetchSnapshotDetail();
        if (onGovernanceActionExecuted) {
          onGovernanceActionExecuted();
        }
      }, 1200);
    } catch (err: any) {
      setActionError(err.message || "Governance action failed.");
    } finally {
      setIsSubmittingAction(false);
    }
  };

  // Render Category-Specific Aggregated Data
  const renderReportContent = () => {
    if (!snapshot || !snapshot.aggregated_data) {
      return (
        <div className="p-8 text-center bg-gray-50 rounded-xl border border-dashed border-gray-300">
          <Info className="w-8 h-8 text-gray-400 mx-auto mb-2" />
          <p className="text-sm font-semibold text-gray-700">No Aggregated Source Data Available</p>
          <p className="text-xs text-gray-500 mt-1">This snapshot contains no recorded dataset.</p>
        </div>
      );
    }

    const data = snapshot.aggregated_data;

    switch (snapshot.report_type) {
      case "cleanups":
        return (
          <div className="space-y-6">
            {/* Metric Summary Cards */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Cleanups Completed</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.completed_cleanups_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Total Waste Collected</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.total_waste_collected_kg ?? 0).toLocaleString()} <span className="text-xs font-normal">kg</span>
                </p>
                <p className="text-[11px] text-emerald-700 mt-0.5">
                  {(data.total_waste_bags ?? 0).toLocaleString()} bags
                </p>
              </div>
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Verified Attendees</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.verified_attendees_count ?? 0).toLocaleString()}
                </p>
                <p className="text-[11px] text-emerald-700 mt-0.5">
                  {(data.unique_participants_count ?? 0).toLocaleString()} unique
                </p>
              </div>
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Geographic Spread</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {data.active_states_count ?? 0} <span className="text-xs font-normal">States</span>
                </p>
                <p className="text-[11px] text-emerald-700 mt-0.5">
                  {data.active_lgas_count ?? 0} LGAs
                </p>
              </div>
            </div>

            {/* State Breakdown Table */}
            <div>
              <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-2.5">
                State Environmental Breakdown
              </h4>
              {Array.isArray(data.state_breakdown) && data.state_breakdown.length > 0 ? (
                <div className="overflow-x-auto border border-gray-200 rounded-xl">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-gray-50 border-b border-gray-200 text-gray-600 font-semibold">
                      <tr>
                        <th className="py-2.5 px-3">State</th>
                        <th className="py-2.5 px-3 text-right">Completed Cleanups</th>
                        <th className="py-2.5 px-3 text-right">Waste Collected (kg)</th>
                        <th className="py-2.5 px-3 text-right">Verified Attendees</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 text-gray-800">
                      {data.state_breakdown.map((row: any, idx: number) => (
                        <tr key={idx} className="hover:bg-gray-50/50">
                          <td className="py-2.5 px-3 font-medium text-gray-900">{row.state_name || "Unknown"}</td>
                          <td className="py-2.5 px-3 text-right font-mono font-semibold">
                            {(row.cleanups_count ?? 0).toLocaleString()}
                          </td>
                          <td className="py-2.5 px-3 text-right font-mono">
                            {(row.waste_collected_kg ?? 0).toLocaleString()}
                          </td>
                          <td className="py-2.5 px-3 text-right font-mono">
                            {(row.verified_attendees ?? 0).toLocaleString()}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="p-4 bg-gray-50 border border-gray-200 rounded-xl text-center text-xs text-gray-500">
                  No individual state breakdown recorded for this period.
                </div>
              )}
            </div>
          </div>
        );

      case "participation":
        return (
          <div className="space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div className="p-3.5 bg-blue-50/60 rounded-xl border border-blue-100">
                <span className="text-xs font-medium text-blue-800">Claims Submitted</span>
                <p className="text-xl font-bold text-blue-950 mt-1">
                  {(data.total_claims_submitted ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Approved Claims</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.approved_claims_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-rose-50/60 rounded-xl border border-rose-100">
                <span className="text-xs font-medium text-rose-800">Rejected Claims</span>
                <p className="text-xl font-bold text-rose-950 mt-1">
                  {(data.rejected_claims_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-purple-50/60 rounded-xl border border-purple-100">
                <span className="text-xs font-medium text-purple-800">Approval Rate</span>
                <p className="text-xl font-bold text-purple-950 mt-1">
                  {typeof data.approval_rate_percent === "number"
                    ? `${data.approval_rate_percent.toFixed(1)}%`
                    : "0.0%"}
                </p>
              </div>
            </div>

            {/* Privacy protected section */}
            <div className="p-4 bg-gray-50 border border-gray-200 rounded-xl space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-gray-700">Unique Verified Members:</span>
                <span className="font-mono text-sm font-bold text-gray-900">
                  {data.unique_members_suppressed ? (
                    <span className="text-amber-700 text-xs font-semibold">
                      {data.unique_members_suppressed}
                    </span>
                  ) : (
                    (data.unique_verified_members ?? 0).toLocaleString()
                  )}
                </span>
              </div>
              {data.unique_members_suppressed && (
                <p className="text-[11px] text-amber-800 bg-amber-50 p-2 rounded border border-amber-200">
                  Civic Privacy Protection: Small member cohorts below 5 individuals are suppressed to preserve participant anonymity under Nigerian Data Protection Regulations.
                </p>
              )}
            </div>
          </div>
        );

      case "finance":
        return (
          <div className="space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Verified Income</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {data.approved_income_formatted || formatKobo(data.approved_income_kobo)}
                </p>
              </div>
              <div className="p-3.5 bg-rose-50/60 rounded-xl border border-rose-100">
                <span className="text-xs font-medium text-rose-800">Verified Expenditure</span>
                <p className="text-xl font-bold text-rose-950 mt-1">
                  {data.approved_expenditure_formatted || formatKobo(data.approved_expenditure_kobo)}
                </p>
              </div>
              <div
                className={`p-3.5 rounded-xl border ${
                  (data.net_surplus_deficit_kobo ?? 0) >= 0
                    ? "bg-emerald-50/60 border-emerald-100 text-emerald-950"
                    : "bg-rose-50/60 border-rose-100 text-rose-950"
                }`}
              >
                <span className="text-xs font-medium">Net Operating Balance</span>
                <p className="text-xl font-bold mt-1">
                  {data.net_surplus_deficit_formatted || formatKobo(data.net_surplus_deficit_kobo)}
                </p>
              </div>
              <div className="p-3.5 bg-gray-50 rounded-xl border border-gray-200">
                <span className="text-xs font-medium text-gray-600">Approved Budget</span>
                <p className="text-lg font-bold text-gray-900 mt-1">
                  {data.total_approved_budget_formatted || formatKobo(data.total_approved_budget_kobo)}
                </p>
              </div>
              <div className="p-3.5 bg-gray-50 rounded-xl border border-gray-200">
                <span className="text-xs font-medium text-gray-600">Budget Variance</span>
                <p className="text-lg font-bold text-gray-900 mt-1">
                  {data.budget_variance_formatted || formatKobo(data.budget_variance_kobo)}
                </p>
              </div>
            </div>

            {/* Spending Categories */}
            <div>
              <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-2.5">
                Expense Category Allocations
              </h4>
              {Array.isArray(data.spending_categories) && data.spending_categories.length > 0 ? (
                <div className="overflow-x-auto border border-gray-200 rounded-xl">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-gray-50 border-b border-gray-200 text-gray-600 font-semibold">
                      <tr>
                        <th className="py-2.5 px-3">Expense Category</th>
                        <th className="py-2.5 px-3 text-right">Transaction Vouchers</th>
                        <th className="py-2.5 px-3 text-right">Total Expenditure</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 text-gray-800">
                      {data.spending_categories.map((cat: any, idx: number) => (
                        <tr key={idx} className="hover:bg-gray-50/50">
                          <td className="py-2.5 px-3 font-medium text-gray-900">
                            {cat.category_label || cat.category || "General"}
                          </td>
                          <td className="py-2.5 px-3 text-right font-mono font-semibold">
                            {(cat.expense_count ?? 0).toLocaleString()}
                          </td>
                          <td className="py-2.5 px-3 text-right font-mono font-bold text-gray-900">
                            {cat.amount_formatted || formatKobo(cat.total_kobo)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="p-4 bg-gray-50 border border-gray-200 rounded-xl text-center text-xs text-gray-500">
                  No verified expenditures recorded in this period.
                </div>
              )}
            </div>
          </div>
        );

      case "foundation":
        return (
          <div className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="p-4 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Verified Humanitarian Donations</span>
                <p className="text-2xl font-bold text-emerald-950 mt-1">
                  {data.total_donations_formatted || formatKobo(data.total_donations_kobo)}
                </p>
                <p className="text-xs text-emerald-700 mt-1">
                  {(data.verified_donations_count ?? 0).toLocaleString()} completed transactions
                </p>
              </div>
              <div className="p-4 bg-blue-50/60 rounded-xl border border-blue-100">
                <span className="text-xs font-medium text-blue-800">Financial Safety Switch</span>
                <p className="text-base font-bold text-blue-950 mt-1 flex items-center gap-2">
                  <ShieldCheck className="w-5 h-5 text-blue-700" />
                  Dual-Signatory Protection Active
                </p>
                <p className="text-xs text-blue-700 mt-1">
                  Dedicated charitable bank accounts strictly segregated from operational funds.
                </p>
              </div>
            </div>

            {/* Governance disclosures */}
            <div className="space-y-3">
              <div className="p-3.5 bg-amber-50/80 border border-amber-200 rounded-xl text-xs text-amber-900 space-y-1">
                <span className="font-bold flex items-center gap-1.5">
                  <AlertTriangle className="w-4 h-4 text-amber-700" />
                  Statutory Registration Notice
                </span>
                <p>{data.provisional_entity_notice || "Martinsvincent Ortse Foundation registration is provisional pending final CAC review."}</p>
              </div>
              <div className="p-3.5 bg-gray-50 border border-gray-200 rounded-xl text-xs text-gray-800 space-y-1">
                <span className="font-bold flex items-center gap-1.5">
                  <Shield className="w-4 h-4 text-gray-700" />
                  Asset Segregation Guarantee
                </span>
                <p>{data.strict_segregation_notice || "Foundation humanitarian funds are strictly separated from Ratels Worldwide operational funds in double-entry ledgers."}</p>
              </div>
            </div>
          </div>
        );

      case "community_projects":
        return (
          <div className="space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Total Projects</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.total_projects_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-blue-50/60 rounded-xl border border-blue-100">
                <span className="text-xs font-medium text-blue-800">Completed (100%)</span>
                <p className="text-xl font-bold text-blue-950 mt-1">
                  {(data.completed_projects_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-amber-50/60 rounded-xl border border-amber-100">
                <span className="text-xs font-medium text-amber-800">In Progress</span>
                <p className="text-xl font-bold text-amber-950 mt-1">
                  {(data.in_progress_projects_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-gray-50 rounded-xl border border-gray-200">
                <span className="text-xs font-medium text-gray-600">Planned / Queued</span>
                <p className="text-xl font-bold text-gray-900 mt-1">
                  {(data.planned_projects_count ?? 0).toLocaleString()}
                </p>
              </div>
            </div>

            {/* Financial indicators for projects */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="p-3 bg-gray-50 rounded-xl border border-gray-200 text-xs">
                <span className="text-gray-500">Allocated Project Budgets:</span>
                <p className="text-base font-bold text-gray-900 mt-0.5">
                  {data.total_approved_budget_formatted || formatKobo(data.total_approved_budget_kobo)}
                </p>
              </div>
              <div className="p-3 bg-gray-50 rounded-xl border border-gray-200 text-xs">
                <span className="text-gray-500">Recorded Project Expenditures:</span>
                <p className="text-base font-bold text-gray-900 mt-0.5">
                  {data.total_recorded_expenditure_formatted || formatKobo(data.total_recorded_expenditure_kobo)}
                </p>
              </div>
            </div>

            {/* Projects Table */}
            <div>
              <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-2.5">
                Community Projects Portfolio
              </h4>
              {Array.isArray(data.projects) && data.projects.length > 0 ? (
                <div className="overflow-x-auto border border-gray-200 rounded-xl">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-gray-50 border-b border-gray-200 text-gray-600 font-semibold">
                      <tr>
                        <th className="py-2.5 px-3">Project Title & Reference</th>
                        <th className="py-2.5 px-3">Location</th>
                        <th className="py-2.5 px-3">Progress</th>
                        <th className="py-2.5 px-3">Status</th>
                        <th className="py-2.5 px-3 text-right">Budget</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 text-gray-800">
                      {data.projects.map((proj: any, idx: number) => (
                        <tr key={idx} className="hover:bg-gray-50/50">
                          <td className="py-2.5 px-3">
                            <span className="font-mono text-[10px] text-emerald-900 font-bold block">
                              {proj.reference}
                            </span>
                            <span className="font-medium text-gray-900">{proj.title}</span>
                          </td>
                          <td className="py-2.5 px-3 text-gray-600">
                            {proj.lga_name ? `${proj.lga_name}, ` : ""}
                            {proj.state_name || "National"}
                          </td>
                          <td className="py-2.5 px-3">
                            <div className="flex items-center gap-2">
                              <div className="w-16 bg-gray-200 rounded-full h-1.5 overflow-hidden">
                                <div
                                  className="bg-emerald-600 h-1.5 rounded-full"
                                  style={{ width: `${Math.min(100, Math.max(0, proj.progress_percentage || 0))}%` }}
                                />
                              </div>
                              <span className="font-mono text-[11px] font-bold">
                                {proj.progress_percentage ?? 0}%
                              </span>
                            </div>
                          </td>
                          <td className="py-2.5 px-3">
                            <span className="capitalize px-2 py-0.5 rounded-full text-[10px] font-medium bg-gray-100 text-gray-700">
                              {(proj.execution_status || proj.funding_status || "Active").replace("_", " ")}
                            </span>
                          </td>
                          <td className="py-2.5 px-3 text-right font-mono font-semibold">
                            {proj.approved_budget_formatted || formatKobo(proj.approved_budget_kobo)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="p-4 bg-gray-50 border border-gray-200 rounded-xl text-center text-xs text-gray-500">
                  No community development projects in this reporting period.
                </div>
              )}
            </div>
          </div>
        );

      case "national_overview":
      case "annual_impact":
      default:
        return (
          <div className="space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Cleanups Completed</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.completed_cleanups_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Verified Attendees</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.verified_attendees_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Total Waste Collected</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.total_waste_collected_kg ?? 0).toLocaleString()} <span className="text-xs font-normal">kg</span>
                </p>
              </div>
              <div className="p-3.5 bg-emerald-50/60 rounded-xl border border-emerald-100">
                <span className="text-xs font-medium text-emerald-800">Approved Projects</span>
                <p className="text-xl font-bold text-emerald-950 mt-1">
                  {(data.approved_projects_count ?? 0).toLocaleString()}
                </p>
              </div>
              <div className="p-3.5 bg-gray-50 rounded-xl border border-gray-200">
                <span className="text-xs font-medium text-gray-600">Approved Income</span>
                <p className="text-lg font-bold text-gray-900 mt-1">
                  {data.approved_income_formatted || formatKobo(data.approved_income_kobo)}
                </p>
              </div>
              <div className="p-3.5 bg-gray-50 rounded-xl border border-gray-200">
                <span className="text-xs font-medium text-gray-600">Approved Expenditure</span>
                <p className="text-lg font-bold text-gray-900 mt-1">
                  {data.approved_expenditure_formatted || formatKobo(data.approved_expenditure_kobo)}
                </p>
              </div>
              <div className="p-3.5 bg-gray-50 rounded-xl border border-gray-200">
                <span className="text-xs font-medium text-gray-600">Foundation Donations</span>
                <p className="text-lg font-bold text-gray-900 mt-1">
                  {data.foundation_donations_formatted || formatKobo(data.foundation_donations_kobo)}
                </p>
              </div>
              <div className="p-3.5 bg-gray-50 rounded-xl border border-gray-200">
                <span className="text-xs font-medium text-gray-600">Active Jurisdiction</span>
                <p className="text-base font-bold text-gray-900 mt-1">
                  {data.active_states_count ?? 0} States / {data.active_lgas_count ?? 0} LGAs
                </p>
              </div>
            </div>
          </div>
        );
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="snapshot-detail-title"
      className="fixed inset-0 z-50 overflow-hidden bg-gray-900/60 backdrop-blur-xs flex justify-end"
    >
      <div className="bg-white w-full max-w-4xl h-full shadow-2xl flex flex-col border-l border-gray-200 animate-in slide-in-from-right duration-200">
        {/* Drawer Header */}
        <div className="p-5 border-b border-gray-200 bg-gray-50/50 flex items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="font-mono text-xs font-bold text-emerald-900 bg-emerald-100/60 px-2 py-0.5 rounded border border-emerald-200">
                {snapshot?.public_reference || "Loading..."}
              </span>
              {snapshot && getStatusBadge(snapshot.status)}
              <span className="text-xs text-gray-500 font-medium">
                Version {snapshot?.publication_version ?? 1}
              </span>
            </div>
            <h3 id="snapshot-detail-title" className="text-lg font-bold text-gray-900 leading-snug">
              {snapshot?.title || "Report Snapshot"}
            </h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-200/50 transition-colors"
            aria-label="Close inspection drawer"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Loading / Error states */}
        {loading && (
          <div className="flex-1 flex flex-col items-center justify-center p-8 space-y-3">
            <Spinner size="lg" />
            <p className="text-sm font-medium text-gray-600">Loading report snapshot contents...</p>
          </div>
        )}

        {error && (
          <div className="flex-1 p-6 space-y-4">
            <Alert type="error" title="Access Authorization Failure">
              {error}
            </Alert>
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-xs font-semibold rounded-lg border border-gray-300 text-gray-700 hover:bg-gray-50"
            >
              Back to Reports Dashboard
            </button>
          </div>
        )}

        {/* Loaded Content */}
        {!loading && !error && snapshot && (
          <>
            {/* Navigation Tabs */}
            <div className="flex items-center gap-1 px-5 border-b border-gray-200 bg-white overflow-x-auto text-xs font-medium">
              <button
                type="button"
                onClick={() => setActiveTab("preview")}
                className={`py-3 px-3.5 border-b-2 font-semibold transition-colors flex items-center gap-1.5 whitespace-nowrap ${
                  activeTab === "preview"
                    ? "border-emerald-700 text-emerald-800"
                    : "border-transparent text-gray-500 hover:text-gray-800"
                }`}
              >
                <Eye className="w-3.5 h-3.5" />
                <span>Report Preview</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveTab("details")}
                className={`py-3 px-3.5 border-b-2 font-semibold transition-colors flex items-center gap-1.5 whitespace-nowrap ${
                  activeTab === "details"
                    ? "border-emerald-700 text-emerald-800"
                    : "border-transparent text-gray-500 hover:text-gray-800"
                }`}
              >
                <FileText className="w-3.5 h-3.5" />
                <span>Metadata & Scope</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveTab("integrity")}
                className={`py-3 px-3.5 border-b-2 font-semibold transition-colors flex items-center gap-1.5 whitespace-nowrap ${
                  activeTab === "integrity"
                    ? "border-emerald-700 text-emerald-800"
                    : "border-transparent text-gray-500 hover:text-gray-800"
                }`}
              >
                <ShieldCheck className="w-3.5 h-3.5" />
                <span>Cryptographic Digest</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveTab("governance")}
                className={`py-3 px-3.5 border-b-2 font-semibold transition-colors flex items-center gap-1.5 whitespace-nowrap ${
                  activeTab === "governance"
                    ? "border-emerald-700 text-emerald-800"
                    : "border-transparent text-gray-500 hover:text-gray-800"
                }`}
              >
                <CheckCircle2 className="w-3.5 h-3.5" />
                <span>Approvals ({snapshot.approvals?.length ?? 0})</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveTab("audit")}
                className={`py-3 px-3.5 border-b-2 font-semibold transition-colors flex items-center gap-1.5 whitespace-nowrap ${
                  activeTab === "audit"
                    ? "border-emerald-700 text-emerald-800"
                    : "border-transparent text-gray-500 hover:text-gray-800"
                }`}
              >
                <Clock className="w-3.5 h-3.5" />
                <span>Audit Trail ({snapshot.audit_logs?.length ?? 0})</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveTab("exports")}
                className={`py-3 px-3.5 border-b-2 font-semibold transition-colors flex items-center gap-1.5 whitespace-nowrap ${
                  activeTab === "exports"
                    ? "border-emerald-700 text-emerald-800"
                    : "border-transparent text-gray-500 hover:text-gray-800"
                }`}
              >
                <Download className="w-3.5 h-3.5" />
                <span>Export Documents ({snapshot.exports?.length ?? 0})</span>
              </button>
            </div>

            {/* Scrollable Main Area */}
            <div className="flex-1 overflow-y-auto p-6 space-y-6">
              {/* TAB 1: REPORT PREVIEW */}
              {activeTab === "preview" && (
                <div className="space-y-6">
                  {/* Category banner */}
                  <div className="flex items-center justify-between p-4 bg-gray-50 rounded-xl border border-gray-200">
                    <div className="flex items-center gap-3">
                      <div className="p-2.5 bg-emerald-100 text-emerald-800 rounded-lg">
                        <FileText className="w-5 h-5" />
                      </div>
                      <div>
                        <span className="text-[11px] font-bold text-gray-500 uppercase tracking-wider block">
                          Report Category
                        </span>
                        <span className="text-sm font-bold text-gray-900 capitalize">
                          {snapshot.report_type.replace("_", " ")} Report
                        </span>
                      </div>
                    </div>
                    <div className="text-right">
                      <span className="text-[11px] font-bold text-gray-500 uppercase tracking-wider block">
                        Period & Scope
                      </span>
                      <span className="text-xs font-semibold text-gray-800">
                        {snapshot.reporting_period} • {snapshot.scope_type.toUpperCase()}
                      </span>
                    </div>
                  </div>

                  {/* Actual aggregated content preview */}
                  {renderReportContent()}

                  {/* Collapsible Raw JSON for Technical Administrators */}
                  <div className="pt-4 border-t border-gray-200">
                    <button
                      type="button"
                      onClick={() => setShowRawJson(!showRawJson)}
                      className="inline-flex items-center gap-1.5 text-xs font-semibold text-gray-600 hover:text-gray-900"
                    >
                      <FileCode className="w-4 h-4 text-gray-400" />
                      <span>{showRawJson ? "Hide Raw JSON Source Data" : "Inspect Raw JSON Source Data"}</span>
                      {showRawJson ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                    </button>

                    {showRawJson && (
                      <div className="mt-3 relative rounded-xl border border-gray-200 bg-gray-900 text-gray-100 p-4 overflow-hidden">
                        <div className="flex items-center justify-between pb-2 border-b border-gray-800 mb-2">
                          <span className="font-mono text-[11px] text-gray-400">aggregated_data (JSON)</span>
                          <button
                            type="button"
                            onClick={handleCopyJson}
                            className="inline-flex items-center gap-1 px-2 py-1 text-[11px] font-semibold text-gray-300 hover:text-white bg-gray-800 hover:bg-gray-700 rounded transition-colors"
                          >
                            {copiedJson ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                            <span>{copiedJson ? "Copied" : "Copy JSON"}</span>
                          </button>
                        </div>
                        <pre className="font-mono text-xs overflow-x-auto max-h-72 text-emerald-300">
                          {JSON.stringify(snapshot.aggregated_data, null, 2)}
                        </pre>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* TAB 2: METADATA & SCOPE */}
              {activeTab === "details" && (
                <div className="space-y-6">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
                    <div className="p-4 rounded-xl bg-gray-50 border border-gray-200 space-y-1">
                      <span className="text-gray-500 font-medium">Public Reference</span>
                      <p className="font-mono text-sm font-bold text-emerald-950">{snapshot.public_reference}</p>
                    </div>
                    <div className="p-4 rounded-xl bg-gray-50 border border-gray-200 space-y-1">
                      <span className="text-gray-500 font-medium">Reporting Category</span>
                      <p className="text-sm font-bold text-gray-900 capitalize">{snapshot.report_type.replace("_", " ")}</p>
                    </div>
                    <div className="p-4 rounded-xl bg-gray-50 border border-gray-200 space-y-1">
                      <span className="text-gray-500 font-medium">Reporting Period</span>
                      <p className="text-sm font-bold text-gray-900">{snapshot.reporting_period}</p>
                      <p className="text-[11px] text-gray-500">
                        {snapshot.period_start} to {snapshot.period_end}
                      </p>
                    </div>
                    <div className="p-4 rounded-xl bg-gray-50 border border-gray-200 space-y-1">
                      <span className="text-gray-500 font-medium">Geographic Scope</span>
                      <p className="text-sm font-bold text-gray-900 capitalize">{snapshot.scope_type}</p>
                      <p className="text-[11px] text-gray-600">
                        {snapshot.state_name ? `${snapshot.state_name} State` : ""}
                        {snapshot.lga_name ? ` • ${snapshot.lga_name} LGA` : ""}
                        {snapshot.zone_name ? ` • ${snapshot.zone_name} Zone` : ""}
                        {!snapshot.state_name && !snapshot.lga_name && !snapshot.zone_name ? "All Federal Republic of Nigeria" : ""}
                      </p>
                    </div>
                    <div className="p-4 rounded-xl bg-gray-50 border border-gray-200 space-y-1">
                      <span className="text-gray-500 font-medium">Author / Origin Attribution</span>
                      <p className="text-sm font-bold text-gray-900">
                        {snapshot.created_by_type === "system" ? "Automated Scheduled Engine" : snapshot.creator_name}
                      </p>
                      <p className="text-[11px] text-gray-500">
                        Origin: {snapshot.created_by_type === "system" ? "External Cron / Scheduled Automation" : "Administrator Manual Generation"}
                      </p>
                    </div>
                    <div className="p-4 rounded-xl bg-gray-50 border border-gray-200 space-y-1">
                      <span className="text-gray-500 font-medium">Publication Lifecycle</span>
                      <p className="text-sm font-bold text-gray-900 capitalize">Version {snapshot.publication_version}</p>
                      <p className="text-[11px] text-gray-500">Created: {formatDate(snapshot.created_at)}</p>
                    </div>
                  </div>

                  {snapshot.correction_notes && (
                    <div className="p-4 bg-amber-50 border border-amber-200 rounded-xl space-y-1">
                      <span className="text-xs font-bold text-amber-900 flex items-center gap-1.5">
                        <AlertTriangle className="w-4 h-4 text-amber-700" />
                        Official Correction Notes
                      </span>
                      <p className="text-xs text-amber-800 leading-relaxed">{snapshot.correction_notes}</p>
                    </div>
                  )}
                </div>
              )}

              {/* TAB 3: CRYPTOGRAPHIC INTEGRITY */}
              {activeTab === "integrity" && (
                <div className="space-y-6">
                  <div className="p-5 bg-gray-50 border border-gray-200 rounded-xl space-y-4">
                    <div className="flex items-center justify-between flex-wrap gap-2">
                      <div className="flex items-center gap-2">
                        <Shield className="w-5 h-5 text-emerald-800" />
                        <h4 className="text-sm font-bold text-gray-900">SHA-256 Cryptographic Hash</h4>
                      </div>
                      <span className="text-xs px-2.5 py-1 bg-amber-100 text-amber-900 rounded-full font-medium border border-amber-200">
                        Stored Digest (Verification Not Performed)
                      </span>
                    </div>

                    <div className="flex items-center gap-2 bg-white p-3 rounded-lg border border-gray-200">
                      <code className="font-mono text-xs font-bold text-gray-900 break-all flex-1">
                        {snapshot.data_hash || "No cryptographic hash recorded"}
                      </code>
                      <button
                        type="button"
                        onClick={handleCopyHash}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md bg-emerald-800 text-white hover:bg-emerald-900 transition-colors shadow-xs"
                      >
                        {copiedHash ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                        <span>{copiedHash ? "Copied" : "Copy Hash"}</span>
                      </button>
                    </div>

                    <div className="p-3 bg-amber-50/70 border border-amber-200 rounded-lg text-xs text-amber-900 leading-relaxed space-y-1">
                      <p className="font-semibold flex items-center gap-1.5">
                        <Info className="w-4 h-4 text-amber-700" />
                        Deterministic Integrity Disclosure:
                      </p>
                      <p>
                        The cryptographic digest above was computed using SHA-256 over authoritative source records at snapshot generation. Real-time browser verification is labeled <strong>Verification Not Performed</strong> because database JSONB serialization normalizes key ordering and whitespace, meaning raw byte re-hashing from JSON without canonical RFC 8785 encoding cannot guarantee exact byte equality.
                      </p>
                    </div>
                  </div>
                </div>
              )}

              {/* TAB 4: GOVERNANCE & APPROVALS */}
              {activeTab === "governance" && (
                <div className="space-y-6">
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-3">
                      Recorded Approval Decisions
                    </h4>
                    {Array.isArray(snapshot.approvals) && snapshot.approvals.length > 0 ? (
                      <div className="divide-y divide-gray-200 border border-gray-200 rounded-xl overflow-hidden bg-white">
                        {snapshot.approvals.map((ap: ReportApprovalItem, idx: number) => (
                          <div key={idx} className="p-4 space-y-2">
                            <div className="flex items-center justify-between">
                              <div className="flex items-center gap-2">
                                <span className={`text-xs px-2 py-0.5 rounded-full font-bold capitalize ${
                                  ap.decision === "approved" ? "bg-emerald-100 text-emerald-800" : "bg-rose-100 text-rose-800"
                                }`}>
                                  {ap.decision}
                                </span>
                                <span className="text-xs font-bold text-gray-900">{ap.reviewer_name}</span>
                              </div>
                              <span className="text-[11px] text-gray-500">{formatDate(ap.created_at)}</span>
                            </div>
                            <p className="text-xs text-gray-700 bg-gray-50 p-2.5 rounded-lg border border-gray-100">
                              "{ap.decision_reason || "No explicit comments attached."}"
                            </p>
                            <div className="text-[11px] text-gray-500">
                              Governance Scope: <span className="font-semibold capitalize text-gray-700">{ap.governance_scope}</span>
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="p-6 bg-gray-50 border border-gray-200 rounded-xl text-center text-xs text-gray-500">
                        No formal governance approvals or rejections have been logged for this snapshot yet.
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* TAB 5: AUDIT TRAIL */}
              {activeTab === "audit" && (
                <div className="space-y-6">
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-3">
                      Lifecycle Audit History
                    </h4>
                    {Array.isArray(snapshot.audit_logs) && snapshot.audit_logs.length > 0 ? (
                      <div className="space-y-3">
                        {snapshot.audit_logs.map((log: ReportAuditItem, idx: number) => (
                          <div key={idx} className="p-3.5 bg-gray-50 border border-gray-200 rounded-xl space-y-1.5 text-xs">
                            <div className="flex items-center justify-between">
                              <span className="font-mono text-emerald-900 font-bold capitalize">
                                {log.action.replace("_", " ")}
                              </span>
                              <span className="text-gray-500 text-[11px]">{formatDate(log.created_at)}</span>
                            </div>
                            <p className="text-gray-700">
                              Actor: <span className="font-semibold">{log.actor_name || "System Automation"}</span>
                            </p>
                            {log.ip_address && isSuperAdmin && (
                              <p className="text-[11px] text-gray-400 font-mono">
                                Client IP: {log.ip_address}
                              </p>
                            )}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="p-6 bg-gray-50 border border-gray-200 rounded-xl text-center text-xs text-gray-500">
                        No audit trail records found.
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* TAB 6: EXPORT DOCUMENTS */}
              {activeTab === "exports" && (
                <div className="space-y-6">
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-3">
                      Export Artifacts & Private Cloudflare R2 Storage
                    </h4>
                    {Array.isArray(snapshot.exports) && snapshot.exports.length > 0 ? (
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        {snapshot.exports.map((exp: AdminReportExportDTO) => (
                          <div key={exp.id} className="p-4 rounded-xl border border-gray-200 bg-white space-y-3 shadow-xs">
                            <div className="flex items-center justify-between">
                              <div className="flex items-center gap-2">
                                {exp.export_format === "pdf" ? (
                                  <FileText className="w-5 h-5 text-rose-600" />
                                ) : (
                                  <FileSpreadsheet className="w-5 h-5 text-emerald-600" />
                                )}
                                <span className="font-bold text-xs uppercase text-gray-900">
                                  {exp.export_format} Document
                                </span>
                              </div>
                              <span className={`text-[10px] px-2 py-0.5 rounded-full font-bold uppercase ${
                                exp.export_status === "generated" ? "bg-emerald-100 text-emerald-800" : "bg-gray-100 text-gray-700"
                              }`}>
                                {exp.export_status}
                              </span>
                            </div>

                            <p className="font-mono text-xs text-gray-700 truncate" title={exp.file_name}>
                              {exp.file_name}
                            </p>

                            <div className="text-[11px] text-gray-500 space-y-0.5">
                              <div>Size: {(exp.file_size_bytes / 1024).toFixed(1)} KB</div>
                              <div>SHA-256: <span className="font-mono text-[10px]">{exp.content_hash.slice(0, 16)}...</span></div>
                              <div>Generated: {formatDate(exp.created_at)}</div>
                            </div>

                            <div className="flex items-center gap-2 pt-2 border-t border-gray-100">
                              <button
                                type="button"
                                onClick={() => handleDownload(exp.export_format)}
                                className="flex-1 inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg bg-gray-900 text-white hover:bg-black transition-colors"
                              >
                                <Download className="w-3.5 h-3.5" />
                                <span>Download</span>
                              </button>
                              {isNationalAdmin && (
                                <button
                                  type="button"
                                  disabled={regeneratingFormat === exp.export_format}
                                  onClick={() => handleRegenerate(exp.export_format)}
                                  className="inline-flex items-center justify-center p-1.5 text-xs font-semibold rounded-lg border border-gray-300 text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-50"
                                  title="Regenerate this export in private R2"
                                >
                                  {regeneratingFormat === exp.export_format ? (
                                    <Spinner size="sm" />
                                  ) : (
                                    <RefreshCw className="w-3.5 h-3.5" />
                                  )}
                                </button>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="p-8 text-center bg-gray-50 border border-dashed border-gray-300 rounded-xl space-y-3">
                        <Download className="w-8 h-8 text-gray-400 mx-auto" />
                        <p className="text-xs text-gray-600 font-semibold">No Export Files Generated Yet</p>
                        <p className="text-[11px] text-gray-500 max-w-sm mx-auto">
                          Export documents (PDF and CSV) are generated upon initial administrative inspection or report approval.
                        </p>
                        {isNationalAdmin && (
                          <div className="flex justify-center gap-2 pt-2">
                            <button
                              type="button"
                              disabled={regeneratingFormat === "pdf"}
                              onClick={() => handleRegenerate("pdf")}
                              className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-emerald-800 text-white hover:bg-emerald-900"
                            >
                              Generate PDF Export
                            </button>
                            <button
                              type="button"
                              disabled={regeneratingFormat === "csv"}
                              onClick={() => handleRegenerate("csv")}
                              className="px-3 py-1.5 text-xs font-semibold rounded-lg border border-gray-300 text-gray-700 hover:bg-gray-50"
                            >
                              Generate CSV Export
                            </button>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* Governance Action Modal / Subform */}
            {governanceAction && (
              <div className="p-5 bg-amber-50/90 border-t border-amber-200 space-y-3">
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-bold text-amber-950 uppercase tracking-wider flex items-center gap-1.5">
                    <AlertTriangle className="w-4 h-4 text-amber-700" />
                    Confirm Action: {governanceAction.replace("_", " ")}
                  </h4>
                  <button
                    type="button"
                    onClick={() => {
                      setGovernanceAction(null);
                      setActionError(null);
                    }}
                    className="text-xs text-gray-500 hover:text-gray-800 underline"
                  >
                    Cancel
                  </button>
                </div>

                {/* Concise Summary Before Approval */}
                {governanceAction === "approve" && (
                  <div className="p-3 bg-white rounded-lg border border-amber-200 text-xs text-gray-800 space-y-1">
                    <p className="font-semibold text-gray-900">Report Review Confirmation:</p>
                    <p>
                      You are certifying snapshot <strong className="font-mono">{snapshot.public_reference}</strong> ({snapshot.title}) for period <strong>{snapshot.reporting_period}</strong>.
                    </p>
                    <p className="text-[11px] text-gray-500">
                      Scope: {snapshot.scope_type.toUpperCase()} • Author: {snapshot.creator_name}
                    </p>
                  </div>
                )}

                {actionError && (
                  <Alert type="error" title="Governance Failed">
                    {actionError}
                  </Alert>
                )}

                {actionSuccess && (
                  <Alert type="success" title="Success">
                    {actionSuccess}
                  </Alert>
                )}

                <form onSubmit={handleExecuteGovernance} className="space-y-3">
                  <div>
                    <label htmlFor="gov-reason" className="block text-xs font-bold text-gray-700 mb-1">
                      {governanceAction === "withdraw" ? (
                        <span className="text-rose-700">Withdrawal Reason (Mandatory)*</span>
                      ) : (
                        "Governance Review Notes / Justification"
                      )}
                    </label>
                    <textarea
                      id="gov-reason"
                      rows={2}
                      required={governanceAction === "withdraw"}
                      value={governanceReason}
                      onChange={(e) => setGovernanceReason(e.target.value)}
                      placeholder={
                        governanceAction === "withdraw"
                          ? "Enter the mandatory justification for withdrawing this publication..."
                          : "Enter optional sign-off remarks or review feedback..."
                      }
                      className="w-full text-xs p-2.5 bg-white border border-gray-300 rounded-lg shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                    />
                  </div>

                  <div className="flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={() => setGovernanceAction(null)}
                      className="px-3 py-1.5 rounded-lg border border-gray-300 text-xs font-semibold text-gray-700 hover:bg-gray-50"
                    >
                      Back
                    </button>
                    <button
                      type="submit"
                      disabled={isSubmittingAction}
                      className={`inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-bold text-white shadow-xs ${
                        governanceAction === "withdraw"
                          ? "bg-rose-700 hover:bg-rose-800"
                          : "bg-emerald-800 hover:bg-emerald-900"
                      } disabled:opacity-50`}
                    >
                      {isSubmittingAction && <Spinner size="sm" />}
                      <span>Execute {governanceAction.toUpperCase()}</span>
                    </button>
                  </div>
                </form>
              </div>
            )}

            {/* Footer Governance Toolbar */}
            {!governanceAction && (
              <div className="p-4 border-t border-gray-200 bg-gray-50/80 flex items-center justify-between gap-3 flex-wrap">
                <div className="text-xs text-gray-500">
                  {isCreator && snapshot.status === "pending_review" && (
                    <span className="text-amber-700 font-semibold flex items-center gap-1">
                      <ShieldAlert className="w-4 h-4" />
                      Anti-Self-Approval: Creator cannot approve own report.
                    </span>
                  )}
                </div>

                <div className="flex items-center gap-2 flex-wrap">
                  {/* Action 1: Submit for Review */}
                  {(snapshot.status === "draft" || snapshot.status === "rejected") && (
                    <button
                      type="button"
                      onClick={() => setGovernanceAction("submit_review")}
                      className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold rounded-lg bg-emerald-800 text-white hover:bg-emerald-900 shadow-xs transition-colors"
                    >
                      <Send className="w-3.5 h-3.5" />
                      <span>Submit for Review</span>
                    </button>
                  )}

                  {/* Action 2: Approve & Reject */}
                  {snapshot.status === "pending_review" && (
                    <>
                      <button
                        type="button"
                        disabled={isCreator || ((snapshot.report_type === "finance" || snapshot.report_type === "annual_impact") && !isSuperAdmin)}
                        onClick={() => setGovernanceAction("approve")}
                        className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold rounded-lg bg-emerald-800 text-white hover:bg-emerald-900 shadow-xs transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                        title={
                          isCreator
                            ? "Anti-Self-Approval: Creator cannot approve own report"
                            : (snapshot.report_type === "finance" && !isSuperAdmin)
                            ? "Finance reports require Super Admin certification"
                            : "Approve Snapshot"
                        }
                      >
                        <CheckCircle2 className="w-3.5 h-3.5" />
                        <span>Approve</span>
                      </button>
                      <button
                        type="button"
                        disabled={isCreator}
                        onClick={() => setGovernanceAction("reject")}
                        className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold rounded-lg bg-rose-700 text-white hover:bg-rose-800 shadow-xs transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                        title={isCreator ? "Anti-Self-Approval: Creator cannot reject own report" : "Reject Snapshot"}
                      >
                        <XCircle className="w-3.5 h-3.5" />
                        <span>Reject</span>
                      </button>
                    </>
                  )}

                  {/* Action 3: Publish */}
                  {snapshot.status === "approved" && isSuperAdmin && (
                    <button
                      type="button"
                      onClick={() => setGovernanceAction("publish")}
                      className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold rounded-lg bg-blue-700 text-white hover:bg-blue-800 shadow-xs transition-colors"
                    >
                      <ShieldCheck className="w-3.5 h-3.5" />
                      <span>Publish Public Report</span>
                    </button>
                  )}

                  {/* Action 4: Withdraw */}
                  {(snapshot.status === "approved" || snapshot.status === "published") && isSuperAdmin && (
                    <button
                      type="button"
                      onClick={() => setGovernanceAction("withdraw")}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg border border-rose-300 text-rose-700 hover:bg-rose-50 transition-colors"
                    >
                      <AlertTriangle className="w-3.5 h-3.5" />
                      <span>Withdraw</span>
                    </button>
                  )}

                  {/* Action 5: Revise */}
                  {snapshot.status !== "withdrawn" && snapshot.status !== "draft" && (
                    <button
                      type="button"
                      onClick={() => setGovernanceAction("revise")}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg border border-gray-300 text-gray-700 hover:bg-gray-50 transition-colors"
                    >
                      <RefreshCw className="w-3.5 h-3.5" />
                      <span>Draft Revision</span>
                    </button>
                  )}
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
