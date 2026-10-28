"use client";

import React, { useEffect, useState, useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ShieldCheck,
  UserCheck,
  Clock,
  PlusCircle,
  AlertCircle,
  ArrowLeft,
  Calendar,
  History,
  X,
  ShieldAlert,
  ArrowRight,
  UserMinus,
  Search,
  Filter,
  CheckCircle2,
  PauseCircle,
  PlayCircle,
  RefreshCw,
  AlertTriangle,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { Appointment, User as UserModel, State, Zone, LGA, AppointmentHistory } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function AdminLeadershipPage() {
  const router = useRouter();
  const { user, isAdmin, isSuperAdmin, isLoading } = useAuth();

  const [appointments, setAppointments] = useState<Appointment[]>([]);
  const [states, setStates] = useState<State[]>([]);
  const [zones, setZones] = useState<Zone[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  // Search & Filter state
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "suspended" | "replaced" | "revoked">("all");
  const [roleFilter, setRoleFilter] = useState<string>("all");

  // New Appointment Modal & Wizard
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [assignStep, setAssignStep] = useState<"form" | "review">("form");
  const [memberSearch, setMemberSearch] = useState("");
  const [memberSearchResults, setMemberSearchResults] = useState<UserModel[]>([]);
  const [searchingMembers, setSearchingMembers] = useState(false);
  const [selectedUser, setSelectedUser] = useState<UserModel | null>(null);

  const [roleTitle, setRoleTitle] = useState("");
  const [roleType, setRoleType] = useState("state_coordinator");
  const [jurisdictionType, setJurisdictionType] = useState("state");
  const [jurisdictionId, setJurisdictionId] = useState("");
  const [selectedStateForLga, setSelectedStateForLga] = useState("");
  const [startDate, setStartDate] = useState(new Date().toISOString().split("T")[0]);
  const [approvalReference, setApprovalReference] = useState("");
  const [publicContact, setPublicContact] = useState("");
  const [assigning, setAssigning] = useState(false);

  // Replace Coordinator Modal (Super Admin only)
  const [replaceApp, setReplaceApp] = useState<Appointment | null>(null);
  const [replaceMemberSearch, setReplaceMemberSearch] = useState("");
  const [replaceSearchResults, setReplaceSearchResults] = useState<UserModel[]>([]);
  const [searchingReplaceMembers, setSearchingReplaceMembers] = useState(false);
  const [replaceSelectedUser, setReplaceSelectedUser] = useState<UserModel | null>(null);
  const [replaceEffectiveDate, setReplaceEffectiveDate] = useState(new Date().toISOString().split("T")[0]);
  const [replaceReason, setReplaceReason] = useState("");
  const [submittingReplace, setSubmittingReplace] = useState(false);

  // History Modal
  const [historyApp, setHistoryApp] = useState<Appointment | null>(null);
  const [historyRecords, setHistoryRecords] = useState<AppointmentHistory[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // Suspend Modal
  const [suspendApp, setSuspendApp] = useState<Appointment | null>(null);
  const [suspendReason, setSuspendReason] = useState("");
  const [submittingSuspend, setSubmittingSuspend] = useState(false);

  // Reactivate Modal
  const [reactivateApp, setReactivateApp] = useState<Appointment | null>(null);
  const [reactivateReason, setReactivateReason] = useState("");
  const [submittingReactivate, setSubmittingReactivate] = useState(false);

  // Revoke Modal
  const [revokeApp, setRevokeApp] = useState<Appointment | null>(null);
  const [revokeReason, setRevokeReason] = useState("");
  const [submittingRevoke, setSubmittingRevoke] = useState(false);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const loadData = async () => {
    try {
      const [appData, stateData, zoneData] = await Promise.all([
        apiRequest<Appointment[]>("/leadership/appointments"),
        apiRequest<State[]>("/geography/states"),
        apiRequest<Zone[]>("/geography/zones"),
      ]);
      setAppointments(appData || []);
      setStates(stateData || []);
      setZones(zoneData || []);
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to load leadership data" });
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    if (!isLoading && (!user || !isAdmin)) {
      router.push("/login?redirect=/admin/leadership");
      return;
    }
    if (user && isAdmin) {
      loadData();
    }
  }, [user, isAdmin, isLoading, router]);

  // Load LGAs when state changes for LGA appointment
  useEffect(() => {
    if (jurisdictionType === "lga" && selectedStateForLga) {
      apiRequest<LGA[]>(`/geography/states/${selectedStateForLga}/lgas`)
        .then((data) => setLgas(data || []))
        .catch(() => setLgas([]));
    }
  }, [jurisdictionType, selectedStateForLga]);

  // Debounced search for assign member
  useEffect(() => {
    if (!memberSearch.trim()) {
      setMemberSearchResults([]);
      return;
    }
    const timer = setTimeout(async () => {
      setSearchingMembers(true);
      try {
        const results = await apiRequest<UserModel[]>(`/members?search=${encodeURIComponent(memberSearch.trim())}&limit=10`);
        setMemberSearchResults(results || []);
      } catch {
        setMemberSearchResults([]);
      } finally {
        setSearchingMembers(false);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [memberSearch]);

  // Debounced search for replace member
  useEffect(() => {
    if (!replaceMemberSearch.trim()) {
      setReplaceSearchResults([]);
      return;
    }
    const timer = setTimeout(async () => {
      setSearchingReplaceMembers(true);
      try {
        const results = await apiRequest<UserModel[]>(`/members?search=${encodeURIComponent(replaceMemberSearch.trim())}&limit=10`);
        setReplaceSearchResults((results || []).filter((m) => m.id !== replaceApp?.user_id));
      } catch {
        setReplaceSearchResults([]);
      } finally {
        setSearchingReplaceMembers(false);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [replaceMemberSearch, replaceApp]);

  const handleRoleTypeChange = (r: string) => {
    setRoleType(r);
    if (r === "national_admin" || r === "super_admin") {
      setJurisdictionType("national");
      setJurisdictionId("");
    } else if (r === "zonal_coordinator") {
      setJurisdictionType("zone");
      if (zones.length > 0 && !jurisdictionId) setJurisdictionId(zones[0].id);
    } else if (r === "state_coordinator") {
      setJurisdictionType("state");
      if (states.length > 0 && !jurisdictionId) setJurisdictionId(states[0].id);
    } else if (r === "lga_representative" || r === "lga_coordinator") {
      setJurisdictionType("lga");
      if (states.length > 0 && !selectedStateForLga) {
        setSelectedStateForLga(states[0].id);
      }
    }
  };

  const getJurisdictionLabel = () => {
    if (jurisdictionType === "national") return "National (All Nigeria)";
    if (jurisdictionType === "zone") {
      const z = zones.find((item) => item.id === jurisdictionId || item.code === jurisdictionId);
      return z ? `${z.name} (${z.code})` : jurisdictionId;
    }
    if (jurisdictionType === "state") {
      const s = states.find((item) => item.id === jurisdictionId || item.code === jurisdictionId);
      return s ? `${s.name} (${s.code})` : jurisdictionId;
    }
    if (jurisdictionType === "lga") {
      const l = lgas.find((item) => item.id === jurisdictionId);
      return l ? l.name : jurisdictionId;
    }
    return "Not selected";
  };

  const handleAssignSubmit = async () => {
    if (!selectedUser) {
      setFeedback({ type: "error", message: "Please select an existing registered member." });
      return;
    }
    setAssigning(true);
    setFeedback(null);

    try {
      await apiRequest("/leadership/appointments", {
        method: "POST",
        body: JSON.stringify({
          user_id: selectedUser.id,
          role_title: roleTitle || `${roleType.replace(/_/g, " ").toUpperCase()} - ${getJurisdictionLabel()}`,
          role_type: roleType,
          jurisdiction_type: jurisdictionType,
          jurisdiction_id: jurisdictionType === "national" ? null : jurisdictionId,
          start_date: new Date(startDate).toISOString(),
          approval_reference: approvalReference || `APPT-${Date.now().toString().slice(-6)}`,
          public_contact: publicContact || undefined,
        }),
      });

      setFeedback({ type: "success", message: `Leadership appointment for ${selectedUser.display_name} successfully confirmed and active.` });
      setShowAssignModal(false);
      setAssignStep("form");
      setSelectedUser(null);
      setMemberSearch("");
      setRoleTitle("");
      setApprovalReference("");
      setPublicContact("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Appointment assignment failed." });
    } finally {
      setAssigning(false);
    }
  };

  const handleOpenHistory = async (app: Appointment) => {
    setHistoryApp(app);
    setLoadingHistory(true);
    try {
      const hist = await apiRequest<AppointmentHistory[]>(`/leadership/appointments/${app.id}/history`);
      setHistoryRecords(hist || []);
    } catch {
      setHistoryRecords([]);
    } finally {
      setLoadingHistory(false);
    }
  };

  const handleReplaceSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!replaceApp || !replaceSelectedUser) return;
    setSubmittingReplace(true);
    setFeedback(null);

    try {
      await apiRequest(`/leadership/appointments/${replaceApp.id}/replace`, {
        method: "POST",
        body: JSON.stringify({
          new_user_id: replaceSelectedUser.id,
          effective_date: new Date(replaceEffectiveDate).toISOString(),
          reason: replaceReason,
        }),
      });

      setFeedback({
        type: "success",
        message: `Appointment replaced successfully. ${replaceSelectedUser.display_name} appointed as the new coordinator.`,
      });
      setReplaceApp(null);
      setReplaceSelectedUser(null);
      setReplaceMemberSearch("");
      setReplaceReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Replacement failed." });
    } finally {
      setSubmittingReplace(false);
    }
  };

  const handleSuspendSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!suspendApp) return;
    setSubmittingSuspend(true);
    setFeedback(null);

    try {
      await apiRequest(`/leadership/appointments/${suspendApp.id}/suspend`, {
        method: "POST",
        body: JSON.stringify({ reason: suspendReason || "Suspended by Super Administrator" }),
      });

      setFeedback({ type: "success", message: `Appointment suspended. Administrative permissions revoked temporarily.` });
      setSuspendApp(null);
      setSuspendReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Suspension failed." });
    } finally {
      setSubmittingSuspend(false);
    }
  };

  const handleReactivateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reactivateApp) return;
    setSubmittingReactivate(true);
    setFeedback(null);

    try {
      await apiRequest(`/leadership/appointments/${reactivateApp.id}/reactivate`, {
        method: "POST",
        body: JSON.stringify({ reason: reactivateReason || "Reactivated by Super Administrator" }),
      });

      setFeedback({ type: "success", message: `Appointment reactivated successfully. Coordinator privileges restored.` });
      setReactivateApp(null);
      setReactivateReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Reactivation failed." });
    } finally {
      setSubmittingReactivate(false);
    }
  };

  const handleRevokeSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!revokeApp) return;
    setSubmittingRevoke(true);
    setFeedback(null);

    try {
      await apiRequest(`/leadership/appointments/${revokeApp.id}/revoke`, {
        method: "POST",
        body: JSON.stringify({ reason: revokeReason || "Revoked by Super Administrator" }),
      });

      setFeedback({ type: "success", message: `Appointment permanently revoked.` });
      setRevokeApp(null);
      setRevokeReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Revocation failed." });
    } finally {
      setSubmittingRevoke(false);
    }
  };

  // Filtered Appointments
  const filteredAppointments = useMemo(() => {
    return appointments.filter((app) => {
      const query = searchQuery.toLowerCase();
      const matchesSearch =
        (app.user_display_name || app.user_name || "").toLowerCase().includes(query) ||
        (app.user_membership_ref || "").toLowerCase().includes(query) ||
        (app.role_title || "").toLowerCase().includes(query) ||
        (app.jurisdiction_name || "").toLowerCase().includes(query) ||
        (app.approval_reference || "").toLowerCase().includes(query);

      const matchesStatus =
        statusFilter === "all" ||
        (statusFilter === "active" && app.status === "active") ||
        (statusFilter === "suspended" && app.status === "suspended") ||
        (statusFilter === "replaced" && app.status === "replaced") ||
        (statusFilter === "revoked" && app.status === "revoked");

      const matchesRole =
        roleFilter === "all" || app.role_type === roleFilter;

      return matchesSearch && matchesStatus && matchesRole;
    });
  }, [appointments, searchQuery, statusFilter, roleFilter]);

  if (isLoading || loading) return <Spinner text="Loading leadership directory..." />;

  const isSuper = isSuperAdmin || user?.primary_role === "super_admin";

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      {/* Navigation Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <Link
            href="/admin"
            className="inline-flex items-center text-xs font-semibold text-emerald-800 hover:text-emerald-950 mb-2 transition"
          >
            <ArrowLeft className="w-4 h-4 mr-1" />
            Back to Nationwide Operational Command Center
          </Link>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl sm:text-3xl font-black text-gray-900 tracking-tight">
              Nationwide Leadership Governance & Appointments
            </h1>
            {isSuper && (
              <Badge variant="warning" size="sm">
                SUPER ADMIN
              </Badge>
            )}
          </div>
          <p className="text-xs text-gray-600 mt-1">
            Authoritative leadership appointments, replacement workflows, and lifecycle audits across all 6 Geopolitical Zones, 36 States + FCT, and 774 Local Government Areas.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => {
              setRefreshing(true);
              loadData();
            }}
            disabled={refreshing}
            className="inline-flex items-center px-3 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 bg-white hover:bg-gray-50 transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${refreshing ? "animate-spin" : ""}`} />
            Refresh
          </button>
          {isSuper && (
            <button
              type="button"
              onClick={() => {
                setShowAssignModal(true);
                setAssignStep("form");
              }}
              className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs font-bold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition"
            >
              <PlusCircle className="w-4 h-4" />
              Appoint Coordinator
            </button>
          )}
        </div>
      </div>

      {!isSuper && (
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-4 flex items-start gap-3 text-xs text-amber-900">
          <ShieldAlert className="w-5 h-5 text-amber-700 flex-shrink-0 mt-0.5" />
          <div>
            <span className="font-bold block">Super Administrator Exclusive Authority</span>
            <span className="block mt-0.5">
              Only a Super Administrator has the authority to appoint, replace, suspend, reactivate or revoke Zonal and State Coordinators. Lower-level appointments can be proposed or reviewed.
            </span>
          </div>
        </div>
      )}

      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}

      {/* Filter and Search Bar */}
      <div className="bg-white p-4 rounded-2xl border border-gray-200 shadow-sm space-y-4">
        <div className="flex flex-col md:flex-row gap-3 items-stretch md:items-center justify-between">
          <div className="relative flex-1">
            <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
            <input
              type="text"
              placeholder="Search by coordinator name, reference, jurisdiction, or approval ref..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-9 pr-4 py-2 border border-gray-200 rounded-xl text-xs focus:ring-emerald-500 focus:border-emerald-500"
            />
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Filter className="w-4 h-4 text-gray-400" />
            <select
              value={roleFilter}
              onChange={(e) => setRoleFilter(e.target.value)}
              className="border border-gray-200 rounded-xl text-xs py-2 px-3 focus:ring-emerald-500 focus:border-emerald-500 bg-white"
            >
              <option value="all">All Roles</option>
              <option value="zonal_coordinator">Zonal Coordinators</option>
              <option value="state_coordinator">State Coordinators</option>
              <option value="lga_representative">LGA Representatives</option>
              <option value="national_admin">National Admins</option>
              <option value="super_admin">Super Admins</option>
            </select>
          </div>
        </div>

        {/* Status Pill Filters */}
        <div className="flex flex-wrap items-center gap-1.5 pt-2 border-t border-gray-100">
          <span className="text-[11px] font-bold text-gray-500 uppercase mr-2">Status:</span>
          {(
            [
              { id: "all", label: "All Records", count: appointments.length },
              { id: "active", label: "Active", count: appointments.filter((a) => a.status === "active").length },
              { id: "suspended", label: "Suspended", count: appointments.filter((a) => a.status === "suspended").length },
              { id: "replaced", label: "Replaced", count: appointments.filter((a) => a.status === "replaced").length },
              { id: "revoked", label: "Revoked", count: appointments.filter((a) => a.status === "revoked").length },
            ] as const
          ).map((item) => {
            const isSelected = statusFilter === item.id;
            return (
              <button
                key={item.id}
                onClick={() => setStatusFilter(item.id)}
                className={`px-3 py-1 rounded-full text-xs font-semibold transition ${
                  isSelected
                    ? "bg-emerald-800 text-white shadow-sm"
                    : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                }`}
              >
                {item.label} ({item.count})
              </button>
            );
          })}
        </div>
      </div>

      {/* Leadership Directory Table */}
      <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
        <div className="p-4 border-b border-gray-100 bg-gray-50/50 flex justify-between items-center text-xs">
          <span className="font-bold text-gray-700">Official Nationwide Leadership Roster</span>
          <span className="text-gray-500">{filteredAppointments.length} matching positions</span>
        </div>

        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 text-xs">
            <thead className="bg-gray-50 text-gray-600 font-bold uppercase text-[11px]">
              <tr>
                <th className="px-4 py-3 text-left">Coordinator</th>
                <th className="px-4 py-3 text-left">Role Title</th>
                <th className="px-4 py-3 text-left">Jurisdiction</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-left">Effective Date</th>
                <th className="px-4 py-3 text-left">Reference</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredAppointments.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-gray-400">
                    No appointments found matching your search criteria.
                  </td>
                </tr>
              ) : (
                filteredAppointments.map((app) => {
                  const isActorSelf = user?.id === app.user_id;
                  return (
                    <tr key={app.id} className="hover:bg-gray-50/60 transition">
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <div className="font-bold text-gray-900">
                          {app.user_display_name || app.user_name}
                        </div>
                        <div className="text-[11px] font-mono text-gray-500">
                          {app.user_membership_ref || app.user_id.slice(0, 8)}
                        </div>
                        {app.user_email && (
                          <div className="text-[10px] text-gray-400">{app.user_email}</div>
                        )}
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap font-medium text-gray-800">
                        {app.role_title}
                        <div className="text-[10px] text-gray-400 capitalize">
                          {app.role_type.replace(/_/g, " ")}
                        </div>
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap text-gray-600">
                        <span className="font-semibold text-gray-800">
                          {app.jurisdiction_name || (app.jurisdiction_type === "national" ? "National (Nigeria)" : app.jurisdiction_type)}
                        </span>
                        <div className="text-[10px] text-gray-400 capitalize">
                          Scope: {app.jurisdiction_type}
                        </div>
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <Badge
                          variant={
                            app.status === "active"
                              ? "success"
                              : app.status === "suspended"
                              ? "warning"
                              : "danger"
                          }
                          size="sm"
                        >
                          {app.status.toUpperCase()}
                        </Badge>
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap text-gray-500">
                        {new Date(app.start_date).toLocaleDateString("en-NG")}
                        {app.end_date && (
                          <div className="text-[10px] text-gray-400">
                            Until: {new Date(app.end_date).toLocaleDateString("en-NG")}
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap font-mono text-gray-500 text-[11px]">
                        {app.approval_reference}
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          <button
                            type="button"
                            onClick={() => handleOpenHistory(app)}
                            className="px-2 py-1 border border-gray-200 hover:bg-gray-100 text-gray-700 font-semibold rounded-lg text-[11px] transition"
                          >
                            History
                          </button>

                          {isSuper && !isActorSelf && (
                            <>
                              {app.status === "active" && (
                                <>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      setReplaceApp(app);
                                      setReplaceSelectedUser(null);
                                      setReplaceMemberSearch("");
                                      setReplaceReason("");
                                    }}
                                    className="px-2 py-1 bg-amber-50 hover:bg-amber-100 text-amber-800 font-semibold rounded-lg text-[11px] transition"
                                  >
                                    Replace
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      setSuspendApp(app);
                                      setSuspendReason("");
                                    }}
                                    className="px-2 py-1 bg-yellow-50 hover:bg-yellow-100 text-yellow-800 font-semibold rounded-lg text-[11px] transition"
                                  >
                                    Suspend
                                  </button>
                                </>
                              )}

                              {app.status === "suspended" && (
                                <>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      setReactivateApp(app);
                                      setReactivateReason("");
                                    }}
                                    className="px-2 py-1 bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-semibold rounded-lg text-[11px] transition"
                                  >
                                    Reactivate
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      setReplaceApp(app);
                                      setReplaceSelectedUser(null);
                                      setReplaceMemberSearch("");
                                      setReplaceReason("");
                                    }}
                                    className="px-2 py-1 bg-amber-50 hover:bg-amber-100 text-amber-800 font-semibold rounded-lg text-[11px] transition"
                                  >
                                    Replace
                                  </button>
                                </>
                              )}

                              {app.status !== "revoked" && (
                                <button
                                  type="button"
                                  onClick={() => {
                                    setRevokeApp(app);
                                    setRevokeReason("");
                                  }}
                                  className="px-2 py-1 bg-red-50 hover:bg-red-100 text-red-800 font-semibold rounded-lg text-[11px] transition"
                                >
                                  Revoke
                                </button>
                              )}
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* =========================================================================
          MODAL 1: Appointment Workflow (Super Admin)
         ========================================================================= */}
      {showAssignModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm overflow-y-auto">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-5 shadow-2xl border border-gray-200 my-8">
            <div className="flex justify-between items-start border-b pb-3">
              <div>
                <span className="text-[10px] font-black uppercase tracking-wider text-emerald-700 block">
                  Nationwide Coordinator Appointment
                </span>
                <h2 className="text-lg font-black text-gray-900 mt-0.5">
                  {assignStep === "form" ? "Assign Leadership Position" : "Review & Confirm Appointment"}
                </h2>
              </div>
              <button
                onClick={() => {
                  setShowAssignModal(false);
                  setAssignStep("form");
                }}
                className="text-gray-400 hover:text-gray-600"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {assignStep === "form" ? (
              <div className="space-y-4 text-xs">
                {/* 1. Appointee Member Search & Select */}
                <div>
                  <label className="block font-bold text-gray-700 mb-1">
                    Search Registered Member *
                  </label>
                  {selectedUser ? (
                    <div className="flex items-center justify-between p-3 rounded-xl bg-emerald-50 border border-emerald-300">
                      <div>
                        <div className="font-bold text-emerald-950 text-sm">
                          {selectedUser.display_name}
                        </div>
                        <div className="text-gray-600 font-mono text-[11px]">
                          Ref: {selectedUser.membership_reference} • Role: {selectedUser.primary_role}
                        </div>
                        {selectedUser.email && (
                          <div className="text-gray-500 text-[10px]">{selectedUser.email}</div>
                        )}
                      </div>
                      <button
                        type="button"
                        onClick={() => setSelectedUser(null)}
                        className="px-2.5 py-1 text-xs font-semibold text-emerald-900 hover:bg-emerald-200 rounded-lg transition"
                      >
                        Change Member
                      </button>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      <div className="relative">
                        <Search className="w-4 h-4 text-gray-400 absolute left-3 top-2.5" />
                        <input
                          type="text"
                          placeholder="Search registered members by name, email, or reference number..."
                          value={memberSearch}
                          onChange={(e) => setMemberSearch(e.target.value)}
                          className="w-full pl-9 pr-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                        />
                      </div>
                      {searchingMembers && (
                        <div className="text-[11px] text-gray-500 italic">Searching registered members...</div>
                      )}
                      {memberSearchResults.length > 0 && (
                        <div className="border border-gray-200 rounded-xl max-h-48 overflow-y-auto divide-y bg-white shadow-sm">
                          {memberSearchResults.map((m) => (
                            <button
                              key={m.id}
                              type="button"
                              onClick={() => {
                                setSelectedUser(m);
                                setMemberSearchResults([]);
                                setMemberSearch("");
                              }}
                              className="w-full text-left p-2.5 hover:bg-gray-50 flex items-center justify-between transition"
                            >
                              <div>
                                <div className="font-bold text-gray-900">{m.display_name}</div>
                                <div className="text-[10px] text-gray-500 font-mono">
                                  {m.membership_reference} • {m.email || m.phone}
                                </div>
                              </div>
                              <span className="text-[10px] font-bold text-emerald-700 uppercase">
                                Select
                              </span>
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* 2. Role Type */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div>
                    <label className="block font-bold text-gray-700 mb-1">Coordinator Role *</label>
                    <select
                      value={roleType}
                      onChange={(e) => handleRoleTypeChange(e.target.value)}
                      className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                    >
                      <option value="state_coordinator">State Coordinator</option>
                      <option value="zonal_coordinator">Zonal Coordinator</option>
                      <option value="lga_representative">LGA Representative</option>
                      <option value="national_admin">National Administrator</option>
                      <option value="super_admin">Super Administrator</option>
                    </select>
                  </div>

                  <div>
                    <label className="block font-bold text-gray-700 mb-1">Official Role Title</label>
                    <input
                      type="text"
                      value={roleTitle}
                      onChange={(e) => setRoleTitle(e.target.value)}
                      placeholder="e.g. State Coordinator - Kano"
                      className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                    />
                  </div>
                </div>

                {/* 3. Dynamic Jurisdiction Selector */}
                {jurisdictionType === "zone" && (
                  <div>
                    <label className="block font-bold text-gray-700 mb-1">
                      Geopolitical Zone *
                    </label>
                    <select
                      required
                      value={jurisdictionId}
                      onChange={(e) => setJurisdictionId(e.target.value)}
                      className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                    >
                      <option value="">Select geopolitical zone...</option>
                      {zones.map((z) => (
                        <option key={z.id} value={z.id}>
                          {z.name} ({z.code})
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                {jurisdictionType === "state" && (
                  <div>
                    <label className="block font-bold text-gray-700 mb-1">
                      State / FCT *
                    </label>
                    <select
                      required
                      value={jurisdictionId}
                      onChange={(e) => setJurisdictionId(e.target.value)}
                      className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                    >
                      <option value="">Select state or territory...</option>
                      {states.map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name} ({s.code}) {s.is_fct ? "— FCT" : ""}
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                {jurisdictionType === "lga" && (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div>
                      <label className="block font-bold text-gray-700 mb-1">State for LGA *</label>
                      <select
                        value={selectedStateForLga}
                        onChange={(e) => {
                          setSelectedStateForLga(e.target.value);
                          setJurisdictionId("");
                        }}
                        className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                      >
                        <option value="">Select State...</option>
                        {states.map((s) => (
                          <option key={s.id} value={s.id}>
                            {s.name} ({s.code})
                          </option>
                        ))}
                      </select>
                    </div>

                    <div>
                      <label className="block font-bold text-gray-700 mb-1">
                        Local Government Council *
                      </label>
                      <select
                        required
                        value={jurisdictionId}
                        onChange={(e) => setJurisdictionId(e.target.value)}
                        className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                      >
                        <option value="">Select LGA...</option>
                        {lgas.map((l) => (
                          <option key={l.id} value={l.id}>
                            {l.name}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>
                )}

                {/* 4. Start Date & Reference */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div>
                    <label className="block font-bold text-gray-700 mb-1">Start Date *</label>
                    <input
                      type="date"
                      required
                      value={startDate}
                      onChange={(e) => setStartDate(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                    />
                  </div>

                  <div>
                    <label className="block font-bold text-gray-700 mb-1">Approval Reference</label>
                    <input
                      type="text"
                      value={approvalReference}
                      onChange={(e) => setApprovalReference(e.target.value)}
                      placeholder="e.g. APPT-2026-HQ-001"
                      className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs font-mono focus:ring-emerald-600 focus:border-emerald-600"
                    />
                  </div>
                </div>

                <div>
                  <label className="block font-bold text-gray-700 mb-1">Public Official Contact (Optional)</label>
                  <input
                    type="text"
                    value={publicContact}
                    onChange={(e) => setPublicContact(e.target.value)}
                    placeholder="e.g. lagos.ratels@example.org or dedicated line"
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                  />
                </div>

                <div className="flex justify-end gap-2 pt-4 border-t">
                  <button
                    type="button"
                    onClick={() => setShowAssignModal(false)}
                    className="px-4 py-2 border border-gray-300 rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    disabled={!selectedUser || (jurisdictionType !== "national" && !jurisdictionId)}
                    onClick={() => setAssignStep("review")}
                    className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl font-bold shadow-sm disabled:opacity-50 transition"
                  >
                    Proceed to Review
                  </button>
                </div>
              </div>
            ) : (
              /* Review & Confirmation Step */
              <div className="space-y-4 text-xs">
                <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-4 space-y-2">
                  <h3 className="font-bold text-emerald-950 text-sm flex items-center gap-1.5">
                    <CheckCircle2 className="w-4 h-4 text-emerald-700" />
                    Appointment Summary Verification
                  </h3>
                  <div className="grid grid-cols-2 gap-2 text-xs pt-1">
                    <div>
                      <span className="text-gray-500 block">Appointee:</span>
                      <span className="font-bold text-gray-900">{selectedUser?.display_name}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Membership Reference:</span>
                      <span className="font-mono text-gray-800">{selectedUser?.membership_reference}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Role Mandate:</span>
                      <span className="font-bold text-gray-900 capitalize">
                        {roleType.replace(/_/g, " ")}
                      </span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Jurisdiction:</span>
                      <span className="font-bold text-emerald-900">{getJurisdictionLabel()}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Effective Date:</span>
                      <span className="font-medium text-gray-800">{startDate}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Approval Reference:</span>
                      <span className="font-mono text-gray-800">
                        {approvalReference || "Auto-generated"}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="bg-amber-50 border border-amber-200 rounded-xl p-3 text-[11px] text-amber-900 flex items-start gap-2">
                  <AlertTriangle className="w-4 h-4 text-amber-700 flex-shrink-0 mt-0.5" />
                  <span>
                    Confirming this appointment will immediately grant administrative leadership access for this jurisdiction and dispatch an official notification to the member.
                  </span>
                </div>

                <div className="flex justify-end gap-2 pt-3 border-t">
                  <button
                    type="button"
                    onClick={() => setAssignStep("form")}
                    className="px-4 py-2 border border-gray-300 rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                  >
                    Back to Edit
                  </button>
                  <button
                    type="button"
                    disabled={assigning}
                    onClick={handleAssignSubmit}
                    className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl font-bold shadow-sm transition"
                  >
                    {assigning ? "Confirming Appointment..." : "Confirm & Appoint"}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* =========================================================================
          MODAL 2: Replacement Workflow (Super Admin)
         ========================================================================= */}
      {replaceApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex justify-between items-start border-b pb-3">
              <div>
                <span className="text-[10px] font-black text-amber-700 uppercase tracking-wider block">
                  Coordinator Replacement Handover
                </span>
                <h2 className="text-base font-bold text-gray-900 mt-0.5">
                  Replace: {replaceApp.role_title}
                </h2>
                <div className="text-xs text-gray-500 mt-0.5">
                  Current holder: <span className="font-semibold">{replaceApp.user_display_name || replaceApp.user_name}</span> ({replaceApp.jurisdiction_name || "National"})
                </div>
              </div>
              <button onClick={() => setReplaceApp(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleReplaceSubmit} className="space-y-4 text-xs">
              <div>
                <label className="block font-bold text-gray-700 mb-1">
                  Select Replacement Member *
                </label>
                {replaceSelectedUser ? (
                  <div className="flex items-center justify-between p-3 rounded-xl bg-amber-50 border border-amber-300">
                    <div>
                      <div className="font-bold text-amber-950 text-sm">
                        {replaceSelectedUser.display_name}
                      </div>
                      <div className="text-gray-600 font-mono text-[11px]">
                        Ref: {replaceSelectedUser.membership_reference}
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => setReplaceSelectedUser(null)}
                      className="px-2.5 py-1 text-xs font-semibold text-amber-900 hover:bg-amber-200 rounded-lg transition"
                    >
                      Change
                    </button>
                  </div>
                ) : (
                  <div className="space-y-2">
                    <div className="relative">
                      <Search className="w-4 h-4 text-gray-400 absolute left-3 top-2.5" />
                      <input
                        type="text"
                        placeholder="Search replacement candidate by name or ref..."
                        value={replaceMemberSearch}
                        onChange={(e) => setReplaceMemberSearch(e.target.value)}
                        className="w-full pl-9 pr-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-amber-600 focus:border-amber-600"
                      />
                    </div>
                    {searchingReplaceMembers && (
                      <div className="text-[11px] text-gray-500 italic">Searching candidates...</div>
                    )}
                    {replaceSearchResults.length > 0 && (
                      <div className="border border-gray-200 rounded-xl max-h-40 overflow-y-auto divide-y bg-white shadow-sm">
                        {replaceSearchResults.map((m) => (
                          <button
                            key={m.id}
                            type="button"
                            onClick={() => {
                              setReplaceSelectedUser(m);
                              setReplaceSearchResults([]);
                              setReplaceMemberSearch("");
                            }}
                            className="w-full text-left p-2.5 hover:bg-gray-50 flex items-center justify-between transition"
                          >
                            <div>
                              <div className="font-bold text-gray-900">{m.display_name}</div>
                              <div className="text-[10px] text-gray-500 font-mono">
                                {m.membership_reference} • {m.email}
                              </div>
                            </div>
                            <span className="text-[10px] font-bold text-amber-700 uppercase">
                              Select
                            </span>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div>
                <label className="block font-bold text-gray-700 mb-1">
                  Effective Handover Date *
                </label>
                <input
                  type="date"
                  required
                  value={replaceEffectiveDate}
                  onChange={(e) => setReplaceEffectiveDate(e.target.value)}
                  className="w-full border border-gray-300 rounded-xl p-2 bg-white"
                />
              </div>

              <div>
                <label className="block font-bold text-gray-700 mb-1">
                  Formal Reason for Replacement * (Minimum 5 characters)
                </label>
                <textarea
                  required
                  minLength={5}
                  rows={3}
                  value={replaceReason}
                  onChange={(e) => setReplaceReason(e.target.value)}
                  placeholder="State the governance justification: e.g. term conclusion, rotation, relocation, or performance review..."
                  className="w-full border border-gray-300 rounded-xl p-2.5 text-xs focus:ring-amber-600 focus:border-amber-600"
                />
              </div>

              <div className="flex justify-end gap-2 pt-3 border-t">
                <button
                  type="button"
                  onClick={() => setReplaceApp(null)}
                  className="px-4 py-2 border rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingReplace || !replaceSelectedUser}
                  className="px-5 py-2 bg-amber-700 hover:bg-amber-800 text-white rounded-xl font-bold transition disabled:opacity-50"
                >
                  {submittingReplace ? "Executing Handover..." : "Confirm Replacement"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* =========================================================================
          MODAL 3: Suspend Coordinator Modal
         ========================================================================= */}
      {suspendApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex items-center gap-2 text-yellow-700 font-bold">
              <PauseCircle className="w-5 h-5 text-yellow-600" />
              <span>Suspend Coordinator Mandate</span>
            </div>
            <p className="text-xs text-gray-600">
              You are about to suspend <span className="font-bold text-gray-900">{suspendApp.user_display_name || suspendApp.user_name}</span> from the role of <span className="font-bold text-gray-900">{suspendApp.role_title}</span>. Their administrative permissions will be halted immediately.
            </p>

            <form onSubmit={handleSuspendSubmit} className="space-y-4 text-xs">
              <div>
                <label className="block font-bold text-gray-700 mb-1">
                  Mandatory Suspension Reason *
                </label>
                <textarea
                  required
                  minLength={5}
                  rows={3}
                  value={suspendReason}
                  onChange={(e) => setSuspendReason(e.target.value)}
                  placeholder="State the formal cause for temporary suspension..."
                  className="w-full border border-gray-300 rounded-xl p-2.5 text-xs focus:ring-yellow-600 focus:border-yellow-600"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setSuspendApp(null)}
                  className="px-4 py-2 border rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingSuspend}
                  className="px-5 py-2 bg-yellow-600 hover:bg-yellow-700 text-white rounded-xl font-bold transition"
                >
                  {submittingSuspend ? "Suspending..." : "Confirm Suspension"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* =========================================================================
          MODAL 4: Reactivate Coordinator Modal
         ========================================================================= */}
      {reactivateApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex items-center gap-2 text-emerald-800 font-bold">
              <PlayCircle className="w-5 h-5 text-emerald-700" />
              <span>Reactivate Suspended Coordinator</span>
            </div>
            <p className="text-xs text-gray-600">
              Restore active mandate and administrative privileges for <span className="font-bold text-gray-900">{reactivateApp.user_display_name || reactivateApp.user_name}</span> as <span className="font-bold text-gray-900">{reactivateApp.role_title}</span>.
            </p>

            <form onSubmit={handleReactivateSubmit} className="space-y-4 text-xs">
              <div>
                <label className="block font-bold text-gray-700 mb-1">
                  Reactivation Note / Reason *
                </label>
                <input
                  type="text"
                  required
                  value={reactivateReason}
                  onChange={(e) => setReactivateReason(e.target.value)}
                  placeholder="e.g. Investigation completed with full clearance"
                  className="w-full border border-gray-300 rounded-xl p-2.5 text-xs focus:ring-emerald-600 focus:border-emerald-600"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setReactivateApp(null)}
                  className="px-4 py-2 border rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingReactivate}
                  className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl font-bold transition"
                >
                  {submittingReactivate ? "Reactivating..." : "Confirm Reactivation"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* =========================================================================
          MODAL 5: Revoke Coordinator Modal
         ========================================================================= */}
      {revokeApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex items-center gap-2 text-red-700 font-bold">
              <AlertCircle className="w-5 h-5 text-red-600" />
              <span>Permanent Revocation of Appointment</span>
            </div>
            <p className="text-xs text-gray-600">
              This action permanently terminates the appointment of <span className="font-bold text-gray-900">{revokeApp.user_display_name || revokeApp.user_name}</span> for <span className="font-bold text-gray-900">{revokeApp.role_title}</span>. Historical audit logs will be permanently retained.
            </p>

            <form onSubmit={handleRevokeSubmit} className="space-y-4 text-xs">
              <div>
                <label className="block font-bold text-gray-700 mb-1">
                  Official Revocation Reason *
                </label>
                <textarea
                  required
                  minLength={5}
                  rows={3}
                  value={revokeReason}
                  onChange={(e) => setRevokeReason(e.target.value)}
                  placeholder="State the permanent termination grounds..."
                  className="w-full border border-gray-300 rounded-xl p-2.5 text-xs focus:ring-red-600 focus:border-red-600"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setRevokeApp(null)}
                  className="px-4 py-2 border rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingRevoke}
                  className="px-5 py-2 bg-red-700 hover:bg-red-800 text-white rounded-xl font-bold transition"
                >
                  {submittingRevoke ? "Revoking..." : "Confirm Revocation"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* =========================================================================
          MODAL 6: Appointment History Drawer / Modal
         ========================================================================= */}
      {historyApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex justify-between items-start border-b pb-3">
              <div>
                <h2 className="text-base font-bold text-gray-900 flex items-center gap-1.5">
                  <History className="w-5 h-5 text-emerald-800" />
                  Appointment Audit History
                </h2>
                <span className="text-xs text-gray-500">
                  {historyApp.role_title} • {historyApp.jurisdiction_name || "National"}
                </span>
              </div>
              <button onClick={() => setHistoryApp(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            {loadingHistory ? (
              <Spinner text="Loading appointment history..." />
            ) : historyRecords.length === 0 ? (
              <div className="p-6 text-center text-xs text-gray-500">
                No appointment history records found for this position.
              </div>
            ) : (
              <div className="space-y-3 max-h-80 overflow-y-auto text-xs pr-1">
                {historyRecords.map((hRec) => (
                  <div key={hRec.id} className="p-3 bg-gray-50 rounded-xl border border-gray-200 space-y-1">
                    <div className="flex justify-between items-center">
                      <span className="font-bold text-gray-900 capitalize">
                        {hRec.action.replace(/_/g, " ")}
                      </span>
                      <span className="text-[10px] text-gray-400 font-mono">
                        {new Date(hRec.timestamp).toLocaleString()}
                      </span>
                    </div>

                    <div className="flex items-center gap-2 text-gray-600 text-[11px]">
                      <span>{hRec.previous_user_name || "Initial"}</span>
                      <ArrowRight className="w-3 h-3 text-gray-400" />
                      <span className="font-semibold text-emerald-900">{hRec.new_user_name || "None"}</span>
                    </div>

                    {hRec.reason && (
                      <p className="text-[11px] text-gray-700 bg-white p-2 rounded border border-gray-100">
                        Reason: {hRec.reason}
                      </p>
                    )}

                    <div className="text-[10px] text-gray-500 pt-1">
                      Recorded by: {hRec.changed_by_name || "Super Admin"} • Effective:{" "}
                      {new Date(hRec.effective_date).toLocaleDateString()}
                    </div>
                  </div>
                ))}
              </div>
            )}

            <div className="flex justify-end pt-2 border-t">
              <button
                type="button"
                onClick={() => setHistoryApp(null)}
                className="px-4 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-xl text-xs font-semibold"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
