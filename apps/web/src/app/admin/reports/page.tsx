"use client";

import React, { useEffect, useState, useTransition, useId } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  BarChart3,
  Calendar,
  Clock,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  FileText,
  Download,
  RefreshCw,
  Eye,
  Send,
  ShieldCheck,
  ShieldAlert,
  Play,
  RotateCcw,
  Search,
  Filter,
  Check,
  ChevronRight,
  ChevronLeft,
  Info,
  Layers,
  ArrowRight,
  Database,
  ExternalLink,
  Lock,
  FileSpreadsheet,
  FileCheck,
  History,
  AlertCircle,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  AdminDashboardStats,
  AdminSnapshotDetailDTO,
  PaginatedSnapshotsResponse,
  SchedulerRunItem,
  PaginatedSchedulerRunsResponse,
  AdminExportItem,
  PaginatedAdminExportsResponse,
  ReportAuditItem,
  PaginatedAuditsResponse,
  RunSchedulerPayload,
  SchedulerRunResult,
  Zone,
  State,
  LGA,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { EmptyState, Spinner } from "@/components/ui/EmptyState";
import { ReportInspectionDrawer } from "@/components/admin/ReportInspectionDrawer";

type TabKey = "overview" | "snapshots" | "scheduler" | "exports" | "audit";

