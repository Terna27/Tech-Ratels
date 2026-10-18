"use client";

import React, { useEffect, useState } from "react";
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
  const [members, setMembers] = useState<UserModel[]>([]);
  const [states, setStates] = useState<State[]>([]);
  const [zones, setZones] = useState<Zone[]>([]);
  const [loading, setLoading] = useState(true);

  // New Appointment Modal
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState("");
  const [roleTitle, setRoleTitle] = useState("");
  const [roleType, setRoleType] = useState("state_coordinator");
  const [jurisdictionType, setJurisdictionType] = useState("state");
  const [jurisdictionId, setJurisdictionId] = useState("");
  const [startDate, setStartDate] = useState(new Date().toISOString().split("T")[0]);
  const [approvalReference, setApprovalReference] = useState("");
  const [publicContact, setPublicContact] = useState("");
  const [assigning, setAssigning] = useState(false);

  // Replace Coordinator Modal (Super Admin only)
  const [replaceApp, setReplaceApp] = useState<Appointment | null>(null);
  const [replaceNewUserId, setReplaceNewUserId] = useState("");
  const [replaceEffectiveDate, setReplaceEffectiveDate] = useState(new Date().toISOString().split("T")[0]);
  const [replaceReason, setReplaceReason] = useState("");
  const [submittingReplace, setSubmittingReplace] = useState(false);

  // History Modal
  const [historyApp, setHistoryApp] = useState<Appointment | null>(null);
  const [historyRecords, setHistoryRecords] = useState<AppointmentHistory[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // Status Change Modal
  const [statusAppointmentId, setStatusAppointmentId] = useState<string | null>(null);
  const [newStatus, setNewStatus] = useState<"active" | "acting" | "expired" | "revoked">("active");
  const [statusReason, setStatusReason] = useState("");
  const [updatingStatus, setUpdatingStatus] = useState(false);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const loadData = async () => {
    try {
      const [appData, memData, stateData, zoneData] = await Promise.all([
        apiRequest<Appointment[]>("/leadership/appointments"),
        apiRequest<UserModel[]>("/members?limit=100"),
        apiRequest<State[]>("/geography/states"),
        apiRequest<Zone[]>("/geography/zones"),
      ]);
      setAppointments(appData || []);
      setMembers(memData || []);
      setStates(stateData || []);
      setZones(zoneData || []);
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Failed to load leadership data" });
    } finally {
      setLoading(false);
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

  const handleAssign = async (e: React.FormEvent) => {
    e.preventDefault();
    setAssigning(true);
    setFeedback(null);

    try {
      await apiRequest("/leadership/appointments", {
        method: "POST",
        body: JSON.stringify({
          user_id: selectedUserId,
          role_title: roleTitle,
          role_type: roleType,
          jurisdiction_type: jurisdictionType,
          jurisdiction_id: jurisdictionType === "national" ? null : jurisdictionId,
          start_date: new Date(startDate).toISOString(),
          approval_reference: approvalReference,
          public_contact: publicContact || undefined,
        }),
      });

      setFeedback({ type: "success", message: "Leadership appointment recorded successfully!" });
      setShowAssignModal(false);
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Appointment assignment failed" });
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
    } catch (err: any) {
      setHistoryRecords([]);
    } finally {
      setLoadingHistory(false);
    }
  };

  const handleReplace = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!replaceApp) return;
    setSubmittingReplace(true);
    setFeedback(null);

    try {
      await apiRequest(`/leadership/appointments/${replaceApp.id}/replace`, {
        method: "POST",
        body: JSON.stringify({
          new_user_id: replaceNewUserId,
          effective_date: new Date(replaceEffectiveDate).toISOString(),
          reason: replaceReason,
        }),
      });

      setFeedback({
        type: "success",
        message: `Appointment replaced successfully. Historical records updated and new coordinator assigned.`,
      });
      setReplaceApp(null);
      setReplaceNewUserId("");
      setReplaceReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Replacement failed" });
    } finally {
      setSubmittingReplace(false);
    }
  };

  const handleStatusChange = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!statusAppointmentId) return;

    setUpdatingStatus(true);
    setFeedback(null);

    try {
      await apiRequest(`/leadership/appointments/${statusAppointmentId}/status`, {
        method: "PUT",
        body: JSON.stringify({
          status: newStatus,
          reason: statusReason,
        }),
      });

      setFeedback({ type: "success", message: `Appointment status updated to ${newStatus}. Permissions updated immediately.` });
      setStatusAppointmentId(null);
      setStatusReason("");
      await loadData();
    } catch (err: any) {
      setFeedback({ type: "error", message: err.message || "Status update failed" });
    } finally {
      setUpdatingStatus(false);
    }
  };

  if (isLoading || loading) return <Spinner text="Loading appointments..." />;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      <Link
        href="/admin"
        className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900"
      >
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to Admin Workspace
      </Link>

      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
              Leadership Appointments & Governance
            </h1>
            {isSuperAdmin && (
              <Badge variant="primary" size="sm">
                SUPER ADMIN
              </Badge>
            )}
          </div>
          <p className="text-xs text-gray-600 mt-1">
            Assign authorized roles by geographic scope, enforce reporting hierarchy, and record appointment history.
          </p>
        </div>

        <button
          type="button"
          onClick={() => setShowAssignModal(true)}
          className="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-xl text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 shadow-sm transition"
        >
          <PlusCircle className="w-4 h-4" />
          Assign Leadership Role
        </button>
      </div>

      {!isSuperAdmin && (
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-4 flex items-start gap-3 text-xs text-amber-900">
          <ShieldAlert className="w-5 h-5 text-amber-700 flex-shrink-0 mt-0.5" />
          <div>
            <span className="font-bold block">Super Administrator Boundary Enforced</span>
            <span className="block mt-0.5">
              Only the Super Administrator may appoint, activate, replace, or revoke Zonal and State Coordinators, or modify their geographic scopes and dates. National Administrators may view records and manage lower LGA appointments.
            </span>
          </div>
        </div>
      )}

      {feedback && <Alert type={feedback.type}>{feedback.message}</Alert>}

      {/* Assign Modal */}
      {showAssignModal && (
        <div className="p-6 bg-white rounded-2xl border-2 border-emerald-400 shadow-xl space-y-5">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">Record Authorized Leadership Appointment</h2>
            <button
              onClick={() => setShowAssignModal(false)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <form onSubmit={handleAssign} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Select Member *</label>
                <select
                  required
                  value={selectedUserId}
                  onChange={(e) => setSelectedUserId(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
                >
                  <option value="">Select registered user...</option>
                  {members.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.display_name} ({m.membership_reference})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Official Role Title *</label>
                <input
                  type="text"
                  required
                  value={roleTitle}
                  onChange={(e) => setRoleTitle(e.target.value)}
                  placeholder="e.g. State Coordinator - Lagos"
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Role Type *</label>
                <select
                  value={roleType}
                  onChange={(e) => {
                    const r = e.target.value;
                    setRoleType(r);
                    if (r === "national_admin") setJurisdictionType("national");
                    else if (r === "zonal_coordinator") setJurisdictionType("zone");
                    else if (r === "state_coordinator") setJurisdictionType("state");
                    else if (r === "lga_representative") setJurisdictionType("lga");
                  }}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
                >
                  <option value="state_coordinator">State Coordinator</option>
                  <option value="zonal_coordinator">Zonal Coordinator</option>
                  <option value="national_admin">National Administrator</option>
                  <option value="lga_representative">LGA Representative</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Jurisdiction Scope *</label>
                <select
                  value={jurisdictionType}
                  onChange={(e) => setJurisdictionType(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
                >
                  <option value="national">National (All Nigeria)</option>
                  <option value="zone">Geopolitical Zone</option>
                  <option value="state">State / FCT</option>
                </select>
              </div>

              {jurisdictionType === "zone" && (
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">Select Geopolitical Zone *</label>
                  <select
                    required
                    value={jurisdictionId}
                    onChange={(e) => setJurisdictionId(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
                  >
                    <option value="">Select zone...</option>
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
                  <label className="block text-xs font-semibold text-gray-700 mb-1">Select State / FCT *</label>
                  <select
                    required
                    value={jurisdictionId}
                    onChange={(e) => setJurisdictionId(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
                  >
                    <option value="">Select state...</option>
                    {states.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.code})
                      </option>
                    ))}
                  </select>
                </div>
              )}

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Start Date *</label>
                <input
                  type="date"
                  required
                  value={startDate}
                  onChange={(e) => setStartDate(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">Approval Reference *</label>
                <input
                  type="text"
                  required
                  value={approvalReference}
                  onChange={(e) => setApprovalReference(e.target.value)}
                  placeholder="e.g. APPT-2026-HQ-001"
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>

              <div className="sm:col-span-2">
                <label className="block text-xs font-semibold text-gray-700 mb-1">Public Contact (Optional)</label>
                <input
                  type="text"
                  value={publicContact}
                  onChange={(e) => setPublicContact(e.target.value)}
                  placeholder="e.g. lagos.ratels@example.org or official WhatsApp line"
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
                />
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-4 border-t">
              <button
                type="button"
                onClick={() => setShowAssignModal(false)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={assigning}
                className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl text-xs font-semibold shadow-sm transition"
              >
                {assigning ? "Recording..." : "Record Appointment"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Replace Coordinator Modal (Super Admin only) */}
      {replaceApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-xl border border-gray-200">
            <div className="flex justify-between items-start border-b pb-3">
              <div>
                <span className="text-xs font-bold text-amber-700 uppercase tracking-wider block">
                  Super Admin Governance
                </span>
                <h2 className="text-base font-bold text-gray-900 mt-0.5">
                  Replace Coordinator: {replaceApp.role_title}
                </h2>
                <span className="text-xs text-gray-500">
                  Current holder: {replaceApp.user_display_name} ({replaceApp.jurisdiction_name || "National"})
                </span>
              </div>
              <button onClick={() => setReplaceApp(null)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleReplace} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Replacement Member *</label>
                <select
                  required
                  value={replaceNewUserId}
                  onChange={(e) => setReplaceNewUserId(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                >
                  <option value="">Select replacement member...</option>
                  {members
                    .filter((m) => m.id !== replaceApp.user_id)
                    .map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.display_name} ({m.membership_reference})
                      </option>
                    ))}
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Effective Handover Date *</label>
                <input
                  type="date"
                  required
                  value={replaceEffectiveDate}
                  onChange={(e) => setReplaceEffectiveDate(e.target.value)}
                  className="w-full border border-gray-300 rounded-lg p-2 bg-white"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Replacement Reason & Handoff Summary *</label>
                <textarea
                  required
                  minLength={5}
                  rows={3}
                  value={replaceReason}
                  onChange={(e) => setReplaceReason(e.target.value)}
                  placeholder="State the formal reason for replacement, governance review, or relocation..."
                  className="w-full border border-gray-300 rounded-lg p-2"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setReplaceApp(null)}
                  className="px-4 py-2 border rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingReplace}
                  className="px-5 py-2 bg-amber-700 hover:bg-amber-800 text-white rounded-xl font-bold transition"
                >
                  {submittingReplace ? "Executing Handover..." : "Confirm Replacement"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Appointment History Modal */}
      {historyApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-4 shadow-xl border border-gray-200">
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
              <div className="space-y-3 max-h-80 overflow-y-auto text-xs">
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

      {/* Modify Status Modal */}
      {statusAppointmentId && (
        <div className="p-6 bg-white rounded-2xl border-2 border-amber-400 shadow-xl space-y-5">
          <div className="flex justify-between items-center">
            <h2 className="text-base font-bold text-gray-900">Modify Appointment Status</h2>
            <button
              onClick={() => setStatusAppointmentId(null)}
              className="text-gray-400 hover:text-gray-600 text-sm font-medium"
            >
              Cancel
            </button>
          </div>

          <form onSubmit={handleStatusChange} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">New Status *</label>
              <select
                value={newStatus}
                onChange={(e) => setNewStatus(e.target.value as any)}
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-emerald-700 focus:border-emerald-700 bg-white"
              >
                <option value="active">Active</option>
                <option value="acting">Acting</option>
                <option value="expired">Expired</option>
                <option value="revoked">Revoked</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Reason for Status Change * (Minimum 3 characters)
              </label>
              <input
                type="text"
                required
                minLength={3}
                value={statusReason}
                onChange={(e) => setStatusReason(e.target.value)}
                placeholder="State formal reason for expiry or revocation..."
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-emerald-700 focus:border-emerald-700"
              />
            </div>

            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setStatusAppointmentId(null)}
                className="px-4 py-2 border border-gray-300 rounded-xl text-xs font-semibold text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={updatingStatus}
                className="px-5 py-2 bg-amber-700 hover:bg-amber-800 text-white rounded-xl text-xs font-semibold transition"
              >
                {updatingStatus ? "Saving..." : "Update Status"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Appointments Table */}
      <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
        <div className="p-4 border-b border-gray-200 bg-gray-50/50 flex justify-between items-center text-xs">
          <span className="font-semibold text-gray-700">Official Movement Appointments</span>
          <span className="text-gray-500">{appointments.length} appointments</span>
        </div>

        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 text-xs">
            <thead className="bg-gray-50 text-gray-600 font-semibold uppercase text-[11px]">
              <tr>
                <th className="px-4 py-3 text-left">Person</th>
                <th className="px-4 py-3 text-left">Role Title</th>
                <th className="px-4 py-3 text-left">Jurisdiction</th>
                <th className="px-4 py-3 text-left">Approval Ref</th>
                <th className="px-4 py-3 text-left">Start Date</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {appointments.map((app) => (
                <tr key={app.id} className="hover:bg-gray-50/60 transition">
                  <td className="px-4 py-3.5 whitespace-nowrap">
                    <div className="font-bold text-gray-900">{app.user_display_name}</div>
                    <div className="text-[11px] font-mono text-gray-500">{app.user_membership_ref}</div>
                  </td>
                  <td className="px-4 py-3.5 whitespace-nowrap font-medium text-gray-800">
                    {app.role_title}
                  </td>
                  <td className="px-4 py-3.5 whitespace-nowrap text-gray-600">
                    {app.jurisdiction_name || "National"}
                  </td>
                  <td className="px-4 py-3.5 whitespace-nowrap font-mono text-gray-500">
                    {app.approval_reference}
                  </td>
                  <td className="px-4 py-3.5 whitespace-nowrap text-gray-500">
                    {new Date(app.start_date).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3.5 whitespace-nowrap">
                    <Badge
                      variant={app.status === "active" ? "success" : app.status === "acting" ? "warning" : "danger"}
                      size="sm"
                    >
                      {app.status.toUpperCase()}
                    </Badge>
                  </td>
                  <td className="px-4 py-3.5 whitespace-nowrap text-right">
                    <div className="flex items-center justify-end gap-1.5">
                      <button
                        type="button"
                        onClick={() => handleOpenHistory(app)}
                        className="px-2 py-1 border border-gray-200 hover:bg-gray-100 text-gray-700 font-semibold rounded-md text-[11px] transition"
                      >
                        History
                      </button>

                      {isSuperAdmin && app.status === "active" && (
                        <button
                          type="button"
                          onClick={() => setReplaceApp(app)}
                          className="px-2 py-1 bg-amber-50 hover:bg-amber-100 text-amber-800 font-semibold rounded-md text-[11px] transition"
                        >
                          Replace
                        </button>
                      )}

                      <button
                        type="button"
                        onClick={() => setStatusAppointmentId(app.id)}
                        className="px-2.5 py-1 border border-gray-300 hover:bg-gray-100 text-gray-700 font-semibold rounded-md text-[11px] transition"
                      >
                        Status
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
