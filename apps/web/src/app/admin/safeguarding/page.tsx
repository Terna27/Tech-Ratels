"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ShieldAlert,
  ShieldCheck,
  AlertTriangle,
  Lock,
  UserCheck,
  UserX,
  FileText,
  Download,
  Send,
  MessageSquare,
  Clock,
  CheckCircle2,
  XCircle,
  Eye,
  Filter,
  Search,
  RefreshCw,
  ArrowRight,
  Shield,
  LifeBuoy,
  Briefcase,
  AlertOctagon,
  Scale,
  Users,
  Calendar,
  MapPin,
  FileCheck,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  SafeguardingCaseListItem,
  SafeguardingCaseDetails,
  SafeguardingAccessLogItem,
  SafeguardingOfficer,
} from "@/lib/types";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";

export default function AdminSafeguardingPage() {
  const router = useRouter();
  const { user, isLoading } = useAuth();

  // Active Tab
  const [activeTab, setActiveTab] = useState<"cases" | "logs" | "officers">("cases");

  // Cases List State
  const [cases, setCases] = useState<SafeguardingCaseListItem[]>([]);
  const [loadingCases, setLoadingCases] = useState(true);
  const [caseError, setCaseError] = useState<string | null>(null);

  // Filters
  const [statusFilter, setStatusFilter] = useState("all");
  const [urgencyFilter, setUrgencyFilter] = useState("all");
  const [minorsOnly, setMinorsOnly] = useState(false);
  const [escalatedOnly, setEscalatedOnly] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  // Selected Case Detail
  const [selectedCaseId, setSelectedCaseId] = useState<string | null>(null);
  const [caseDetails, setCaseDetails] = useState<SafeguardingCaseDetails | null>(null);
  const [loadingDetails, setLoadingDetails] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  // Modals & Action States
  const [showTriageModal, setShowTriageModal] = useState(false);
  const [newUrgency, setNewUrgency] = useState("medium");
  const [triageNotes, setTriageNotes] = useState("");

  const [showAssignModal, setShowAssignModal] = useState(false);
  const [assigneeUserId, setAssigneeUserId] = useState("");
  const [assignedRole, setAssignedRole] = useState("lead_investigator");

  const [showConflictModal, setShowConflictModal] = useState(false);
  const [conflictReason, setConflictReason] = useState("");

  const [showEscalateModal, setShowEscalateModal] = useState(false);
  const [escalationReason, setEscalationReason] = useState("");

  const [showResolveModal, setShowResolveModal] = useState(false);
  const [resolutionSummary, setResolutionSummary] = useState("");
  const [resolveAction, setResolveAction] = useState<"resolve" | "close">("resolve");

  // Messaging & Notes in Detail Drawer
  const [reporterMessage, setReporterMessage] = useState("");
  const [internalNote, setInternalNote] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  // Access Logs State
  const [logs, setLogs] = useState<SafeguardingAccessLogItem[]>([]);
  const [loadingLogs, setLoadingLogs] = useState(false);

  // Officers State
  const [officers, setOfficers] = useState<SafeguardingOfficer[]>([]);
  const [loadingOfficers, setLoadingOfficers] = useState(false);
  const [appointUserId, setAppointUserId] = useState("");
  const [appointRole, setAppointRole] = useState("safeguarding_officer");
  const [appointSuccess, setAppointSuccess] = useState<string | null>(null);

  // Initial Auth & Access Check
  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login?redirect=/admin/safeguarding");
      return;
    }

    if (user) {
      fetchCases();
      fetchOfficers();
    }
  }, [user, isLoading, router]);

  // Fetch Cases with Filters
  const fetchCases = async () => {
    setLoadingCases(true);
    setCaseError(null);
    try {
      const params = new URLSearchParams();
      if (statusFilter !== "all") params.append("status", statusFilter);
      if (urgencyFilter !== "all") params.append("urgency", urgencyFilter);
      if (minorsOnly) params.append("is_minor_involved", "true");
      if (escalatedOnly) params.append("is_escalated", "true");

      const url = `/admin/safeguarding/cases${params.toString() ? `?${params.toString()}` : ""}`;
      const data = await apiRequest<any>(url);
      setCases(Array.isArray(data) ? data : data?.items || []);
    } catch (err: any) {
      setCaseError(err.message || "Failed to load safeguarding cases. You may not possess the required appointment.");
    } finally {
      setLoadingCases(false);
    }
  };

  // Fetch Case Details
  const openCaseDetails = async (caseId: string) => {
    setSelectedCaseId(caseId);
    setLoadingDetails(true);
    setDetailError(null);
    try {
      const data = await apiRequest<SafeguardingCaseDetails>(`/admin/safeguarding/cases/${caseId}`);
      setCaseDetails(data);
      setNewUrgency(data.risk_level || data.urgency || "medium");
      setTriageNotes(data.triage_notes || "");
    } catch (err: any) {
      setDetailError(err.message || "Failed to load case details.");
      setCaseDetails(null);
    } finally {
      setLoadingDetails(false);
    }
  };

  // Fetch Access Logs
  const fetchLogs = async () => {
    setLoadingLogs(true);
    try {
      const data = await apiRequest<any>("/admin/safeguarding/access-logs");
      setLogs(Array.isArray(data) ? data : data?.items || []);
    } catch (err: any) {
      console.error("Failed to load access logs", err);
    } finally {
      setLoadingLogs(false);
    }
  };

  // Fetch Active Officers
  const fetchOfficers = async () => {
    setLoadingOfficers(true);
    try {
      const data = await apiRequest<any>("/admin/safeguarding/officers");
      setOfficers(Array.isArray(data) ? data : data?.items || []);
    } catch (err: any) {
      console.error("Failed to load officers", err);
    } finally {
      setLoadingOfficers(false);
    }
  };

  // Handle Triage Update
  const handleTriageSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCaseId) return;
    setActionLoading(true);
    try {
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/triage`, {
        method: "POST",
        body: JSON.stringify({
          risk_level: newUrgency,
          urgency: newUrgency,
          reason: triageNotes,
          triage_notes: triageNotes,
        }),
      });
      setShowTriageModal(false);
      openCaseDetails(selectedCaseId);
      fetchCases();
    } catch (err: any) {
      alert(err.message || "Failed to update triage.");
    } finally {
      setActionLoading(false);
    }
  };

  // Handle Assignment
  const handleAssignSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCaseId || !assigneeUserId) return;
    setActionLoading(true);
    try {
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/assign`, {
        method: "POST",
        body: JSON.stringify({ officer_user_id: assigneeUserId, role: assignedRole }),
      });
      setShowAssignModal(false);
      setAssigneeUserId("");
      openCaseDetails(selectedCaseId);
      fetchCases();
    } catch (err: any) {
      alert(err.message || "Failed to assign officer.");
    } finally {
      setActionLoading(false);
    }
  };

  // Handle Conflict Declaration
  const handleConflictSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCaseId || !conflictReason.trim()) return;
    setActionLoading(true);
    try {
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/conflict`, {
        method: "POST",
        body: JSON.stringify({ reason: conflictReason.trim() }),
      });
      setShowConflictModal(false);
      setConflictReason("");
      setSelectedCaseId(null);
      setCaseDetails(null);
      fetchCases();
      alert("Conflict of interest successfully recorded. You have been recused from this case.");
    } catch (err: any) {
      alert(err.message || "Failed to declare conflict of interest.");
    } finally {
      setActionLoading(false);
    }
  };

  // Handle Escalation
  const handleEscalateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCaseId || !escalationReason.trim()) return;
    setActionLoading(true);
    try {
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/escalate`, {
        method: "POST",
        body: JSON.stringify({ reason: escalationReason.trim() }),
      });
      setShowEscalateModal(false);
      setEscalationReason("");
      openCaseDetails(selectedCaseId);
      fetchCases();
    } catch (err: any) {
      alert(err.message || "Failed to escalate case.");
    } finally {
      setActionLoading(false);
    }
  };

  // Handle Resolution / Closure
  const handleResolveSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCaseId || !resolutionSummary.trim()) return;
    setActionLoading(true);
    try {
      const endpoint = resolveAction === "resolve" ? "resolve" : "close";
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/${endpoint}`, {
        method: "POST",
        body: JSON.stringify({ resolution_summary: resolutionSummary.trim() }),
      });
      setShowResolveModal(false);
      setResolutionSummary("");
      openCaseDetails(selectedCaseId);
      fetchCases();
    } catch (err: any) {
      alert(err.message || "Failed to update case resolution.");
    } finally {
      setActionLoading(false);
    }
  };

  // Toggle Legal Hold
  const handleToggleLegalHold = async () => {
    if (!selectedCaseId || !caseDetails) return;
    const targetState = !caseDetails.legal_hold;
    const reason = prompt(
      targetState
        ? "Enter justification for placing this case on legal hold:"
        : "Enter reason for lifting the legal hold:"
    );
    if (!reason) return;

    try {
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/legal-hold`, {
        method: "POST",
        body: JSON.stringify({ legal_hold: targetState, reason }),
      });
      openCaseDetails(selectedCaseId);
      fetchCases();
    } catch (err: any) {
      alert(err.message || "Failed to toggle legal hold.");
    }
  };

  // Send Reporter Message
  const handleSendReporterMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCaseId || !reporterMessage.trim()) return;
    setActionLoading(true);
    try {
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/reporter-message`, {
        method: "POST",
        body: JSON.stringify({ content: reporterMessage.trim(), message: reporterMessage.trim() }),
      });
      setReporterMessage("");
      openCaseDetails(selectedCaseId);
      setActionSuccess("Message dispatched to reporter.");
      setTimeout(() => setActionSuccess(null), 3000);
    } catch (err: any) {
      alert(err.message || "Failed to send message to reporter.");
    } finally {
      setActionLoading(false);
    }
  };

  // Add Internal Note
  const handleAddInternalNote = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCaseId || !internalNote.trim()) return;
    setActionLoading(true);
    try {
      await apiRequest(`/admin/safeguarding/cases/${selectedCaseId}/notes`, {
        method: "POST",
        body: JSON.stringify({ note: internalNote.trim() }),
      });
      setInternalNote("");
      openCaseDetails(selectedCaseId);
      setActionSuccess("Internal note recorded in audit log.");
      setTimeout(() => setActionSuccess(null), 3000);
    } catch (err: any) {
      alert(err.message || "Failed to record internal note.");
    } finally {
      setActionLoading(false);
    }
  };

  // Appoint Officer
  const handleAppointOfficer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!appointUserId.trim()) return;
    try {
      await apiRequest("/admin/safeguarding/officers", {
        method: "POST",
        body: JSON.stringify({ user_id: appointUserId.trim(), role: appointRole }),
      });
      setAppointUserId("");
      setAppointSuccess("New safeguarding officer appointed successfully.");
      setTimeout(() => setAppointSuccess(null), 4000);
      fetchOfficers();
    } catch (err: any) {
      alert(err.message || "Failed to appoint officer.");
    }
  };

  // Presigned Download Evidence Helper
  const downloadEvidence = async (evidenceId: string) => {
    if (!selectedCaseId) return;
    try {
      const res = await apiRequest<{ download_url: string }>(
        `/admin/safeguarding/cases/${selectedCaseId}/evidence/${evidenceId}/download`
      );
      window.open(res.download_url, "_blank");
    } catch (err: any) {
      alert(err.message || "Unable to generate download URL.");
    }
  };

  // Render Badge Helper
  const getUrgencyBadge = (urgency: string) => {
    switch (urgency) {
      case "urgent":
        return <Badge variant="danger">Urgent</Badge>;
      case "high":
        return <Badge variant="warning">High Priority</Badge>;
      case "medium":
        return <Badge variant="neutral">Medium</Badge>;
      case "low":
        return <Badge variant="neutral">Low</Badge>;
      default:
        return <Badge variant="neutral">{urgency}</Badge>;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "submitted":
        return <Badge variant="neutral">Submitted</Badge>;
      case "triaged":
        return <Badge variant="warning">Triaged</Badge>;
      case "under_investigation":
        return <Badge variant="primary">Under Investigation</Badge>;
      case "action_taken":
        return <Badge variant="primary">Action Taken</Badge>;
      case "resolved":
        return <Badge variant="success">Resolved</Badge>;
      case "closed":
        return <Badge variant="neutral">Closed</Badge>;
      default:
        return <Badge variant="neutral">{status}</Badge>;
    }
  };

  // Filtered cases by search query
  const filteredCases = cases.filter((c) => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    const ref = (c.public_reference || c.reference_number || "").toLowerCase();
    const cat = (c.category || "").toLowerCase();
    const state = (c.state_name || "").toLowerCase();
    return ref.includes(q) || cat.includes(q) || state.includes(q);
  });

  if (isLoading) return <Spinner text="Authenticating safeguarding officer..." />;
  if (!user) return null;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Top Header & Role Jurisdiction Context */}
      <div className="bg-emerald-950 text-white rounded-3xl p-6 sm:p-8 shadow-sm flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border border-emerald-800">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="p-1.5 bg-emerald-800 rounded-md text-emerald-300">
              <ShieldAlert className="w-4 h-4" />
            </span>
            <span className="text-xs uppercase font-extrabold tracking-wider text-emerald-400">
              Restricted Confidential Module
            </span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold">
            Safeguarding & Incident Case Management
          </h1>
          <p className="text-xs text-emerald-200">
            Confidential duty-of-care workspace. Access restricted to authorized Safeguarding Officers and Lead Trustees.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Link
            href="/admin"
            className="px-4 py-2 bg-emerald-900 hover:bg-emerald-800 text-white text-xs font-semibold rounded-lg transition"
          >
            ← Admin Overview
          </Link>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="flex items-center gap-2 border-b border-gray-200 pb-3">
        <button
          type="button"
          onClick={() => setActiveTab("cases")}
          className={`px-4 py-2 rounded-xl text-xs font-bold transition flex items-center gap-2 ${
            activeTab === "cases"
              ? "bg-emerald-800 text-white shadow-sm"
              : "bg-white text-gray-600 hover:bg-gray-100"
          }`}
        >
          <FileText className="w-4 h-4" />
          Incident Cases ({cases.length})
        </button>

        <button
          type="button"
          onClick={() => {
            setActiveTab("logs");
            fetchLogs();
          }}
          className={`px-4 py-2 rounded-xl text-xs font-bold transition flex items-center gap-2 ${
            activeTab === "logs"
              ? "bg-emerald-800 text-white shadow-sm"
              : "bg-white text-gray-600 hover:bg-gray-100"
          }`}
        >
          <Lock className="w-4 h-4" />
          Access Audit Logs
        </button>

        <button
          type="button"
          onClick={() => {
            setActiveTab("officers");
            fetchOfficers();
          }}
          className={`px-4 py-2 rounded-xl text-xs font-bold transition flex items-center gap-2 ${
            activeTab === "officers"
              ? "bg-emerald-800 text-white shadow-sm"
              : "bg-white text-gray-600 hover:bg-gray-100"
          }`}
        >
          <Users className="w-4 h-4" />
          Safeguarding Officers ({officers.length})
        </button>
      </div>

      {caseError && (
        <div className="p-6 bg-red-50 border border-red-200 rounded-2xl text-red-950 space-y-2">
          <div className="font-bold flex items-center gap-2 text-sm text-red-900">
            <AlertTriangle className="w-4 h-4 text-red-600" />
            Access Restricted (Safeguarding RBAC)
          </div>
          <p className="text-xs leading-relaxed">
            {caseError}
          </p>
          <p className="text-xs text-red-800">
            General administrators and state coordinators do not have permissions to view confidential incident reports.
          </p>
        </div>
      )}

      {/* ============================================================== */}
      {/* CASES TAB */}
      {/* ============================================================== */}
      {activeTab === "cases" && (
        <div className="space-y-6">
          {/* Filter Bar */}
          <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-4 gap-4">
              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-gray-500 mb-1">
                  Search Reference
                </label>
                <div className="relative">
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="CASE-2026-..."
                    className="w-full pl-8 pr-3 py-2 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                  />
                  <Search className="w-4 h-4 text-gray-400 absolute left-2.5 top-2.5" />
                </div>
              </div>

              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-gray-500 mb-1">
                  Status
                </label>
                <select
                  value={statusFilter}
                  onChange={(e) => {
                    setStatusFilter(e.target.value);
                    setTimeout(fetchCases, 50);
                  }}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg bg-white"
                >
                  <option value="all">All Statuses</option>
                  <option value="submitted">Submitted</option>
                  <option value="triaged">Triaged</option>
                  <option value="under_investigation">Under Investigation</option>
                  <option value="action_taken">Action Taken</option>
                  <option value="resolved">Resolved</option>
                  <option value="closed">Closed</option>
                </select>
              </div>

              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-gray-500 mb-1">
                  Urgency
                </label>
                <select
                  value={urgencyFilter}
                  onChange={(e) => {
                    setUrgencyFilter(e.target.value);
                    setTimeout(fetchCases, 50);
                  }}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg bg-white"
                >
                  <option value="all">All Urgency Levels</option>
                  <option value="urgent">Urgent</option>
                  <option value="high">High</option>
                  <option value="medium">Medium</option>
                  <option value="low">Low</option>
                </select>
              </div>

              <div className="flex items-center gap-4 pt-4 sm:pt-6">
                <label className="flex items-center gap-2 cursor-pointer text-xs font-semibold text-gray-700">
                  <input
                    type="checkbox"
                    checked={minorsOnly}
                    onChange={(e) => {
                      setMinorsOnly(e.target.checked);
                      setTimeout(fetchCases, 50);
                    }}
                    className="h-4 w-4 text-emerald-700 rounded"
                  />
                  <span>Minors Involved</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer text-xs font-semibold text-gray-700">
                  <input
                    type="checkbox"
                    checked={escalatedOnly}
                    onChange={(e) => {
                      setEscalatedOnly(e.target.checked);
                      setTimeout(fetchCases, 50);
                    }}
                    className="h-4 w-4 text-emerald-700 rounded"
                  />
                  <span>Escalated Only</span>
                </label>
              </div>
            </div>
          </div>

          {/* Cases Table */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
            {loadingCases ? (
              <div className="py-12">
                <Spinner text="Loading cases..." />
              </div>
            ) : filteredCases.length === 0 ? (
              <div className="text-center py-12 text-xs text-gray-500 space-y-2">
                <Shield className="w-8 h-8 text-gray-300 mx-auto" />
                <p>No safeguarding cases match your current filter selection.</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-bold text-[10px]">
                      <th className="py-3 px-4">Reference</th>
                      <th className="py-3 px-4">Category</th>
                      <th className="py-3 px-4">Urgency</th>
                      <th className="py-3 px-4">Status</th>
                      <th className="py-3 px-4">Jurisdiction</th>
                      <th className="py-3 px-4">Protocols</th>
                      <th className="py-3 px-4">Submitted</th>
                      <th className="py-3 px-4 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100 font-medium text-gray-800">
                    {filteredCases.map((c) => (
                      <tr key={c.id} className="hover:bg-gray-50 transition">
                        <td className="py-3 px-4 font-mono font-bold text-gray-900">
                          {c.public_reference || c.reference_number}
                        </td>
                        <td className="py-3 px-4 capitalize">
                          {c.category.replace(/_/g, " ")}
                        </td>
                        <td className="py-3 px-4">
                          {getUrgencyBadge(c.risk_level || c.urgency || "medium")}
                        </td>
                        <td className="py-3 px-4">
                          {getStatusBadge(c.status)}
                        </td>
                        <td className="py-3 px-4 text-gray-600">
                          {c.state_name || "National / Unspecified"}
                          {c.lga_name && ` (${c.lga_name})`}
                        </td>
                        <td className="py-3 px-4">
                          <div className="flex flex-wrap gap-1">
                            {c.is_minor_involved && (
                              <span className="px-1.5 py-0.5 bg-amber-100 text-amber-900 rounded font-bold text-[10px]">
                                Minor
                              </span>
                            )}
                            {c.is_anonymous ? (
                              <span className="px-1.5 py-0.5 bg-gray-100 text-gray-700 rounded text-[10px]">
                                Anonymous
                              </span>
                            ) : (
                              <span className="px-1.5 py-0.5 bg-emerald-50 text-emerald-800 rounded text-[10px]">
                                Identified
                              </span>
                            )}
                            {c.is_escalated && (
                              <span className="px-1.5 py-0.5 bg-purple-100 text-purple-900 rounded font-bold text-[10px]">
                                Escalated
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="py-3 px-4 text-gray-500">
                          {new Date(c.created_at).toLocaleDateString()}
                        </td>
                        <td className="py-3 px-4 text-right">
                          <button
                            type="button"
                            onClick={() => openCaseDetails(c.id)}
                            className="px-3 py-1.5 bg-emerald-800 hover:bg-emerald-700 text-white rounded-lg font-bold text-[11px] transition"
                          >
                            Inspect Case
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ============================================================== */}
      {/* ACCESS AUDIT LOGS TAB */}
      {/* ============================================================== */}
      {activeTab === "logs" && (
        <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden space-y-4 p-6">
          <div className="flex items-center justify-between pb-3 border-b border-gray-100">
            <div>
              <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
                <Lock className="w-5 h-5 text-emerald-700" />
                Immutable Safeguarding Access Audit Logs
              </h2>
              <p className="text-xs text-gray-500">
                Independent audit record of all case reads, decryption events, triage modifications, and assignments.
              </p>
            </div>
            <button
              type="button"
              onClick={fetchLogs}
              className="p-2 border border-gray-300 hover:bg-gray-50 rounded-lg text-gray-600 transition"
              title="Refresh logs"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
          </div>

          {loadingLogs ? (
            <Spinner text="Loading access logs..." />
          ) : logs.length === 0 ? (
            <div className="text-center py-8 text-xs text-gray-400">
              No access logs recorded yet.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-bold text-[10px]">
                    <th className="py-2.5 px-4">Timestamp</th>
                    <th className="py-2.5 px-4">Officer / Actor</th>
                    <th className="py-2.5 px-4">Access Type</th>
                    <th className="py-2.5 px-4">Case ID</th>
                    <th className="py-2.5 px-4">IP Address</th>
                    <th className="py-2.5 px-4">Reason / Context</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {logs.map((log) => (
                    <tr key={log.id} className="hover:bg-gray-50">
                      <td className="py-2.5 px-4 text-gray-500">
                        {new Date(log.created_at).toLocaleString()}
                      </td>
                      <td className="py-2.5 px-4 font-bold text-gray-900">
                        {log.actor_name}
                      </td>
                      <td className="py-2.5 px-4">
                        <span className="font-mono text-emerald-800 bg-emerald-50 px-2 py-0.5 rounded">
                          {log.access_type}
                        </span>
                      </td>
                      <td className="py-2.5 px-4 font-mono text-gray-600 truncate max-w-xs">
                        {log.case_id || "Global Vault"}
                      </td>
                      <td className="py-2.5 px-4 font-mono text-gray-500">
                        {log.ip_address || "Internal"}
                      </td>
                      <td className="py-2.5 px-4 text-gray-700">
                        {log.access_reason}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* ============================================================== */}
      {/* OFFICERS DIRECTORY TAB */}
      {/* ============================================================== */}
      {activeTab === "officers" && (
        <div className="space-y-6">
          {/* Appoint Officer Box (Super Admin / Lead Officer) */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
            <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
              <UserCheck className="w-5 h-5 text-emerald-700" />
              Appoint Safeguarding Officer
            </h2>
            <p className="text-xs text-gray-500">
              Grant a verified user safeguarding clearance. Appointees gain authorization to review confidential narratives and conduct inquiries.
            </p>

            {appointSuccess && <Alert type="success">{appointSuccess}</Alert>}

            <form onSubmit={handleAppointOfficer} className="grid grid-cols-1 sm:grid-cols-3 gap-4 pt-2">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  User UUID <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  required
                  value={appointUserId}
                  onChange={(e) => setAppointUserId(e.target.value)}
                  placeholder="e.g. 550e8400-e29b-41d4-a716-446655440000"
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg font-mono focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Safeguarding Role
                </label>
                <select
                  value={appointRole}
                  onChange={(e) => setAppointRole(e.target.value)}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg bg-white"
                >
                  <option value="safeguarding_officer">Safeguarding Officer</option>
                  <option value="lead_safeguarding_officer">Lead Safeguarding Officer</option>
                  <option value="independent_review_officer">Independent Review Officer</option>
                  <option value="trustee_observer">Trustee Observer</option>
                </select>
              </div>

              <div className="flex items-end">
                <button
                  type="submit"
                  className="w-full py-2.5 bg-emerald-800 hover:bg-emerald-700 text-white rounded-lg font-bold text-xs uppercase tracking-wider transition"
                >
                  Appoint Officer
                </button>
              </div>
            </form>
          </div>

          {/* Officers Table */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden p-6 space-y-4">
            <h3 className="text-sm font-bold text-gray-900">
              Active Safeguarding Officers Directory
            </h3>

            {loadingOfficers ? (
              <Spinner text="Loading officers..." />
            ) : officers.length === 0 ? (
              <div className="text-center py-6 text-xs text-gray-400">
                No safeguarding officers appointed yet.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="bg-gray-50 border-b border-gray-200 text-gray-500 uppercase tracking-wider font-bold text-[10px]">
                      <th className="py-2.5 px-4">Officer Name</th>
                      <th className="py-2.5 px-4">Email</th>
                      <th className="py-2.5 px-4">Role</th>
                      <th className="py-2.5 px-4">Appointed At</th>
                      <th className="py-2.5 px-4">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {officers.map((o) => (
                      <tr key={o.id} className="hover:bg-gray-50">
                        <td className="py-2.5 px-4 font-bold text-gray-900">
                          {o.display_name}
                        </td>
                        <td className="py-2.5 px-4 text-gray-600 font-mono">
                          {o.email}
                        </td>
                        <td className="py-2.5 px-4 capitalize font-semibold text-emerald-800">
                          {((o as any).role_title || o.officer_role || "Officer").replace(/_/g, " ")}
                        </td>
                        <td className="py-2.5 px-4 text-gray-500">
                          {new Date(o.appointed_at).toLocaleDateString()}
                        </td>
                        <td className="py-2.5 px-4">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                              o.is_active
                                ? "bg-emerald-100 text-emerald-800"
                                : "bg-gray-100 text-gray-600"
                            }`}
                          >
                            {o.is_active ? "Active" : "Inactive"}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ============================================================== */}
      {/* CASE DETAIL DRAWER / MODAL */}
      {/* ============================================================== */}
      {selectedCaseId && (
        <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex justify-end animate-in fade-in duration-200">
          <div className="bg-white w-full max-w-3xl h-full shadow-2xl overflow-y-auto flex flex-col">
            {/* Drawer Header */}
            <div className="p-6 bg-emerald-950 text-white flex items-center justify-between sticky top-0 z-10">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="text-[10px] font-mono uppercase tracking-wider text-emerald-400">
                    Confidential File
                  </span>
                  {caseDetails && getStatusBadge(caseDetails.status)}
                  {caseDetails && getUrgencyBadge(caseDetails.risk_level || caseDetails.urgency || "medium")}
                </div>
                <h2 className="text-xl font-mono font-black">
                  {caseDetails ? (caseDetails.public_reference || caseDetails.reference_number) : "Loading Case..."}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => {
                  setSelectedCaseId(null);
                  setCaseDetails(null);
                }}
                className="p-2 hover:bg-emerald-800 rounded-lg text-white transition"
              >
                ✕ Close
              </button>
            </div>

            {loadingDetails ? (
              <div className="p-12">
                <Spinner text="Decrypting confidential case file..." />
              </div>
            ) : detailError ? (
              <div className="p-8 space-y-4">
                <Alert type="error">{detailError}</Alert>
                <button
                  type="button"
                  onClick={() => setSelectedCaseId(null)}
                  className="px-4 py-2 border border-gray-300 rounded-lg text-xs"
                >
                  Return to Case Queue
                </button>
              </div>
            ) : caseDetails ? (
              <div className="p-6 space-y-6 flex-1 text-xs">
                {actionSuccess && <Alert type="success">{actionSuccess}</Alert>}

                {/* Legal Hold / Escalated Banner */}
                {caseDetails.legal_hold && (
                  <div className="p-4 bg-red-100 border border-red-300 rounded-xl text-red-950 flex items-center justify-between">
                    <div className="flex items-center gap-2 font-bold">
                      <Scale className="w-5 h-5 text-red-700" />
                      <span>LEGAL HOLD ACTIVE: Automatic retention purge frozen for statutory inquiry</span>
                    </div>
                    <button
                      type="button"
                      onClick={handleToggleLegalHold}
                      className="px-2.5 py-1 bg-white border border-red-300 rounded text-[11px] font-bold text-red-900"
                    >
                      Lift Hold
                    </button>
                  </div>
                )}

                {caseDetails.is_escalated && (
                  <div className="p-4 bg-purple-50 border border-purple-200 rounded-xl text-purple-950 space-y-1">
                    <div className="font-bold flex items-center gap-1.5">
                      <AlertOctagon className="w-4 h-4 text-purple-700" />
                      Case Escalated to Independent Trustees / Board
                    </div>
                    <p className="text-[11px] text-purple-800">
                      Reason: {caseDetails.escalation_reason || "Escalation requested by investigating officer."}
                    </p>
                  </div>
                )}

                {/* Action Buttons Toolbar */}
                <div className="flex flex-wrap gap-2 p-3 bg-gray-50 border border-gray-200 rounded-xl">
                  <button
                    type="button"
                    onClick={() => setShowTriageModal(true)}
                    className="px-3 py-1.5 bg-white border border-gray-300 hover:bg-gray-100 rounded-lg font-bold text-gray-800 transition"
                  >
                    Triage & Priority
                  </button>

                  <button
                    type="button"
                    onClick={() => setShowAssignModal(true)}
                    className="px-3 py-1.5 bg-white border border-gray-300 hover:bg-gray-100 rounded-lg font-bold text-gray-800 transition"
                  >
                    Assign Officer
                  </button>

                  <button
                    type="button"
                    onClick={() => setShowResolveModal(true)}
                    className="px-3 py-1.5 bg-emerald-800 hover:bg-emerald-700 text-white rounded-lg font-bold transition"
                  >
                    Resolve / Close Case
                  </button>

                  <button
                    type="button"
                    onClick={() => setShowEscalateModal(true)}
                    className="px-3 py-1.5 bg-purple-100 hover:bg-purple-200 text-purple-900 rounded-lg font-bold transition"
                  >
                    Escalate
                  </button>

                  {!caseDetails.legal_hold && (
                    <button
                      type="button"
                      onClick={handleToggleLegalHold}
                      className="px-3 py-1.5 bg-amber-50 hover:bg-amber-100 text-amber-900 border border-amber-300 rounded-lg font-bold transition"
                    >
                      Place Legal Hold
                    </button>
                  )}

                  <button
                    type="button"
                    onClick={() => setShowConflictModal(true)}
                    className="px-3 py-1.5 bg-red-50 hover:bg-red-100 text-red-800 border border-red-300 rounded-lg font-bold transition ml-auto"
                  >
                    Declare Conflict of Interest
                  </button>
                </div>

                {/* Case Meta Details Grid */}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                  <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                    <span className="text-gray-500 block">Category</span>
                    <span className="font-bold text-gray-900 capitalize block">
                      {caseDetails.category.replace(/_/g, " ")}
                    </span>
                  </div>

                  <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                    <span className="text-gray-500 block">Minor Safeguarding</span>
                    <span className="font-bold text-gray-900 block">
                      {caseDetails.is_minor_involved ? "Yes (7-Year Hold)" : "No"}
                    </span>
                  </div>

                  <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                    <span className="text-gray-500 block">State / LGA</span>
                    <span className="font-bold text-gray-900 block">
                      {caseDetails.state_name || "Unspecified"}
                      {caseDetails.lga_name && ` (${caseDetails.lga_name})`}
                    </span>
                  </div>

                  <div className="p-3 bg-gray-50 rounded-xl space-y-1">
                    <span className="text-gray-500 block">Submitted</span>
                    <span className="font-bold text-gray-900 block">
                      {new Date(caseDetails.created_at).toLocaleDateString()}
                    </span>
                  </div>
                </div>

                {/* Reporter Details (Encrypted Vault) */}
                <div className="p-4 bg-gray-50 border border-gray-200 rounded-xl space-y-2">
                  <div className="text-[11px] font-bold text-gray-700 uppercase tracking-wider flex items-center gap-1.5">
                    {caseDetails.is_anonymous ? <UserX className="w-4 h-4 text-gray-400" /> : <UserCheck className="w-4 h-4 text-emerald-700" />}
                    Reporter Information
                  </div>
                  {caseDetails.is_anonymous ? (
                    <p className="text-gray-500 italic">
                      This incident was filed completely anonymously. No names, email addresses, or phone numbers were collected.
                    </p>
                  ) : (
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                      <div>
                        <span className="text-gray-500 block">Name:</span>
                        <span className="font-bold text-gray-800">{caseDetails.reporter?.name || caseDetails.reporter_info?.name || "N/A"}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 block">Email:</span>
                        <span className="font-bold text-gray-800 font-mono">{caseDetails.reporter?.email || caseDetails.reporter_info?.email || "N/A"}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 block">Phone:</span>
                        <span className="font-bold text-gray-800 font-mono">{caseDetails.reporter?.phone || caseDetails.reporter_info?.phone || "N/A"}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 block">Relationship:</span>
                        <span className="font-bold text-gray-800 capitalize">{caseDetails.reporter?.relationship || caseDetails.reporter_info?.relationship || caseDetails.reporter?.contact_preference || "N/A"}</span>
                      </div>
                    </div>
                  )}
                </div>

                {/* Decrypted Narrative */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="text-xs font-bold text-gray-900 uppercase tracking-wider flex items-center gap-1.5">
                      <Lock className="w-4 h-4 text-emerald-700" />
                      Decrypted Incident Narrative
                    </div>
                    <span className="text-[10px] text-gray-400 font-mono">AES-256-GCM Verified</span>
                  </div>
                  <div className="p-4 bg-amber-50/40 border border-amber-200 rounded-xl text-gray-900 font-sans leading-relaxed whitespace-pre-wrap select-text">
                    {caseDetails.decrypted_description || caseDetails.narrative_decrypted}
                  </div>
                </div>

                {/* Evidence Files */}
                <div className="space-y-2">
                  <div className="text-xs font-bold text-gray-900 uppercase tracking-wider">
                    Supporting Evidence ({caseDetails.evidence ? caseDetails.evidence.length : 0})
                  </div>
                  {!caseDetails.evidence || caseDetails.evidence.length === 0 ? (
                    <p className="text-gray-400 italic">No evidence files were attached to this report.</p>
                  ) : (
                    <div className="space-y-1.5">
                      {caseDetails.evidence.map((ev) => (
                        <div
                          key={ev.id}
                          className="flex items-center justify-between p-3 bg-gray-50 border border-gray-200 rounded-xl"
                        >
                          <div className="space-y-0.5 truncate pr-2">
                            <span className="font-bold text-gray-800 block truncate">{ev.file_name || ev.original_filename}</span>
                            <div className="text-[10px] text-gray-400 font-mono flex items-center gap-2">
                              <span>{(ev.file_size_bytes / 1024 / 1024).toFixed(2)} MB</span>
                              <span>•</span>
                              <span>SHA256: {ev.sha256_hash.substring(0, 16)}...</span>
                            </div>
                          </div>
                          <button
                            type="button"
                            onClick={() => downloadEvidence(ev.id)}
                            className="inline-flex items-center gap-1 px-3 py-1.5 bg-emerald-800 hover:bg-emerald-700 text-white rounded-lg font-bold text-[11px] transition shrink-0"
                          >
                            <Download className="w-3.5 h-3.5" />
                            Download
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Assigned Officers */}
                <div className="space-y-2">
                  <div className="text-xs font-bold text-gray-900 uppercase tracking-wider">
                    Assigned Investigation Team
                  </div>
                  {!caseDetails.assignments || caseDetails.assignments.length === 0 ? (
                    <p className="text-gray-400 italic">No investigating officers currently assigned.</p>
                  ) : (
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      {caseDetails.assignments.map((asgn) => (
                        <div key={asgn.id} className="p-3 bg-gray-50 border border-gray-200 rounded-xl">
                          <span className="font-bold text-gray-800 block">{asgn.officer_name}</span>
                          <span className="text-[10px] text-emerald-800 uppercase font-semibold">
                            {(asgn.assigned_role || asgn.status || "Investigator").replace(/_/g, " ")}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Two Tabs: Reporter Messaging & Internal Notes */}
                <div className="border-t border-gray-200 pt-4 space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    {/* Reporter Messaging Thread */}
                    <div className="space-y-3">
                      <div className="font-bold text-xs text-gray-900 uppercase tracking-wider flex items-center gap-1.5">
                        <MessageSquare className="w-4 h-4 text-emerald-700" />
                        Communication with Reporter
                      </div>

                      <div className="space-y-2 max-h-60 overflow-y-auto p-2 bg-gray-50 rounded-xl border border-gray-200">
                        {!caseDetails.messages || caseDetails.messages.length === 0 ? (
                          <div className="text-center py-4 text-gray-400 text-[11px]">
                            No messages with reporter.
                          </div>
                        ) : (
                          caseDetails.messages.map((m) => {
                            const isOfficer = m.sender_role === "safeguarding_officer" || m.author_type === "officer";
                            return (
                              <div
                                key={m.id}
                                className={`p-2.5 rounded-lg text-[11px] space-y-1 ${
                                  isOfficer
                                    ? "bg-emerald-100 text-emerald-950 font-medium"
                                    : "bg-white text-gray-800 border border-gray-200"
                                }`}
                              >
                                <div className="font-bold text-[9px] uppercase tracking-wider">
                                  {isOfficer ? "Safeguarding Team" : "Reporter"}
                                </div>
                                <p className="whitespace-pre-wrap">{m.content || m.message}</p>
                                <div className="text-[8px] text-gray-400 text-right">
                                  {new Date(m.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                                </div>
                              </div>
                            );
                          })
                        )}
                      </div>

                      <form onSubmit={handleSendReporterMessage} className="space-y-2">
                        <textarea
                          rows={2}
                          value={reporterMessage}
                          onChange={(e) => setReporterMessage(e.target.value)}
                          placeholder="Type response or inquiry to reporter..."
                          className="w-full p-2.5 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                        />
                        <button
                          type="submit"
                          disabled={actionLoading || !reporterMessage.trim()}
                          className="w-full py-2 bg-emerald-800 hover:bg-emerald-700 disabled:bg-gray-300 text-white rounded-lg font-bold text-xs transition"
                        >
                          Send Message to Reporter
                        </button>
                      </form>
                    </div>

                    {/* Internal Investigation Notes */}
                    <div className="space-y-3">
                      <div className="font-bold text-xs text-gray-900 uppercase tracking-wider flex items-center gap-1.5">
                        <FileCheck className="w-4 h-4 text-purple-700" />
                        Internal Case Notes (Officers Only)
                      </div>

                      <div className="space-y-2 max-h-60 overflow-y-auto p-2 bg-gray-50 rounded-xl border border-gray-200">
                        {caseDetails.triage_notes && (
                          <div className="p-2.5 bg-amber-50 border border-amber-200 rounded-lg text-[11px] space-y-1 text-amber-900">
                            <span className="font-bold text-[9px] uppercase block">Triage Notes</span>
                            <p>{caseDetails.triage_notes}</p>
                          </div>
                        )}
                        {!caseDetails.actions || caseDetails.actions.length === 0 ? (
                          <div className="text-center py-4 text-gray-400 text-[11px]">
                            No internal action records.
                          </div>
                        ) : (
                          caseDetails.actions.map((act) => (
                            <div key={act.id} className="p-2.5 bg-white border border-gray-200 rounded-lg text-[11px] space-y-1">
                              <div className="flex justify-between items-center text-[9px] text-gray-400">
                                <span className="font-bold text-gray-700 uppercase">{act.actor_name || "System"}</span>
                                <span>{new Date(act.created_at).toLocaleDateString()}</span>
                              </div>
                              <p className="font-medium text-gray-800">{act.reason || act.summary}</p>
                            </div>
                          ))
                        )}
                      </div>

                      <form onSubmit={handleAddInternalNote} className="space-y-2">
                        <textarea
                          rows={2}
                          value={internalNote}
                          onChange={(e) => setInternalNote(e.target.value)}
                          placeholder="Record private internal note or interview record..."
                          className="w-full p-2.5 text-xs border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-600 focus:outline-none"
                        />
                        <button
                          type="submit"
                          disabled={actionLoading || !internalNote.trim()}
                          className="w-full py-2 bg-gray-800 hover:bg-gray-700 disabled:bg-gray-300 text-white rounded-lg font-bold text-xs transition"
                        >
                          Save Internal Note
                        </button>
                      </form>
                    </div>
                  </div>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      )}

      {/* ============================================================== */}
      {/* ACTION MODALS */}
      {/* ============================================================== */}

      {/* Triage Modal */}
      {showTriageModal && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-gray-900 text-base">Triage & Priority Update</h3>
            <form onSubmit={handleTriageSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Urgency Level
                </label>
                <select
                  value={newUrgency}
                  onChange={(e) => setNewUrgency(e.target.value)}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg bg-white"
                >
                  <option value="urgent">Urgent (Immediate attention)</option>
                  <option value="high">High</option>
                  <option value="medium">Medium</option>
                  <option value="low">Low</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Internal Triage Notes
                </label>
                <textarea
                  rows={3}
                  value={triageNotes}
                  onChange={(e) => setTriageNotes(e.target.value)}
                  placeholder="Justification for priority assessment..."
                  className="w-full p-2.5 text-xs border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowTriageModal(false)}
                  className="px-4 py-2 border border-gray-300 rounded-lg text-xs"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading}
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white rounded-lg font-bold text-xs"
                >
                  Update Triage
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Assign Modal */}
      {showAssignModal && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-gray-900 text-base">Assign Officer to Case</h3>
            <form onSubmit={handleAssignSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Select Safeguarding Officer
                </label>
                <select
                  required
                  value={assigneeUserId}
                  onChange={(e) => setAssigneeUserId(e.target.value)}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg bg-white"
                >
                  <option value="">Select an officer...</option>
                  {officers.map((o) => (
                    <option key={o.user_id || o.id} value={o.user_id || o.id}>
                      {o.display_name} ({((o as any).role_title || o.officer_role || "Officer").replace(/_/g, " ")})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Assigned Case Role
                </label>
                <select
                  value={assignedRole}
                  onChange={(e) => setAssignedRole(e.target.value)}
                  className="w-full px-3 py-2 text-xs border border-gray-300 rounded-lg bg-white"
                >
                  <option value="lead_investigator">Lead Investigator</option>
                  <option value="reviewing_officer">Reviewing Officer</option>
                  <option value="observer">Observer</option>
                </select>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAssignModal(false)}
                  className="px-4 py-2 border border-gray-300 rounded-lg text-xs"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading || !assigneeUserId}
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white rounded-lg font-bold text-xs"
                >
                  Confirm Assignment
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Conflict Modal */}
      {showConflictModal && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-red-950 text-base flex items-center gap-2">
              <AlertTriangle className="w-5 h-5 text-red-600" />
              Declare Conflict of Interest (Self-Recusal)
            </h3>
            <p className="text-xs text-gray-600">
              If you have a personal, familial, or managerial connection with any person named in this report, or if you witnessed or are subject of the incident, you must declare it. You will be permanently recused from viewing or handling this case.
            </p>
            <form onSubmit={handleConflictSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Reason for Recusal <span className="text-red-500">*</span>
                </label>
                <textarea
                  rows={3}
                  required
                  value={conflictReason}
                  onChange={(e) => setConflictReason(e.target.value)}
                  placeholder="e.g. Named party is my direct report / family relation..."
                  className="w-full p-2.5 text-xs border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowConflictModal(false)}
                  className="px-4 py-2 border border-gray-300 rounded-lg text-xs"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading || !conflictReason.trim()}
                  className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg font-bold text-xs"
                >
                  Confirm Recusal
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Escalate Modal */}
      {showEscalateModal && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-purple-950 text-base flex items-center gap-2">
              <AlertOctagon className="w-5 h-5 text-purple-700" />
              Escalate Case
            </h3>
            <p className="text-xs text-gray-600">
              Escalate to Independent Review Officers / Board of Trustees. Use this when executive leadership is implicated or independent legal inquiry is necessary.
            </p>
            <form onSubmit={handleEscalateSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Escalation Justification <span className="text-red-500">*</span>
                </label>
                <textarea
                  rows={3}
                  required
                  value={escalationReason}
                  onChange={(e) => setEscalationReason(e.target.value)}
                  placeholder="Justify why standard investigation is insufficient..."
                  className="w-full p-2.5 text-xs border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowEscalateModal(false)}
                  className="px-4 py-2 border border-gray-300 rounded-lg text-xs"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading || !escalationReason.trim()}
                  className="px-4 py-2 bg-purple-800 hover:bg-purple-700 text-white rounded-lg font-bold text-xs"
                >
                  Confirm Escalation
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Resolve / Close Modal */}
      {showResolveModal && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-gray-900 text-base">Case Outcome & Conclusion</h3>
            <form onSubmit={handleResolveSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Action Type
                </label>
                <div className="flex gap-4">
                  <label className="flex items-center gap-2 text-xs font-bold">
                    <input
                      type="radio"
                      name="resolveAction"
                      checked={resolveAction === "resolve"}
                      onChange={() => setResolveAction("resolve")}
                    />
                    <span>Mark as Resolved</span>
                  </label>
                  <label className="flex items-center gap-2 text-xs font-bold">
                    <input
                      type="radio"
                      name="resolveAction"
                      checked={resolveAction === "close"}
                      onChange={() => setResolveAction("close")}
                    />
                    <span>Formally Close File</span>
                  </label>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Formal Resolution Summary <span className="text-red-500">*</span>
                </label>
                <p className="text-[11px] text-gray-500 mb-1">
                  This summary will be visible to the reporter in the status follow-up portal.
                </p>
                <textarea
                  rows={4}
                  required
                  value={resolutionSummary}
                  onChange={(e) => setResolutionSummary(e.target.value)}
                  placeholder="State the findings, corrective measures taken, disciplinary actions or support provided..."
                  className="w-full p-2.5 text-xs border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowResolveModal(false)}
                  className="px-4 py-2 border border-gray-300 rounded-lg text-xs"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={actionLoading || !resolutionSummary.trim()}
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white rounded-lg font-bold text-xs"
                >
                  Save Outcome
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