export default function AdminReportsDashboardPage() {
  const router = useRouter();
  const { user, isCoordinator, isSuperAdmin, isAdmin, isLoading: authLoading } = useAuth();
  const isNationalAdmin = Boolean(
    user && (user.primary_role === "national_admin" || user.primary_role === "super_admin" || isSuperAdmin)
  );

  // Active navigation tab
  const [activeTab, setActiveTab] = useState<TabKey>("overview");

  // Dashboard Stats State
  const [stats, setStats] = useState<AdminDashboardStats | null>(null);
  const [statsLoading, setStatsLoading] = useState<boolean>(true);
  const [statsError, setStatsError] = useState<string | null>(null);

  // Snapshots State
  const [snapshots, setSnapshots] = useState<AdminSnapshotDetailDTO[]>([]);
  const [snapshotsLoading, setSnapshotsLoading] = useState<boolean>(false);
  const [snapshotsTotal, setSnapshotsTotal] = useState<number>(0);
  const [snapshotsPage, setSnapshotsPage] = useState<number>(1);
  const [snapshotStatusFilter, setSnapshotStatusFilter] = useState<string>("all");
  const [snapshotTypeFilter, setSnapshotTypeFilter] = useState<string>("all");
  const [snapshotSearch, setSnapshotSearch] = useState<string>("");

  // Scheduler Runs State
  const [schedulerRuns, setSchedulerRuns] = useState<SchedulerRunItem[]>([]);
  const [schedulerLoading, setSchedulerLoading] = useState<boolean>(false);
  const [schedulerTotal, setSchedulerTotal] = useState<number>(0);
  const [schedulerPage, setSchedulerPage] = useState<number>(1);
  const [schedulerStatusFilter, setSchedulerStatusFilter] = useState<string>("all");
  const [schedulerSourceFilter, setSchedulerSourceFilter] = useState<string>("all");
  const [schedulerPeriodFilter, setSchedulerPeriodFilter] = useState<string>("");
  const [selectedRunDetail, setSelectedRunDetail] = useState<SchedulerRunItem | null>(null);

  // Exports State
  const [exportsList, setExportsList] = useState<AdminExportItem[]>([]);
  const [exportsLoading, setExportsLoading] = useState<boolean>(false);
  const [exportsTotal, setExportsTotal] = useState<number>(0);
  const [exportsPage, setExportsPage] = useState<number>(1);
  const [exportFormatFilter, setExportFormatFilter] = useState<string>("all");
  const [exportStatusFilter, setExportStatusFilter] = useState<string>("all");
  const [regeneratingExportId, setRegeneratingExportId] = useState<string | null>(null);

  // Audits State
  const [audits, setAudits] = useState<ReportAuditItem[]>([]);
  const [auditsLoading, setAuditsLoading] = useState<boolean>(false);
  const [auditsTotal, setAuditsTotal] = useState<number>(0);
  const [auditsPage, setAuditsPage] = useState<number>(1);
  const [auditActionFilter, setAuditActionFilter] = useState<string>("all");
  const [auditOutcomeFilter, setAuditOutcomeFilter] = useState<string>("all");
  const [auditDateFrom, setAuditDateFrom] = useState<string>("");
  const [auditDateTo, setAuditDateTo] = useState<string>("");

  // Manual Generation Modal State
  const [showRunModal, setShowRunModal] = useState<boolean>(false);
  const [manualPeriodType, setManualPeriodType] = useState<"monthly" | "quarterly">("monthly");
  const [manualPeriodValue, setManualPeriodValue] = useState<string>("");
  const [manualReportType, setManualReportType] = useState<string>("all");
  const [manualScopeType, setManualScopeType] = useState<string>("national");
  const [manualConfirmChecked, setManualConfirmChecked] = useState<boolean>(false);
  const [isSubmittingRun, setIsSubmittingRun] = useState<boolean>(false);
  const [runError, setRunError] = useState<string | null>(null);
  const [runResult, setRunResult] = useState<SchedulerRunResult | null>(null);

  // Governance / Snapshot Action State
  const [selectedSnapshot, setSelectedSnapshot] = useState<AdminSnapshotDetailDTO | null>(null);
  const [inspectingSnapshotId, setInspectingSnapshotId] = useState<string | null>(null);
  const [governanceAction, setGovernanceAction] = useState<
    "submit" | "approve" | "publish" | "revise" | "withdraw" | null
  >(null);
  const [governanceReason, setGovernanceReason] = useState<string>("");
  const [governanceError, setGovernanceError] = useState<string | null>(null);
  const [governanceSuccess, setGovernanceSuccess] = useState<string | null>(null);
  const [isGovernanceSubmitting, setIsGovernanceSubmitting] = useState<boolean>(false);

  // Geography references for filters
  const [zones, setZones] = useState<Zone[]>([]);
  const [states, setStates] = useState<State[]>([]);

  // Auth guard
  useEffect(() => {
    if (!authLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin/reports");
    }
  }, [user, isCoordinator, authLoading, router]);

  // Load geography
  useEffect(() => {
    async function loadGeo() {
      try {
        const [z, s] = await Promise.all([
          apiRequest<Zone[]>("/geography/zones").catch(() => []),
          apiRequest<State[]>("/geography/states").catch(() => []),
        ]);
        if (z) setZones(z);
        if (s) setStates(s);
      } catch (err) {
        console.error("Failed to load geography data", err);
      }
    }
    loadGeo();
  }, []);

  // Fetch Dashboard Stats
  const fetchStats = async () => {
    setStatsLoading(true);
    setStatsError(null);
    try {
      const data = await apiRequest<AdminDashboardStats>("/admin/reports/dashboard/stats");
      setStats(data);
    } catch (err: any) {
      setStatsError(err.message || "Unable to load dashboard statistics");
    } finally {
      setStatsLoading(false);
    }
  };

  // Fetch Snapshots
  const fetchSnapshots = async () => {
    setSnapshotsLoading(true);
    try {
      const params = new URLSearchParams({
        page: snapshotsPage.toString(),
        page_size: "10",
      });
      if (snapshotStatusFilter !== "all") params.append("status", snapshotStatusFilter);
      if (snapshotTypeFilter !== "all") params.append("report_type", snapshotTypeFilter);
      if (snapshotSearch.trim()) params.append("search", snapshotSearch.trim());

      const data = await apiRequest<PaginatedSnapshotsResponse>(
        `/admin/reports?${params.toString()}`
      );
      setSnapshots(data.snapshots || []);
      setSnapshotsTotal(data.total_count || 0);
    } catch (err) {
      console.error("Failed to load snapshots", err);
    } finally {
      setSnapshotsLoading(false);
    }
  };

  // Fetch Scheduler Runs
  const fetchSchedulerRuns = async () => {
    setSchedulerLoading(true);
    try {
      const params = new URLSearchParams({
        page: schedulerPage.toString(),
        page_size: "10",
      });
      if (schedulerStatusFilter !== "all") params.append("status", schedulerStatusFilter);
      if (schedulerSourceFilter !== "all") params.append("trigger_source", schedulerSourceFilter);
      if (schedulerPeriodFilter.trim()) params.append("reporting_period", schedulerPeriodFilter.trim());

      const data = await apiRequest<PaginatedSchedulerRunsResponse>(
        `/admin/reports/scheduler/runs?${params.toString()}`
      );
      setSchedulerRuns(data.runs || []);
      setSchedulerTotal(data.total_count || 0);
    } catch (err) {
      console.error("Failed to load scheduler runs", err);
    } finally {
      setSchedulerLoading(false);
    }
  };

  // Fetch Exports
  const fetchExports = async () => {
    setExportsLoading(true);
    try {
      const params = new URLSearchParams({
        page: exportsPage.toString(),
        page_size: "10",
      });
      if (exportFormatFilter !== "all") params.append("format", exportFormatFilter);
      if (exportStatusFilter !== "all") params.append("status", exportStatusFilter);

      const data = await apiRequest<PaginatedAdminExportsResponse>(
        `/admin/reports/exports?${params.toString()}`
      );
      setExportsList(data.exports || []);
      setExportsTotal(data.total_count || 0);
    } catch (err) {
      console.error("Failed to load exports", err);
    } finally {
      setExportsLoading(false);
    }
  };

  // Fetch Audits
  const fetchAudits = async () => {
    setAuditsLoading(true);
    try {
      const params = new URLSearchParams({
        page: auditsPage.toString(),
        page_size: "15",
      });
      if (auditActionFilter !== "all") params.append("action", auditActionFilter);
      if (auditOutcomeFilter !== "all") params.append("outcome", auditOutcomeFilter);
      if (auditDateFrom) params.append("date_from", auditDateFrom);
      if (auditDateTo) params.append("date_to", auditDateTo);

      const data = await apiRequest<PaginatedAuditsResponse>(
        `/admin/reports/audit?${params.toString()}`
      );
      setAudits(data.audits || []);
      setAuditsTotal(data.total_count || 0);
    } catch (err) {
      console.error("Failed to load audit history", err);
    } finally {
      setAuditsLoading(false);
    }
  };

  // Initial load & Tab switching
  useEffect(() => {
    if (!user || !isCoordinator) return;
    if (activeTab === "overview") {
      fetchStats();
    } else if (activeTab === "snapshots") {
      fetchSnapshots();
    } else if (activeTab === "scheduler") {
      fetchSchedulerRuns();
    } else if (activeTab === "exports") {
      fetchExports();
    } else if (activeTab === "audit") {
      fetchAudits();
    }
  }, [
    activeTab,
    snapshotsPage,
    snapshotStatusFilter,
    snapshotTypeFilter,
    schedulerPage,
    schedulerStatusFilter,
    schedulerSourceFilter,
    exportsPage,
    exportFormatFilter,
    exportStatusFilter,
    auditsPage,
    auditActionFilter,
    auditOutcomeFilter,
    user,
    isCoordinator,
  ]);

  // Set default historical periods when opening manual generation modal
  const openManualGenerationModal = () => {
    const now = new Date();
    // Default to previous concluded month: e.g., if now is 2026-10, default to 2026-09
    const prevMonthDate = new Date(now.getFullYear(), now.getMonth() - 1, 1);
    const yyyy = prevMonthDate.getFullYear();
    const mm = String(prevMonthDate.getMonth() + 1).padStart(2, "0");
    setManualPeriodValue(`${yyyy}-${mm}`);
    setManualPeriodType("monthly");
    setManualReportType("all");
    setManualScopeType("national");
    setManualConfirmChecked(false);
    setRunError(null);
    setRunResult(null);
    setShowRunModal(true);
  };

  // Validate historical period format and boundaries
  const validatePeriod = (period: string, type: "monthly" | "quarterly"): string | null => {
    const trimmed = period.trim();
    if (!trimmed) return "Reporting period cannot be empty";

    const now = new Date();
    const currentYear = now.getFullYear();
    const currentMonth = now.getMonth() + 1; // 1-12

    if (type === "monthly") {
      const match = trimmed.match(/^(\d{4})-(\d{2})$/);
      if (!match) return "Monthly period must be in YYYY-MM format (e.g., 2024-05)";
      const y = parseInt(match[1], 10);
      const m = parseInt(match[2], 10);
      if (m < 1 || m > 12) return "Month must be between 01 and 12";
      if (y > currentYear || (y === currentYear && m >= currentMonth)) {
        return "Manual generation is strictly prohibited for future or incomplete current periods. Only concluded periods may be generated.";
      }
    } else {
      const match = trimmed.match(/^(\d{4})-Q([1-4])$/i);
      if (!match) return "Quarterly period must be in YYYY-Q# format (e.g., 2024-Q2)";
      const y = parseInt(match[1], 10);
      const q = parseInt(match[2], 10);
      const currentQuarter = Math.ceil(currentMonth / 3);
      if (y > currentYear || (y === currentYear && q >= currentQuarter)) {
        return "Quarter has not concluded yet. Only concluded quarters may be generated.";
      }
    }
    return null;
  };

  // Execute Manual Report Generation
  const handleExecuteManualRun = async (e: React.FormEvent) => {
    e.preventDefault();
    setRunError(null);

    const validationMsg = validatePeriod(manualPeriodValue, manualPeriodType);
    if (validationMsg) {
      setRunError(validationMsg);
      return;
    }

    if (!manualConfirmChecked) {
      setRunError("Please confirm that you understand this will generate draft report snapshots.");
      return;
    }

    setIsSubmittingRun(true);
    try {
      const payload: RunSchedulerPayload = {
        target_period: manualPeriodValue.trim(),
        period_type: manualPeriodType,
        report_type: manualReportType === "all" ? undefined : manualReportType,
        scope_type: manualScopeType,
      };

      const result = await apiRequest<SchedulerRunResult>(
        "/admin/reports/scheduler/run",
        {
          method: "POST",
          body: JSON.stringify(payload),
        }
      );

      setRunResult(result);
      fetchStats();
      if (activeTab === "snapshots") fetchSnapshots();
      if (activeTab === "scheduler") fetchSchedulerRuns();
    } catch (err: any) {
      setRunError(err.message || "Failed to trigger report generation");
    } finally {
      setIsSubmittingRun(false);
    }
  };

  // Handle Export Download
  const handleDownloadExport = (exportItem: AdminExportItem) => {
    const downloadUrl = `/api/v1/admin/reports/${exportItem.snapshot_id}/exports/${exportItem.export_format}/download`;
    window.open(downloadUrl, "_blank");
  };

  // Handle Export Regeneration
  const handleRegenerateExport = async (exportItem: AdminExportItem) => {
    if (!confirm(`Are you sure you want to regenerate the ${exportItem.export_format.toUpperCase()} export for "${exportItem.report_title}"? This will recreate the file in private storage and update its cryptographic checksum.`)) {
      return;
    }

    setRegeneratingExportId(exportItem.id);
    try {
      await apiRequest(
        `/admin/reports/${exportItem.snapshot_id}/exports/${exportItem.export_format}/regenerate`,
        { method: "POST" }
      );
      fetchExports();
      alert(`Successfully regenerated ${exportItem.export_format.toUpperCase()} export.`);
    } catch (err: any) {
      alert(`Export regeneration failed: ${err.message || "Unknown error"}`);
    } finally {
      setRegeneratingExportId(null);
    }
  };

  // Handle Governance Action
  const handleExecuteGovernance = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedSnapshot || !governanceAction) return;

    setGovernanceError(null);
    setGovernanceSuccess(null);
    setIsGovernanceSubmitting(true);

    try {
      let endpoint = `/admin/reports/${selectedSnapshot.snapshot_id}/${governanceAction}`;
      let body: any = {};

      if (governanceAction === "submit") {
        endpoint = `/admin/reports/${selectedSnapshot.snapshot_id}/submit-review`;
        body = { notes: governanceReason || "Submitted for administrative review" };
      } else if (governanceAction === "approve") {
        body = { notes: governanceReason || "Approved after review" };
      } else if (governanceAction === "publish") {
        body = { notes: governanceReason || "Approved for public transparency release" };
      } else if (governanceAction === "revise") {
        endpoint = `/admin/reports/${selectedSnapshot.snapshot_id}/revise`;
        body = { reason: governanceReason || "Administrative revisions required" };
      } else if (governanceAction === "withdraw") {
        if (!governanceReason.trim()) {
          setGovernanceError("A documented withdrawal reason is mandatory.");
          setIsGovernanceSubmitting(false);
          return;
        }
        body = { reason: governanceReason.trim() };
      }

      await apiRequest(endpoint, {
        method: "POST",
        body: JSON.stringify(body),
      });

      setGovernanceSuccess(`Report status updated successfully.`);
      fetchSnapshots();
      fetchStats();
      setTimeout(() => {
        setGovernanceAction(null);
        setSelectedSnapshot(null);
        setGovernanceReason("");
        setGovernanceSuccess(null);
      }, 1200);
    } catch (err: any) {
      setGovernanceError(err.message || "Action failed due to governance policy violation.");
    } finally {
      setIsGovernanceSubmitting(false);
    }
  };

  // Helper: Format file size
  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  // Helper: Format date
  const formatDate = (isoString?: string): string => {
    if (!isoString) return "—";
    try {
      const d = new Date(isoString);
      return d.toLocaleDateString("en-GB", {
        day: "2-digit",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return isoString;
    }
  };

  // Status badge styling
  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case "published":
        return <Badge variant="success">Published</Badge>;
      case "approved":
        return <Badge variant="primary">Approved</Badge>;
      case "pending_review":
        return <Badge variant="warning">Pending Review</Badge>;
      case "draft":
        return <Badge variant="neutral">Draft</Badge>;
      case "withdrawn":
      case "revoked":
        return <Badge variant="danger">Withdrawn</Badge>;
      case "completed":
        return <Badge variant="success">Completed</Badge>;
      case "failed":
        return <Badge variant="danger">Failed</Badge>;
      case "partial_failure":
        return <Badge variant="warning">Partial Failure</Badge>;
      case "running":
        return <Badge variant="info">Running</Badge>;
      default:
        return <Badge variant="neutral">{status}</Badge>;
    }
  };

  if (authLoading) {
    return (
      <div className="min-h-[50vh] flex items-center justify-center">
        <Spinner size="lg" text="Authenticating administrative session..." />
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
      {/* Page Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-gray-200 pb-6">
        <div>
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-emerald-800">
            <ShieldCheck className="w-4 h-4 text-emerald-700" />
            <span>Governance & Compliance</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-bold text-gray-900 tracking-tight mt-1">
            Reports & Transparency Administration
          </h1>
          <p className="text-sm text-gray-600 mt-1 max-w-2xl">
            Monitor automated scheduled generation, verify cryptographic audit trails, manage R2 document exports, and enforce multi-tier publication governance.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={openManualGenerationModal}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 focus:outline-none focus:ring-2 focus:ring-emerald-700 focus:ring-offset-2 shadow-sm transition"
          >
            <Play className="w-4 h-4 fill-white" />
            <span>Run Report Generation</span>
          </button>
          <button
            type="button"
            onClick={() => {
              if (activeTab === "overview") fetchStats();
              if (activeTab === "snapshots") fetchSnapshots();
              if (activeTab === "scheduler") fetchSchedulerRuns();
              if (activeTab === "exports") fetchExports();
              if (activeTab === "audit") fetchAudits();
            }}
            aria-label="Refresh current tab"
            className="p-2 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-emerald-700 transition"
            title="Refresh current dataset"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Accessible Tab Navigation */}
      <nav
        aria-label="Reports Administration Subnavigation"
        className="flex border-b border-gray-200 space-x-1 overflow-x-auto scrollbar-none"
      >
        <button
          onClick={() => setActiveTab("overview")}
          className={`py-3 px-4 text-sm font-medium border-b-2 whitespace-nowrap transition-colors flex items-center gap-2 ${
            activeTab === "overview"
              ? "border-emerald-800 text-emerald-900 font-semibold"
              : "border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300"
          }`}
          aria-current={activeTab === "overview" ? "page" : undefined}
        >
          <BarChart3 className="w-4 h-4" />
          <span>Overview</span>
        </button>

        <button
          onClick={() => setActiveTab("snapshots")}
          className={`py-3 px-4 text-sm font-medium border-b-2 whitespace-nowrap transition-colors flex items-center gap-2 ${
            activeTab === "snapshots"
              ? "border-emerald-800 text-emerald-900 font-semibold"
              : "border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300"
          }`}
          aria-current={activeTab === "snapshots" ? "page" : undefined}
        >
          <FileText className="w-4 h-4" />
          <span>Report Snapshots</span>
          {stats && stats.total_snapshots > 0 && (
            <span className="ml-1 px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-700 font-medium">
              {stats.total_snapshots}
            </span>
          )}
        </button>

        <button
          onClick={() => setActiveTab("scheduler")}
          className={`py-3 px-4 text-sm font-medium border-b-2 whitespace-nowrap transition-colors flex items-center gap-2 ${
            activeTab === "scheduler"
              ? "border-emerald-800 text-emerald-900 font-semibold"
              : "border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300"
          }`}
          aria-current={activeTab === "scheduler" ? "page" : undefined}
        >
          <Clock className="w-4 h-4" />
          <span>Scheduled Jobs</span>
        </button>

        <button
          onClick={() => setActiveTab("exports")}
          className={`py-3 px-4 text-sm font-medium border-b-2 whitespace-nowrap transition-colors flex items-center gap-2 ${
            activeTab === "exports"
              ? "border-emerald-800 text-emerald-900 font-semibold"
              : "border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300"
          }`}
          aria-current={activeTab === "exports" ? "page" : undefined}
        >
          <Download className="w-4 h-4" />
          <span>Exports & R2 Storage</span>
        </button>

        <button
          onClick={() => setActiveTab("audit")}
          className={`py-3 px-4 text-sm font-medium border-b-2 whitespace-nowrap transition-colors flex items-center gap-2 ${
            activeTab === "audit"
              ? "border-emerald-800 text-emerald-900 font-semibold"
              : "border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300"
          }`}
          aria-current={activeTab === "audit" ? "page" : undefined}
        >
          <History className="w-4 h-4" />
          <span>Audit History</span>
        </button>
      </nav>

      {/* =========================================================================
          TAB 1: OVERVIEW & REAL-TIME METRICS
          ========================================================================= */}
      {activeTab === "overview" && (
        <section aria-labelledby="overview-heading" className="space-y-6">
          <h2 id="overview-heading" className="sr-only">
            Reporting System Overview
          </h2>

          {statsError && (
            <Alert type="error" title="Error Loading Statistics">
              {statsError}
            </Alert>
          )}

          {statsLoading && !stats ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {[...Array(8)].map((_, i) => (
                <div key={i} className="h-28 rounded-xl bg-gray-100 animate-pulse border border-gray-200" />
              ))}
            </div>
          ) : stats ? (
            <>
              {/* Snapshot Governance Counts */}
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <div className="p-5 rounded-xl border border-gray-200 bg-white shadow-xs">
                  <div className="flex items-center justify-between text-gray-500 text-xs font-semibold uppercase tracking-wider">
                    <span>Total Snapshots</span>
                    <Layers className="w-4 h-4 text-gray-400" />
                  </div>
                  <div className="mt-2 text-3xl font-extrabold text-gray-900">
                    {stats.total_snapshots}
                  </div>
                  <div className="mt-1 text-xs text-gray-500">
                    Concluded reporting periods
                  </div>
                </div>

                <div className="p-5 rounded-xl border border-gray-200 bg-white shadow-xs">
                  <div className="flex items-center justify-between text-gray-500 text-xs font-semibold uppercase tracking-wider">
                    <span>Draft Reports</span>
                    <span className="w-2.5 h-2.5 rounded-full bg-gray-400" />
                  </div>
                  <div className="mt-2 text-3xl font-extrabold text-gray-700">
                    {stats.draft_reports}
                  </div>
                  <div className="mt-1 text-xs text-gray-500">
                    Generated & awaiting submission
                  </div>
                </div>

                <div className="p-5 rounded-xl border border-amber-200 bg-amber-50/40 shadow-xs">
                  <div className="flex items-center justify-between text-amber-800 text-xs font-semibold uppercase tracking-wider">
                    <span>Pending Review</span>
                    <AlertTriangle className="w-4 h-4 text-amber-600" />
                  </div>
                  <div className="mt-2 text-3xl font-extrabold text-amber-900">
                    {stats.pending_review_reports}
                  </div>
                  <div className="mt-1 text-xs text-amber-700">
                    Requiring administrative approval
                  </div>
                </div>

                <div className="p-5 rounded-xl border border-emerald-200 bg-emerald-50/30 shadow-xs">
                  <div className="flex items-center justify-between text-emerald-800 text-xs font-semibold uppercase tracking-wider">
                    <span>Approved Reports</span>
                    <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                  </div>
                  <div className="mt-2 text-3xl font-extrabold text-emerald-900">
                    {stats.approved_reports}
                  </div>
                  <div className="mt-1 text-xs text-emerald-700">
                    Ready for publication
                  </div>
                </div>

                <div className="p-5 rounded-xl border border-emerald-300 bg-emerald-100/40 shadow-xs">
                  <div className="flex items-center justify-between text-emerald-900 text-xs font-semibold uppercase tracking-wider">
                    <span>Published Reports</span>
                    <ShieldCheck className="w-4 h-4 text-emerald-700" />
                  </div>
                  <div className="mt-2 text-3xl font-extrabold text-emerald-950">
                    {stats.published_reports}
                  </div>
                  <div className="mt-1 text-xs text-emerald-800">
                    Live on public transparency portal
                  </div>
                </div>

                <div className="p-5 rounded-xl border border-gray-200 bg-white shadow-xs">
                  <div className="flex items-center justify-between text-gray-500 text-xs font-semibold uppercase tracking-wider">
                    <span>Withdrawn Reports</span>
                    <RotateCcw className="w-4 h-4 text-gray-400" />
                  </div>
                  <div className="mt-2 text-3xl font-extrabold text-gray-600">
                    {stats.withdrawn_reports}
                  </div>
                  <div className="mt-1 text-xs text-gray-500">
                    Revoked from public visibility
                  </div>
                </div>

                <div className="p-5 rounded-xl border border-rose-200 bg-rose-50/30 shadow-xs">
                  <div className="flex items-center justify-between text-rose-800 text-xs font-semibold uppercase tracking-wider">
                    <span>Failed Attempts</span>
                    <XCircle className="w-4 h-4 text-rose-600" />
                  </div>
                  <div className="mt-2 text-3xl font-extrabold text-rose-900">
                    {stats.failed_generation_attempts}
                  </div>
                  <div className="mt-1 text-xs text-rose-700">
                    Execution or export errors
                  </div>
                </div>

                <div className="p-5 rounded-xl border border-sky-200 bg-sky-50/30 shadow-xs">
                  <div className="flex items-center justify-between text-sky-800 text-xs font-semibold uppercase tracking-wider">
                    <span>Available Exports</span>
                    <Download className="w-4 h-4 text-sky-600" />
                  </div>
                  <div className="mt-2 text-2xl font-extrabold text-sky-950 flex items-baseline gap-2">
                    <span>{stats.available_pdf_exports}</span>
                    <span className="text-xs font-medium text-sky-700 uppercase">PDF</span>
                    <span className="text-gray-300">|</span>
                    <span>{stats.available_csv_exports}</span>
                    <span className="text-xs font-medium text-sky-700 uppercase">CSV</span>
                  </div>
                  <div className="mt-1 text-xs text-sky-700">
                    Stored privately in Cloudflare R2
                  </div>
                </div>
              </div>

              {/* Scheduler Health and Next Cycle */}
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-6">
                <div className="p-6 rounded-xl border border-gray-200 bg-white shadow-xs space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="text-base font-bold text-gray-900 flex items-center gap-2">
                      <Clock className="w-5 h-5 text-emerald-800" />
                      <span>Last Scheduler Execution</span>
                    </h3>
                    {stats.last_scheduler_run ? (
                      getStatusBadge(stats.last_scheduler_run.status)
                    ) : (
                      <Badge variant="neutral">Never Run</Badge>
                    )}
                  </div>

                  {stats.last_scheduler_run ? (
                    <div className="space-y-3 text-sm">
                      <div className="flex justify-between py-1 border-b border-gray-100">
                        <span className="text-gray-500">Run Identifier:</span>
                        <span className="font-mono text-gray-800 text-xs">
                          {stats.last_scheduler_run.run_identifier}
                        </span>
                      </div>
                      <div className="flex justify-between py-1 border-b border-gray-100">
                        <span className="text-gray-500">Reporting Period:</span>
                        <span className="font-semibold text-gray-900">
                          {stats.last_scheduler_run.reporting_period}
                        </span>
                      </div>
                      <div className="flex justify-between py-1 border-b border-gray-100">
                        <span className="text-gray-500">Trigger Source:</span>
                        <span className="capitalize text-gray-700 font-medium">
                          {stats.last_scheduler_run.trigger_source.replace("_", " ")}
                        </span>
                      </div>
                      <div className="flex justify-between py-1 border-b border-gray-100">
                        <span className="text-gray-500">Execution Time:</span>
                        <span className="text-gray-700">
                          {formatDate(stats.last_scheduler_run.started_at)}
                        </span>
                      </div>
                      <div className="flex justify-between py-1">
                        <span className="text-gray-500">Generated Snapshots:</span>
                        <span className="font-bold text-emerald-800">
                          {stats.last_scheduler_run.reports_generated} new snapshots
                        </span>
                      </div>
                    </div>
                  ) : (
                    <p className="text-sm text-gray-500 italic py-4">
                      No automated scheduled executions recorded yet.
                    </p>
                  )}

                  <div className="pt-2">
                    <button
                      type="button"
                      onClick={() => setActiveTab("scheduler")}
                      className="text-xs font-semibold text-emerald-800 hover:text-emerald-950 inline-flex items-center gap-1"
                    >
                      <span>View complete scheduler logs</span>
                      <ArrowRight className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>

                <div className="p-6 rounded-xl border border-gray-200 bg-white shadow-xs space-y-4">
                  <h3 className="text-base font-bold text-gray-900 flex items-center gap-2">
                    <Calendar className="w-5 h-5 text-emerald-800" />
                    <span>Next Expected Reporting Cycle</span>
                  </h3>

                  <div className="space-y-4">
                    <div className="p-4 rounded-lg bg-gray-50 border border-gray-200">
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-bold uppercase tracking-wider text-gray-600">
                          {stats.next_expected_period.period_type === "quarterly" ? "Quarterly Cycle" : "Monthly Cycle"}
                        </span>
                        <Badge variant="neutral">Scheduled Engine</Badge>
                      </div>
                      <div className="mt-2 text-lg font-bold text-gray-900">
                        {stats.next_expected_period.period_label}
                      </div>
                      <div className="mt-1 text-xs text-gray-500 flex items-center gap-1">
                        <Clock className="w-3.5 h-3.5 text-gray-400" />
                        <span>
                          Expected automated run:{" "}
                          <strong>{stats.next_expected_period.expected_date}</strong> (02:00 UTC)
                        </span>
                      </div>
                      <p className="mt-2 text-xs text-gray-600">
                        {stats.next_expected_period.description}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </>
          ) : null}
        </section>
      )}

      {/* =========================================================================
          TAB 2: REPORT SNAPSHOTS & GOVERNANCE WORKFLOW
          ========================================================================= */}
      {activeTab === "snapshots" && (
        <section aria-labelledby="snapshots-heading" className="space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h2 id="snapshots-heading" className="text-lg font-bold text-gray-900">
                Administrative Report Snapshots
              </h2>
              <p className="text-xs text-gray-500">
                Review snapshot metrics, verify generated data, and transition reports through the governance lifecycle.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <select
                value={snapshotStatusFilter}
                onChange={(e) => {
                  setSnapshotStatusFilter(e.target.value);
                  setSnapshotsPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter by snapshot status"
              >
                <option value="all">All Statuses</option>
                <option value="draft">Draft</option>
                <option value="pending_review">Pending Review</option>
                <option value="approved">Approved</option>
                <option value="published">Published</option>
                <option value="withdrawn">Withdrawn</option>
              </select>

              <select
                value={snapshotTypeFilter}
                onChange={(e) => {
                  setSnapshotTypeFilter(e.target.value);
                  setSnapshotsPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter by report category"
              >
                <option value="all">All Report Types</option>
                <option value="cleanups">Cleanups</option>
                <option value="community_projects">Community Projects</option>
                <option value="safeguarding">Safeguarding</option>
                <option value="financial_summary">Financial Summary</option>
              </select>
            </div>
          </div>

          {/* Search bar */}
          <div className="relative">
            <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
            <input
              type="text"
              placeholder="Search by report title or public reference..."
              value={snapshotSearch}
              onChange={(e) => {
                setSnapshotSearch(e.target.value);
                setSnapshotsPage(1);
              }}
              className="w-full pl-9 pr-4 py-2 text-sm border border-gray-300 rounded-lg shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
            />
          </div>

          {/* Snapshots Table */}
          {snapshotsLoading ? (
            <div className="py-12 flex justify-center">
              <Spinner size="md" text="Loading report snapshots..." />
            </div>
          ) : snapshots.length === 0 ? (
            <EmptyState
              title="No report snapshots found"
              description="No snapshots match the selected filter criteria or no historical reports have been generated yet."
              actionText="Generate Historical Report"
              onAction={openManualGenerationModal}
            />
          ) : (
            <div className="overflow-x-auto border border-gray-200 rounded-xl bg-white shadow-xs">
              <table className="min-w-full divide-y divide-gray-200 text-left text-sm">
                <thead className="bg-gray-50 text-xs font-semibold text-gray-600 uppercase tracking-wider">
                  <tr>
                    <th scope="col" className="px-6 py-3">Reference / Title</th>
                    <th scope="col" className="px-6 py-3">Type & Period</th>
                    <th scope="col" className="px-6 py-3">Scope</th>
                    <th scope="col" className="px-6 py-3">Status</th>
                    <th scope="col" className="px-6 py-3">Created</th>
                    <th scope="col" className="px-6 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {snapshots.map((s) => (
                    <tr key={s.snapshot_id} className="hover:bg-gray-50/80 transition-colors">
                      <td className="px-6 py-4">
                        <div className="font-mono text-xs font-bold text-emerald-950">
                          {s.public_reference}
                        </div>
                        <div className="font-medium text-gray-900 mt-0.5 line-clamp-1">
                          {s.title}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="text-gray-900 font-medium capitalize">
                          {s.report_type.replace("_", " ")}
                        </div>
                        <div className="text-xs text-gray-500 font-mono">
                          {s.reporting_period}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <span className="capitalize text-xs font-semibold px-2 py-0.5 rounded-md bg-gray-100 text-gray-700">
                          {s.scope_type}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        {getStatusBadge(s.status)}
                      </td>
                      <td className="px-6 py-4 text-xs text-gray-500">
                        {formatDate(s.created_at)}
                      </td>
                      <td className="px-6 py-4 text-right space-x-2">
                        <button
                          type="button"
                          onClick={() => {
                            setInspectingSnapshotId(s.snapshot_id);
                            setSelectedSnapshot(s);
                            setGovernanceAction(null);
                            setGovernanceReason("");
                            setGovernanceError(null);
                          }}
                          className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-semibold rounded-md border border-gray-300 text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-emerald-700"
                        >
                          <Eye className="w-3.5 h-3.5" />
                          <span>Inspect</span>
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              {/* Snapshots Pagination */}
              <div className="px-6 py-4 border-t border-gray-200 flex items-center justify-between text-xs text-gray-600 bg-gray-50/50">
                <div>
                  Showing {snapshots.length} of {snapshotsTotal} snapshots
                </div>
                <div className="flex gap-2">
                  <button
                    disabled={snapshotsPage <= 1}
                    onClick={() => setSnapshotsPage((p) => Math.max(1, p - 1))}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <button
                    disabled={snapshotsPage * 10 >= snapshotsTotal}
                    onClick={() => setSnapshotsPage((p) => p + 1)}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          )}
        </section>
      )}

      {/* =========================================================================
          TAB 3: SCHEDULED JOBS MONITORING
          ========================================================================= */}
      {activeTab === "scheduler" && (
        <section aria-labelledby="scheduler-heading" className="space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h2 id="scheduler-heading" className="text-lg font-bold text-gray-900">
                Automated Scheduler Execution History
              </h2>
              <p className="text-xs text-gray-500">
                Audit durable scheduler runs, evaluate skipped duplicates, and inspect failure classifications.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <select
                value={schedulerStatusFilter}
                onChange={(e) => {
                  setSchedulerStatusFilter(e.target.value);
                  setSchedulerPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter scheduler runs by execution status"
              >
                <option value="all">All Execution Statuses</option>
                <option value="completed">Completed</option>
                <option value="partial_failure">Partial Failure</option>
                <option value="failed">Failed</option>
                <option value="running">Running</option>
              </select>

              <select
                value={schedulerSourceFilter}
                onChange={(e) => {
                  setSchedulerSourceFilter(e.target.value);
                  setSchedulerPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter scheduler runs by trigger source"
              >
                <option value="all">All Trigger Sources</option>
                <option value="scheduled">Scheduled Engine</option>
                <option value="admin_manual">Admin Manual</option>
                <option value="external_cron">External Cron</option>
              </select>
            </div>
          </div>

          {schedulerLoading ? (
            <div className="py-12 flex justify-center">
              <Spinner size="md" text="Loading scheduler execution logs..." />
            </div>
          ) : schedulerRuns.length === 0 ? (
            <EmptyState
              title="No scheduler runs recorded"
              description="Scheduler executions will appear here once the automated engine runs or an administrator triggers a manual generation cycle."
              actionText="Run Manual Cycle"
              onAction={openManualGenerationModal}
            />
          ) : (
            <div className="overflow-x-auto border border-gray-200 rounded-xl bg-white shadow-xs">
              <table className="min-w-full divide-y divide-gray-200 text-left text-sm">
                <thead className="bg-gray-50 text-xs font-semibold text-gray-600 uppercase tracking-wider">
                  <tr>
                    <th scope="col" className="px-6 py-3">Run Identifier</th>
                    <th scope="col" className="px-6 py-3">Source & Period</th>
                    <th scope="col" className="px-6 py-3">Status</th>
                    <th scope="col" className="px-6 py-3">Results</th>
                    <th scope="col" className="px-6 py-3">Started / Actor</th>
                    <th scope="col" className="px-6 py-3 text-right">Details</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {schedulerRuns.map((r) => (
                    <tr key={r.id} className="hover:bg-gray-50/80 transition-colors">
                      <td className="px-6 py-4">
                        <div className="font-mono text-xs font-semibold text-gray-900">
                          {r.run_identifier}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="font-medium text-gray-900 capitalize">
                          {r.trigger_source.replace("_", " ")}
                        </div>
                        <div className="text-xs text-gray-500 font-mono">
                          {r.reporting_period}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        {getStatusBadge(r.status)}
                      </td>
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-2 text-xs">
                          <span className="text-emerald-700 font-semibold">
                            +{r.reports_generated} gen
                          </span>
                          <span className="text-gray-400">|</span>
                          <span className="text-gray-600">
                            {r.reports_skipped} skip
                          </span>
                          {r.reports_failed > 0 && (
                            <>
                              <span className="text-gray-400">|</span>
                              <span className="text-rose-700 font-semibold">
                                {r.reports_failed} fail
                              </span>
                            </>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4 text-xs">
                        <div className="text-gray-900 font-medium">
                          {formatDate(r.started_at)}
                        </div>
                        <div className="text-gray-500">
                          {r.actor_name || r.actor_type}
                        </div>
                      </td>
                      <td className="px-6 py-4 text-right">
                        <button
                          type="button"
                          onClick={() => setSelectedRunDetail(r)}
                          className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-semibold rounded-md border border-gray-300 text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-emerald-700"
                        >
                          <span>Inspect</span>
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              <div className="px-6 py-4 border-t border-gray-200 flex items-center justify-between text-xs text-gray-600 bg-gray-50/50">
                <div>
                  Showing {schedulerRuns.length} of {schedulerTotal} runs
                </div>
                <div className="flex gap-2">
                  <button
                    disabled={schedulerPage <= 1}
                    onClick={() => setSchedulerPage((p) => Math.max(1, p - 1))}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <button
                    disabled={schedulerPage * 10 >= schedulerTotal}
                    onClick={() => setSchedulerPage((p) => p + 1)}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          )}
        </section>
      )}

      {/* =========================================================================
          TAB 4: EXPORT MANAGEMENT & R2 STORAGE
          ========================================================================= */}
      {activeTab === "exports" && (
        <section aria-labelledby="exports-heading" className="space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h2 id="exports-heading" className="text-lg font-bold text-gray-900">
                R2 Export Management & Verification
              </h2>
              <p className="text-xs text-gray-500">
                Directly verify cryptographic SHA-256 hashes, private storage status, and initiate administrator regenerations.
              </p>
            </div>

            <div className="flex items-center gap-3">
              <select
                value={exportFormatFilter}
                onChange={(e) => {
                  setExportFormatFilter(e.target.value);
                  setExportsPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter exports by format"
              >
                <option value="all">All Formats</option>
                <option value="pdf">PDF</option>
                <option value="csv">CSV</option>
              </select>

              <select
                value={exportStatusFilter}
                onChange={(e) => {
                  setExportStatusFilter(e.target.value);
                  setExportsPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter exports by status"
              >
                <option value="all">All Statuses</option>
                <option value="generated">Generated</option>
                <option value="withdrawn">Withdrawn</option>
              </select>
            </div>
          </div>

          {exportsLoading ? (
            <div className="py-12 flex justify-center">
              <Spinner size="md" text="Loading exports from storage catalogue..." />
            </div>
          ) : exportsList.length === 0 ? (
            <EmptyState
              title="No exports catalogue records"
              description="Generated PDF and CSV exports will appear here once reports are published or manually exported."
            />
          ) : (
            <div className="overflow-x-auto border border-gray-200 rounded-xl bg-white shadow-xs">
              <table className="min-w-full divide-y divide-gray-200 text-left text-sm">
                <thead className="bg-gray-50 text-xs font-semibold text-gray-600 uppercase tracking-wider">
                  <tr>
                    <th scope="col" className="px-6 py-3">Report / Format</th>
                    <th scope="col" className="px-6 py-3">File Size</th>
                    <th scope="col" className="px-6 py-3">Storage State</th>
                    <th scope="col" className="px-6 py-3">SHA-256 Checksum</th>
                    <th scope="col" className="px-6 py-3">Created</th>
                    <th scope="col" className="px-6 py-3 text-right">Admin Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {exportsList.map((exp) => (
                    <tr key={exp.id} className="hover:bg-gray-50/80 transition-colors">
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-2">
                          {exp.export_format === "pdf" ? (
                            <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-rose-100 text-rose-800">
                              PDF
                            </span>
                          ) : (
                            <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-100 text-emerald-800">
                              CSV
                            </span>
                          )}
                          <span className="font-mono text-xs font-bold text-gray-900">
                            {exp.public_reference}
                          </span>
                        </div>
                        <div className="text-xs text-gray-600 mt-1 line-clamp-1">
                          {exp.report_title}
                        </div>
                      </td>
                      <td className="px-6 py-4 text-xs font-mono text-gray-700">
                        {formatBytes(exp.file_size_bytes)}
                      </td>
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-1.5">
                          {exp.storage_verified ? (
                            <span className="inline-flex items-center gap-1 text-xs text-emerald-800 font-semibold">
                              <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                              <span>Verified in R2</span>
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-1 text-xs text-amber-800 font-semibold">
                              <AlertCircle className="w-3.5 h-3.5 text-amber-600" />
                              <span>Pending Sync</span>
                            </span>
                          )}
                          {exp.export_status === "withdrawn" && (
                            <Badge variant="danger" size="sm">Withdrawn</Badge>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <div
                          className="font-mono text-[11px] text-gray-600 truncate max-w-[140px]"
                          title={exp.content_hash}
                        >
                          {exp.content_hash.substring(0, 16)}...
                        </div>
                      </td>
                      <td className="px-6 py-4 text-xs text-gray-500">
                        {formatDate(exp.created_at)}
                      </td>
                      <td className="px-6 py-4 text-right space-x-2">
                        <button
                          type="button"
                          onClick={() => handleDownloadExport(exp)}
                          className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-semibold rounded border border-gray-300 text-gray-700 hover:bg-gray-50"
                          title="Stream verified export directly from storage"
                        >
                          <Download className="w-3.5 h-3.5" />
                          <span>Stream</span>
                        </button>

                        {(isSuperAdmin || isNationalAdmin) && (
                          <button
                            type="button"
                            disabled={regeneratingExportId === exp.id}
                            onClick={() => handleRegenerateExport(exp)}
                            className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-semibold rounded border border-emerald-800 text-emerald-800 hover:bg-emerald-50 disabled:opacity-50"
                            title="Regenerate document export from source snapshot"
                          >
                            <RefreshCw
                              className={`w-3.5 h-3.5 ${
                                regeneratingExportId === exp.id ? "animate-spin" : ""
                              }`}
                            />
                            <span>Regenerate</span>
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              <div className="px-6 py-4 border-t border-gray-200 flex items-center justify-between text-xs text-gray-600 bg-gray-50/50">
                <div>
                  Showing {exportsList.length} of {exportsTotal} exports
                </div>
                <div className="flex gap-2">
                  <button
                    disabled={exportsPage <= 1}
                    onClick={() => setExportsPage((p) => Math.max(1, p - 1))}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <button
                    disabled={exportsPage * 10 >= exportsTotal}
                    onClick={() => setExportsPage((p) => p + 1)}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          )}
        </section>
      )}

      {/* =========================================================================
          TAB 5: AUDIT HISTORY
          ========================================================================= */}
      {activeTab === "audit" && (
        <section aria-labelledby="audit-heading" className="space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h2 id="audit-heading" className="text-lg font-bold text-gray-900">
                Reporting & Governance Audit Trail
              </h2>
              <p className="text-xs text-gray-500">
                Immutable record of generation runs, reviews, approvals, publications, and downloads.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <select
                value={auditActionFilter}
                onChange={(e) => {
                  setAuditActionFilter(e.target.value);
                  setAuditsPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter audit records by action type"
              >
                <option value="all">All Actions</option>
                <option value="generate">Generate</option>
                <option value="submit_for_review">Submit for Review</option>
                <option value="approve">Approve</option>
                <option value="publish">Publish</option>
                <option value="withdraw">Withdraw</option>
                <option value="revise">Revise</option>
                <option value="export_pdf">Export PDF</option>
                <option value="export_csv">Export CSV</option>
              </select>

              <select
                value={auditOutcomeFilter}
                onChange={(e) => {
                  setAuditOutcomeFilter(e.target.value);
                  setAuditsPage(1);
                }}
                className="text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                aria-label="Filter audit records by outcome"
              >
                <option value="all">All Outcomes</option>
                <option value="success">Success</option>
                <option value="failed">Failed</option>
              </select>
            </div>
          </div>

          {/* Date range filters */}
          <div className="flex flex-wrap items-center gap-4 bg-gray-50 p-3 rounded-lg border border-gray-200 text-xs">
            <span className="font-semibold text-gray-700">Filter by Date:</span>
            <div className="flex items-center gap-2">
              <label htmlFor="audit-date-from" className="text-gray-600">From:</label>
              <input
                id="audit-date-from"
                type="date"
                value={auditDateFrom}
                onChange={(e) => {
                  setAuditDateFrom(e.target.value);
                  setAuditsPage(1);
                }}
                className="text-xs rounded border-gray-300 shadow-xs"
              />
            </div>
            <div className="flex items-center gap-2">
              <label htmlFor="audit-date-to" className="text-gray-600">To:</label>
              <input
                id="audit-date-to"
                type="date"
                value={auditDateTo}
                onChange={(e) => {
                  setAuditDateTo(e.target.value);
                  setAuditsPage(1);
                }}
                className="text-xs rounded border-gray-300 shadow-xs"
              />
            </div>
            {(auditDateFrom || auditDateTo) && (
              <button
                type="button"
                onClick={() => {
                  setAuditDateFrom("");
                  setAuditDateTo("");
                }}
                className="text-xs text-rose-700 hover:underline"
              >
                Clear Dates
              </button>
            )}
          </div>

          {auditsLoading ? (
            <div className="py-12 flex justify-center">
              <Spinner size="md" text="Loading audit logs..." />
            </div>
          ) : audits.length === 0 ? (
            <EmptyState
              title="No audit entries match criteria"
              description="Administrative actions and scheduler events are captured automatically."
            />
          ) : (
            <div className="overflow-x-auto border border-gray-200 rounded-xl bg-white shadow-xs">
              <table className="min-w-full divide-y divide-gray-200 text-left text-sm">
                <thead className="bg-gray-50 text-xs font-semibold text-gray-600 uppercase tracking-wider">
                  <tr>
                    <th scope="col" className="px-6 py-3">Timestamp</th>
                    <th scope="col" className="px-6 py-3">Action</th>
                    <th scope="col" className="px-6 py-3">Actor</th>
                    <th scope="col" className="px-6 py-3">Report Reference</th>
                    <th scope="col" className="px-6 py-3">Outcome</th>
                    <th scope="col" className="px-6 py-3">Notes / Safe Reason</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {audits.map((a) => {
                    const isSuccess = !(a.action.includes("fail") || (a.details && (a.details as any).outcome === "failed"));
                    const safeReason = (a.details && ((a.details as any).failure_reason || (a.details as any).reason || (a.details as any).notes)) || (typeof a.details === "string" ? a.details : "—");
                    return (
                      <tr key={a.id} className="hover:bg-gray-50/80 transition-colors">
                        <td className="px-6 py-4 text-xs font-mono text-gray-600 whitespace-nowrap">
                          {formatDate(a.created_at)}
                        </td>
                        <td className="px-6 py-4">
                          <span className="font-semibold text-gray-900 text-xs capitalize">
                            {a.action.replace(/_/g, " ")}
                          </span>
                        </td>
                        <td className="px-6 py-4 text-xs">
                          <div className="font-medium text-gray-900">{a.actor_name || "System"}</div>
                          <div className="text-gray-500">{a.actor_email || a.performed_by_id || "Automated Process"}</div>
                        </td>
                        <td className="px-6 py-4 text-xs font-mono text-gray-800">
                          {a.snapshot_id ? a.snapshot_id.substring(0, 8) + "..." : "System Run"}
                        </td>
                        <td className="px-6 py-4">
                          {isSuccess ? (
                            <span className="inline-flex items-center text-xs text-emerald-800 font-bold">
                              SUCCESS
                            </span>
                          ) : (
                            <span className="inline-flex items-center text-xs text-rose-800 font-bold">
                              FAILED
                            </span>
                          )}
                        </td>
                        <td className="px-6 py-4 text-xs text-gray-600 max-w-xs truncate">
                          {safeReason}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>

              <div className="px-6 py-4 border-t border-gray-200 flex items-center justify-between text-xs text-gray-600 bg-gray-50/50">
                <div>
                  Showing {audits.length} of {auditsTotal} audit records
                </div>
                <div className="flex gap-2">
                  <button
                    disabled={auditsPage <= 1}
                    onClick={() => setAuditsPage((p) => Math.max(1, p - 1))}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <button
                    disabled={auditsPage * 15 >= auditsTotal}
                    onClick={() => setAuditsPage((p) => p + 1)}
                    className="p-1.5 border border-gray-300 rounded-md disabled:opacity-40 hover:bg-white"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          )}
        </section>
      )}

      {/* =========================================================================
          MODAL: MANUAL REPORT GENERATION
          ========================================================================= */}
      {showRunModal && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="run-generation-modal-title"
          className="fixed inset-0 z-50 overflow-y-auto bg-gray-900/60 backdrop-blur-xs flex items-center justify-center p-4"
        >
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 shadow-2xl border border-gray-200 space-y-6">
            <div className="flex items-center justify-between border-b border-gray-200 pb-4">
              <div className="flex items-center gap-2">
                <div className="p-2 rounded-lg bg-emerald-50 text-emerald-800">
                  <Play className="w-5 h-5 fill-emerald-800" />
                </div>
                <div>
                  <h3 id="run-generation-modal-title" className="text-lg font-bold text-gray-900">
                    Run Manual Report Generation
                  </h3>
                  <p className="text-xs text-gray-500">
                    Generate official draft report snapshots for historical concluded periods.
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setShowRunModal(false)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-md"
              >
                <XCircle className="w-5 h-5" />
              </button>
            </div>

            {/* Instruction Warning */}
            <Alert type="info" title="Reporting Governance Notice">
              Manual report generation strictly produces <strong>Draft</strong> snapshots for concluded historical periods only. Draft reports require governance review and independent approval before publication.
            </Alert>

            {runError && (
              <Alert type="error" title="Validation / Generation Error">
                {runError}
              </Alert>
            )}

            {runResult ? (
              <div className="space-y-4">
                <div className="p-4 rounded-xl bg-emerald-50 border border-emerald-200">
                  <div className="flex items-center gap-2 text-emerald-900 font-bold">
                    <CheckCircle2 className="w-5 h-5 text-emerald-700" />
                    <span>Generation Run Completed</span>
                  </div>
                  <div className="mt-2 text-sm text-emerald-800 space-y-1">
                    <p>Run ID: <strong className="font-mono text-xs">{runResult.run_identifier}</strong></p>
                    <p>Period: <strong>{runResult.period_label}</strong></p>
                    <p>Generated: <strong>{runResult.reports_generated}</strong> new draft snapshots</p>
                    <p>Skipped (Existing / Up-to-date): <strong>{runResult.reports_skipped}</strong></p>
                    {runResult.reports_failed > 0 && (
                      <p className="text-rose-800">Failed: <strong>{runResult.reports_failed}</strong></p>
                    )}
                  </div>
                </div>

                {runResult.generated_references && runResult.generated_references.length > 0 && (
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-2">
                      Generated References:
                    </h4>
                    <div className="flex flex-wrap gap-1.5">
                      {runResult.generated_references.map((ref) => (
                        <span key={ref} className="font-mono text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-800 border border-gray-200">
                          {ref}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                <div className="pt-2 flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      setShowRunModal(false);
                      setActiveTab("snapshots");
                    }}
                    className="px-4 py-2 rounded-lg bg-emerald-800 text-white font-semibold text-sm hover:bg-emerald-900"
                  >
                    View Generated Snapshots
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleExecuteManualRun} className="space-y-5">
                <div>
                  <label className="block text-xs font-bold uppercase tracking-wider text-gray-700 mb-2">
                    Period Cadence
                  </label>
                  <div className="grid grid-cols-2 gap-3">
                    <button
                      type="button"
                      onClick={() => {
                        setManualPeriodType("monthly");
                        const now = new Date();
                        const prev = new Date(now.getFullYear(), now.getMonth() - 1, 1);
                        setManualPeriodValue(`${prev.getFullYear()}-${String(prev.getMonth() + 1).padStart(2, "0")}`);
                      }}
                      className={`p-3 text-center rounded-lg border text-sm font-semibold transition ${
                        manualPeriodType === "monthly"
                          ? "border-emerald-800 bg-emerald-50 text-emerald-900"
                          : "border-gray-200 hover:bg-gray-50 text-gray-700"
                      }`}
                    >
                      Monthly
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setManualPeriodType("quarterly");
                        const now = new Date();
                        const q = Math.ceil((now.getMonth() + 1) / 3);
                        const prevQ = q === 1 ? 4 : q - 1;
                        const y = q === 1 ? now.getFullYear() - 1 : now.getFullYear();
                        setManualPeriodValue(`${y}-Q${prevQ}`);
                      }}
                      className={`p-3 text-center rounded-lg border text-sm font-semibold transition ${
                        manualPeriodType === "quarterly"
                          ? "border-emerald-800 bg-emerald-50 text-emerald-900"
                          : "border-gray-200 hover:bg-gray-50 text-gray-700"
                      }`}
                    >
                      Quarterly
                    </button>
                  </div>
                </div>

                <div>
                  <label htmlFor="manual-target-period" className="block text-xs font-bold uppercase tracking-wider text-gray-700 mb-1">
                    Concluded Historical Period ({manualPeriodType === "monthly" ? "YYYY-MM" : "YYYY-Q#"})
                  </label>
                  <input
                    id="manual-target-period"
                    type="text"
                    required
                    placeholder={manualPeriodType === "monthly" ? "e.g. 2024-05" : "e.g. 2024-Q2"}
                    value={manualPeriodValue}
                    onChange={(e) => setManualPeriodValue(e.target.value.trim())}
                    className="w-full px-3 py-2 text-sm border border-gray-300 rounded-lg shadow-xs focus:ring-emerald-700 focus:border-emerald-700"
                  />
                  <p className="mt-1 text-[11px] text-gray-500">
                    Strict boundary enforced: Future or ongoing current periods are rejected.
                  </p>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="manual-report-type" className="block text-xs font-bold uppercase tracking-wider text-gray-700 mb-1">
                      Report Category
                    </label>
                    <select
                      id="manual-report-type"
                      value={manualReportType}
                      onChange={(e) => setManualReportType(e.target.value)}
                      className="w-full text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700"
                    >
                      <option value="all">All Available Categories</option>
                      <option value="cleanups">Cleanups</option>
                      <option value="community_projects">Community Projects</option>
                      <option value="safeguarding">Safeguarding</option>
                      <option value="financial_summary">Financial Summary</option>
                    </select>
                  </div>

                  <div>
                    <label htmlFor="manual-scope-type" className="block text-xs font-bold uppercase tracking-wider text-gray-700 mb-1">
                      Geographic Scope
                    </label>
                    <select
                      id="manual-scope-type"
                      value={manualScopeType}
                      onChange={(e) => setManualScopeType(e.target.value)}
                      className="w-full text-xs rounded-lg border-gray-300 shadow-xs focus:ring-emerald-700"
                    >
                      <option value="national">National (All Areas)</option>
                      <option value="zone">Zonal Level</option>
                      <option value="state">State Level</option>
                    </select>
                  </div>
                </div>

                <div className="pt-2">
                  <label className="flex items-start gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={manualConfirmChecked}
                      onChange={(e) => setManualConfirmChecked(e.target.checked)}
                      className="mt-0.5 rounded text-emerald-800 focus:ring-emerald-700"
                    />
                    <span className="text-xs text-gray-600">
                      I confirm that I am initiating an official manual report generation run. Duplicate concluded snapshots will be safely skipped.
                    </span>
                  </label>
                </div>

                <div className="flex items-center justify-end gap-3 pt-4 border-t border-gray-200">
                  <button
                    type="button"
                    onClick={() => setShowRunModal(false)}
                    className="px-4 py-2 rounded-lg border border-gray-300 text-sm font-semibold text-gray-700 hover:bg-gray-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isSubmittingRun || !manualConfirmChecked}
                    className="inline-flex items-center gap-2 px-5 py-2 rounded-lg bg-emerald-800 text-white font-semibold text-sm hover:bg-emerald-900 disabled:opacity-50"
                  >
                    {isSubmittingRun && <Spinner size="sm" />}
                    <span>Execute Generation</span>
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {/* =========================================================================
          MODAL: INSPECT SCHEDULER RUN DETAIL
          ========================================================================= */}
      {selectedRunDetail && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="run-detail-modal-title"
          className="fixed inset-0 z-50 overflow-y-auto bg-gray-900/60 backdrop-blur-xs flex items-center justify-center p-4"
        >
          <div className="bg-white rounded-2xl max-w-2xl w-full p-6 shadow-2xl border border-gray-200 space-y-5">
            <div className="flex items-center justify-between border-b border-gray-200 pb-3">
              <div>
                <h3 id="run-detail-modal-title" className="text-lg font-bold text-gray-900">
                  Scheduler Run Inspection
                </h3>
                <p className="font-mono text-xs text-gray-500">
                  {selectedRunDetail.run_identifier}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setSelectedRunDetail(null)}
                className="text-gray-400 hover:text-gray-600"
              >
                <XCircle className="w-5 h-5" />
              </button>
            </div>

            <div className="grid grid-cols-2 gap-4 text-xs">
              <div className="p-3 rounded-lg bg-gray-50 border border-gray-200">
                <span className="text-gray-500">Reporting Period</span>
                <p className="font-bold text-gray-900 text-sm mt-0.5">
                  {selectedRunDetail.reporting_period}
                </p>
              </div>
              <div className="p-3 rounded-lg bg-gray-50 border border-gray-200">
                <span className="text-gray-500">Execution Status</span>
                <div className="mt-0.5">{getStatusBadge(selectedRunDetail.status)}</div>
              </div>
              <div className="p-3 rounded-lg bg-gray-50 border border-gray-200">
                <span className="text-gray-500">Trigger Source & Actor</span>
                <p className="font-semibold text-gray-800 capitalize mt-0.5">
                  {selectedRunDetail.trigger_source} ({selectedRunDetail.actor_name})
                </p>
              </div>
              <div className="p-3 rounded-lg bg-gray-50 border border-gray-200">
                <span className="text-gray-500">Execution Timestamps</span>
                <p className="text-gray-800 mt-0.5">
                  {formatDate(selectedRunDetail.started_at)}
                </p>
              </div>
            </div>

            {/* Counts Breakdown */}
            <div className="grid grid-cols-4 gap-2 text-center">
              <div className="p-2 rounded bg-gray-50 border border-gray-200">
                <div className="text-xs text-gray-500">Evaluated</div>
                <div className="font-bold text-gray-900 text-base">{selectedRunDetail.reports_evaluated}</div>
              </div>
              <div className="p-2 rounded bg-emerald-50 border border-emerald-200">
                <div className="text-xs text-emerald-800">Generated</div>
                <div className="font-bold text-emerald-950 text-base">+{selectedRunDetail.reports_generated}</div>
              </div>
              <div className="p-2 rounded bg-gray-50 border border-gray-200">
                <div className="text-xs text-gray-600">Skipped</div>
                <div className="font-bold text-gray-800 text-base">{selectedRunDetail.reports_skipped}</div>
              </div>
              <div className="p-2 rounded bg-rose-50 border border-rose-200">
                <div className="text-xs text-rose-800">Failed</div>
                <div className="font-bold text-rose-900 text-base">{selectedRunDetail.reports_failed}</div>
              </div>
            </div>

            {/* Skipped details */}
            {selectedRunDetail.skipped_reasons && Object.keys(selectedRunDetail.skipped_reasons).length > 0 && (
              <div>
                <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700 mb-1">
                  Skipped Reports
                </h4>
                <div className="max-h-32 overflow-y-auto space-y-1 p-2 rounded bg-gray-50 border border-gray-200 text-xs">
                  {Object.entries(selectedRunDetail.skipped_reasons).map(([key, reason]) => (
                    <div key={key} className="flex justify-between text-gray-600">
                      <span className="font-mono text-gray-800">{key}:</span>
                      <span>{reason}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Error classification */}
            {selectedRunDetail.error_details && selectedRunDetail.error_details.length > 0 && (
              <div>
                <h4 className="text-xs font-bold uppercase tracking-wider text-rose-800 mb-1">
                  Safe Error Classifications
                </h4>
                <div className="p-2 rounded bg-rose-50 border border-rose-200 text-xs text-rose-900 space-y-1">
                  {selectedRunDetail.error_details.map((err, i) => (
                    <div key={i}>{err}</div>
                  ))}
                </div>
              </div>
            )}

            <div className="flex justify-end pt-2">
              <button
                type="button"
                onClick={() => setSelectedRunDetail(null)}
                className="px-4 py-2 rounded-lg border border-gray-300 text-sm font-semibold text-gray-700 hover:bg-gray-50"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* =========================================================================
          REPORT CONTENT PREVIEW, INTEGRITY & GOVERNANCE INSPECTION DRAWER
          ========================================================================= */}
      {(inspectingSnapshotId || selectedSnapshot) && (
        <ReportInspectionDrawer
          snapshotId={inspectingSnapshotId || selectedSnapshot?.snapshot_id || ""}
          currentUser={user}
          isSuperAdmin={Boolean(isSuperAdmin)}
          isNationalAdmin={Boolean(isNationalAdmin)}
          onClose={() => {
            setInspectingSnapshotId(null);
            setSelectedSnapshot(null);
            setGovernanceAction(null);
            setGovernanceReason("");
            setGovernanceError(null);
          }}
          onGovernanceActionExecuted={() => {
            fetchSnapshots();
            fetchStats();
          }}
        />
      )}
    </div>
  );
}
