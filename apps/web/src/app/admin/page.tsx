"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  LayoutDashboard,
  Users,
  ShieldCheck,
  Calendar,
  Clock,
  AlertCircle,
  Award,
  CheckCircle2,
  FileCheck2,
  ArrowRight,
  Shield,
  MapPin,
  History,
  Building,
  DollarSign,
  Boxes,
  Store,
  Briefcase,
  Film,
  FolderKanban,
  Camera,
  Send,
  ShieldAlert,
  BarChart3,
  Globe,
  UserCheck,
  UserX,
  Search,
  Filter,
  RefreshCw,
  ChevronRight,
  Settings,
  Database,
  Check,
  XCircle,
  PlusCircle,
  X,
  PauseCircle,
  PlayCircle,
  AlertTriangle,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  AdminOverview,
  SuperAdminDashboardStats,
  StateCoverageRow,
  LGACoverageRow,
  OrganizationSetting,
  User as UserModel,
  AppointmentHistory,
  Zone,
  State,
  LGA,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

type AdminTab =
  | "overview"
  | "members"
  | "zones"
  | "states"
  | "lgas"
  | "activities"
  | "marketplace"
  | "finance"
  | "audit"
  | "system";

export default function AdminOverviewPage() {
  const router = useRouter();
  const { user, isCoordinator, isSuperAdmin, isLoading } = useAuth();

  // Active view tab for Super Admin
  const [activeTab, setActiveTab] = useState<AdminTab>("overview");

  // Super Admin statistics and coverage states
  const [stats, setStats] = useState<SuperAdminDashboardStats | null>(null);
  const [stateCoverage, setStateCoverage] = useState<StateCoverageRow[]>([]);
  const [lgaCoverage, setLgaCoverage] = useState<LGACoverageRow[]>([]);
  const [selectedStateForLga, setSelectedStateForLga] = useState<string>("");
  const [loadingLgas, setLoadingLgas] = useState(false);

  // Regional Coordinator fallback overview
  const [overview, setOverview] = useState<AdminOverview | null>(null);

  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Search & Filter state for lists
  const [stateSearch, setStateSearch] = useState("");
  const [selectedZoneFilter, setSelectedZoneFilter] = useState("all");
  const [lgaSearch, setLgaSearch] = useState("");

  // Status Filter Pills for Directories
  const [zoneStatusFilter, setZoneStatusFilter] = useState<"all" | "active" | "vacant" | "suspended">("all");
  const [stateStatusFilter, setStateStatusFilter] = useState<"all" | "active" | "vacant" | "suspended">("all");
  const [lgaStatusFilter, setLgaStatusFilter] = useState<"all" | "active" | "vacant" | "suspended">("all");

  // Geographic reference lists for appointment dropdowns
  const [allZones, setAllZones] = useState<Zone[]>([]);
  const [allStates, setAllStates] = useState<State[]>([]);
  const [stateLgas, setStateLgas] = useState<LGA[]>([]);

  // Leadership Management Modal States
  const [leadershipSuccess, setLeadershipSuccess] = useState<string | null>(null);

  // Appointment Modal
  const [showAppointModal, setShowAppointModal] = useState(false);
  const [appointStep, setAssignStep] = useState<"form" | "review">("form");
  const [appointRoleType, setAppointRoleType] = useState<string>("state_coordinator");
  const [appointRoleTitle, setAppointRoleTitle] = useState<string>("");
  const [appointJurisdictionType, setAppointJurisdictionType] = useState<string>("state");
  const [appointJurisdictionId, setAppointJurisdictionId] = useState<string>("");
  const [appointJurisdictionLabel, setAppointJurisdictionLabel] = useState<string>("");
  const [appointStateForLga, setAppointStateForLga] = useState<string>("");
  const [appointStartDate, setAppointStartDate] = useState<string>(new Date().toISOString().split("T")[0]);
  const [appointRef, setAppointRef] = useState<string>("");
  const [appointContact, setAppointContact] = useState<string>("");
  const [appointMemberSearch, setAppointMemberSearch] = useState<string>("");
  const [appointMemberResults, setAppointMemberResults] = useState<UserModel[]>([]);
  const [appointSearching, setAppointSearching] = useState(false);
  const [appointSelectedUser, setAppointSelectedUser] = useState<UserModel | null>(null);
  const [submittingAppoint, setSubmittingAppoint] = useState(false);

  // Replacement Modal
  const [replaceTarget, setReplaceTarget] = useState<{
    appointmentId: string;
    currentHolder: string;
    roleTitle: string;
    jurisdictionName: string;
  } | null>(null);
  const [replaceMemberSearch, setReplaceMemberSearch] = useState<string>("");
  const [replaceMemberResults, setReplaceMemberResults] = useState<UserModel[]>([]);
  const [replaceSearching, setReplaceSearching] = useState(false);
  const [replaceSelectedUser, setReplaceSelectedUser] = useState<UserModel | null>(null);
  const [replaceEffectiveDate, setReplaceEffectiveDate] = useState<string>(new Date().toISOString().split("T")[0]);
  const [replaceReason, setReplaceReason] = useState<string>("");
  const [submittingReplace, setSubmittingReplace] = useState(false);

  // Suspend Modal
  const [suspendTarget, setSuspendTarget] = useState<{
    appointmentId: string;
    coordinatorName: string;
    roleTitle: string;
  } | null>(null);
  const [suspendReason, setSuspendReason] = useState<string>("");
  const [submittingSuspend, setSubmittingSuspend] = useState(false);

  // Reactivate Modal
  const [reactivateTarget, setReactivateTarget] = useState<{
    appointmentId: string;
    coordinatorName: string;
    roleTitle: string;
  } | null>(null);
  const [reactivateReason, setReactivateReason] = useState<string>("");
  const [submittingReactivate, setSubmittingReactivate] = useState(false);

  // Revoke Modal
  const [revokeTarget, setRevokeTarget] = useState<{
    appointmentId: string;
    coordinatorName: string;
    roleTitle: string;
  } | null>(null);
  const [revokeReason, setRevokeReason] = useState<string>("");
  const [submittingRevoke, setSubmittingRevoke] = useState(false);

  // History Modal
  const [historyTarget, setHistoryTarget] = useState<{
    appointmentId: string;
    roleTitle: string;
    jurisdictionName: string;
  } | null>(null);
  const [historyRecords, setHistoryRecords] = useState<AppointmentHistory[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // System Seeding Status
  const [seedingGeography, setSeedingGeography] = useState(false);
  const [seedSuccessMessage, setSeedSuccessMessage] = useState<string | null>(null);

  // Organization-Wide Settings (Super Admin privileged)
  const [settings, setSettings] = useState<OrganizationSetting[]>([]);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editingValue, setEditingValue] = useState<string>("");
  const [savingSetting, setSavingSetting] = useState(false);
  const [settingSuccess, setSettingSuccess] = useState<string | null>(null);

  const loadData = async () => {
    try {
      setError(null);
      if (isSuperAdmin || user?.primary_role === "super_admin") {
        const [statsData, statesData, settingsData, zonesData, statesGeoData] = await Promise.all([
          apiRequest<SuperAdminDashboardStats>("/admin/super/dashboard"),
          apiRequest<StateCoverageRow[]>("/admin/super/coverage/states").catch(() => []),
          apiRequest<OrganizationSetting[]>("/admin/settings").catch(() => []),
          apiRequest<Zone[]>("/geography/zones").catch(() => []),
          apiRequest<State[]>("/geography/states").catch(() => []),
        ]);
        setStats(statsData);
        setStateCoverage(statesData || []);
        setSettings(settingsData || []);
        setAllZones(zonesData || []);
        setAllStates(statesGeoData || []);
        if (statesData && statesData.length > 0 && !selectedStateForLga) {
          setSelectedStateForLga(statesData[0].state_id);
        }
      } else {
        const data = await apiRequest<AdminOverview>("/admin/overview");
        setOverview(data);
      }
    } catch (err: any) {
      setError(err.message || "Failed to load admin dashboard metrics");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  // Debounced search for appoint member
  useEffect(() => {
    if (!appointMemberSearch.trim()) {
      setAppointMemberResults([]);
      return;
    }
    const timer = setTimeout(async () => {
      setAppointSearching(true);
      try {
        const res = await apiRequest<UserModel[]>(`/members?search=${encodeURIComponent(appointMemberSearch.trim())}&limit=10`);
        setAppointMemberResults(res || []);
      } catch {
        setAppointMemberResults([]);
      } finally {
        setAppointSearching(false);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [appointMemberSearch]);

  // Debounced search for replace member
  useEffect(() => {
    if (!replaceMemberSearch.trim()) {
      setReplaceMemberResults([]);
      return;
    }
    const timer = setTimeout(async () => {
      setReplaceSearching(true);
      try {
        const res = await apiRequest<UserModel[]>(`/members?search=${encodeURIComponent(replaceMemberSearch.trim())}&limit=10`);
        setReplaceMemberResults(res || []);
      } catch {
        setReplaceMemberResults([]);
      } finally {
        setReplaceSearching(false);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [replaceMemberSearch]);

  // Fetch LGAs when state changes for LGA appointment
  useEffect(() => {
    if (appointJurisdictionType === "lga" && appointStateForLga) {
      apiRequest<LGA[]>(`/geography/states/${appointStateForLga}/lgas`)
        .then((data) => setStateLgas(data || []))
        .catch(() => setStateLgas([]));
    }
  }, [appointJurisdictionType, appointStateForLga]);

  const openAppointModal = (prefill?: {
    roleType: string;
    jurisdictionType: string;
    jurisdictionId: string;
    jurisdictionName: string;
  }) => {
    setError(null);
    setLeadershipSuccess(null);
    setAssignStep("form");
    setAppointSelectedUser(null);
    setAppointMemberSearch("");
    setAppointStartDate(new Date().toISOString().split("T")[0]);
    setAppointRef(`APPT-${Date.now().toString().slice(-6)}`);
    setAppointContact("");

    if (prefill) {
      setAppointRoleType(prefill.roleType);
      setAppointJurisdictionType(prefill.jurisdictionType);
      setAppointJurisdictionId(prefill.jurisdictionId);
      setAppointJurisdictionLabel(prefill.jurisdictionName);
      setAppointRoleTitle(`${prefill.roleType.replace(/_/g, " ").toUpperCase()} - ${prefill.jurisdictionName}`);
      if (prefill.jurisdictionType === "lga") {
        setAppointStateForLga(selectedStateForLga);
      }
    } else {
      setAppointRoleType("state_coordinator");
      setAppointJurisdictionType("state");
      setAppointJurisdictionId(allStates[0]?.id || "");
      setAppointJurisdictionLabel(allStates[0]?.name || "");
      setAppointRoleTitle(`State Coordinator - ${allStates[0]?.name || ""}`);
    }
    setShowAppointModal(true);
  };

  const openHistoryModal = async (target: { appointmentId: string; roleTitle: string; jurisdictionName: string }) => {
    setHistoryTarget(target);
    setLoadingHistory(true);
    try {
      const hist = await apiRequest<AppointmentHistory[]>(`/leadership/appointments/${target.appointmentId}/history`);
      setHistoryRecords(hist || []);
    } catch {
      setHistoryRecords([]);
    } finally {
      setLoadingHistory(false);
    }
  };

  const handleAppointSubmit = async () => {
    if (!appointSelectedUser) {
      setError("Please search and select a registered member.");
      return;
    }
    setSubmittingAppoint(true);
    setError(null);
    try {
      await apiRequest("/leadership/appointments", {
        method: "POST",
        body: JSON.stringify({
          user_id: appointSelectedUser.id,
          role_title: appointRoleTitle,
          role_type: appointRoleType,
          jurisdiction_type: appointJurisdictionType,
          jurisdiction_id: appointJurisdictionType === "national" ? null : appointJurisdictionId,
          start_date: new Date(appointStartDate).toISOString(),
          approval_reference: appointRef,
          public_contact: appointContact || undefined,
        }),
      });
      setLeadershipSuccess(`Successfully appointed ${appointSelectedUser.display_name} to ${appointRoleTitle}.`);
      setShowAppointModal(false);
      await loadData();
      if (activeTab === "lgas" && selectedStateForLga) {
        const lgasData = await apiRequest<LGACoverageRow[]>(`/admin/super/coverage/lgas?state_id=${encodeURIComponent(selectedStateForLga)}`);
        setLgaCoverage(lgasData || []);
      }
    } catch (err: any) {
      setError(err.message || "Failed to record coordinator appointment.");
    } finally {
      setSubmittingAppoint(false);
    }
  };

  const handleReplaceSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!replaceTarget || !replaceSelectedUser) return;
    setSubmittingReplace(true);
    setError(null);
    try {
      await apiRequest(`/leadership/appointments/${replaceTarget.appointmentId}/replace`, {
        method: "POST",
        body: JSON.stringify({
          new_user_id: replaceSelectedUser.id,
          effective_date: new Date(replaceEffectiveDate).toISOString(),
          reason: replaceReason,
        }),
      });
      setLeadershipSuccess(`Successfully replaced coordinator for ${replaceTarget.roleTitle}.`);
      setReplaceTarget(null);
      await loadData();
      if (activeTab === "lgas" && selectedStateForLga) {
        const lgasData = await apiRequest<LGACoverageRow[]>(`/admin/super/coverage/lgas?state_id=${encodeURIComponent(selectedStateForLga)}`);
        setLgaCoverage(lgasData || []);
      }
    } catch (err: any) {
      setError(err.message || "Replacement failed.");
    } finally {
      setSubmittingReplace(false);
    }
  };

  const handleSuspendSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!suspendTarget) return;
    setSubmittingSuspend(true);
    setError(null);
    try {
      await apiRequest(`/leadership/appointments/${suspendTarget.appointmentId}/suspend`, {
        method: "POST",
        body: JSON.stringify({ reason: suspendReason || "Suspended by Super Administrator" }),
      });
      setLeadershipSuccess(`Appointment for ${suspendTarget.coordinatorName} suspended.`);
      setSuspendTarget(null);
      await loadData();
      if (activeTab === "lgas" && selectedStateForLga) {
        const lgasData = await apiRequest<LGACoverageRow[]>(`/admin/super/coverage/lgas?state_id=${encodeURIComponent(selectedStateForLga)}`);
        setLgaCoverage(lgasData || []);
      }
    } catch (err: any) {
      setError(err.message || "Suspension failed.");
    } finally {
      setSubmittingSuspend(false);
    }
  };

  const handleReactivateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reactivateTarget) return;
    setSubmittingReactivate(true);
    setError(null);
    try {
      await apiRequest(`/leadership/appointments/${reactivateTarget.appointmentId}/reactivate`, {
        method: "POST",
        body: JSON.stringify({ reason: reactivateReason || "Reactivated by Super Administrator" }),
      });
      setLeadershipSuccess(`Appointment for ${reactivateTarget.coordinatorName} reactivated.`);
      setReactivateTarget(null);
      await loadData();
      if (activeTab === "lgas" && selectedStateForLga) {
        const lgasData = await apiRequest<LGACoverageRow[]>(`/admin/super/coverage/lgas?state_id=${encodeURIComponent(selectedStateForLga)}`);
        setLgaCoverage(lgasData || []);
      }
    } catch (err: any) {
      setError(err.message || "Reactivation failed.");
    } finally {
      setSubmittingReactivate(false);
    }
  };

  const handleRevokeSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!revokeTarget) return;
    setSubmittingRevoke(true);
    setError(null);
    try {
      await apiRequest(`/leadership/appointments/${revokeTarget.appointmentId}/revoke`, {
        method: "POST",
        body: JSON.stringify({ reason: revokeReason || "Revoked by Super Administrator" }),
      });
      setLeadershipSuccess(`Appointment for ${revokeTarget.coordinatorName} revoked.`);
      setRevokeTarget(null);
      await loadData();
      if (activeTab === "lgas" && selectedStateForLga) {
        const lgasData = await apiRequest<LGACoverageRow[]>(`/admin/super/coverage/lgas?state_id=${encodeURIComponent(selectedStateForLga)}`);
        setLgaCoverage(lgasData || []);
      }
    } catch (err: any) {
      setError(err.message || "Revocation failed.");
    } finally {
      setSubmittingRevoke(false);
    }
  };

  const handleSaveSetting = async (key: string, value: string) => {
    setSavingSetting(true);
    setSettingSuccess(null);
    try {
      const updated = await apiRequest<OrganizationSetting>(
        `/admin/settings/${encodeURIComponent(key)}`,
        {
          method: "PUT",
          body: JSON.stringify({ value }),
        }
      );
      setSettings((prev) => prev.map((s) => (s.key === key ? updated : s)));
      setEditingKey(null);
      setSettingSuccess(`Organization setting '${key}' successfully updated.`);
      // Refresh stats to include latest audit trail entry
      const updatedStats = await apiRequest<SuperAdminDashboardStats>("/admin/super/dashboard");
      setStats(updatedStats);
    } catch (err: any) {
      setError(err.message || "Failed to update organization setting");
    } finally {
      setSavingSetting(false);
    }
  };

  useEffect(() => {
    if (!isLoading && (!user || (!isCoordinator && !isSuperAdmin && user.primary_role !== "super_admin"))) {
      router.push("/login?redirect=/admin");
      return;
    }

    if (user) {
      loadData();
    }
  }, [user, isCoordinator, isSuperAdmin, isLoading, router]);

  // Load LGAs when state selection changes in LGA Tab
  useEffect(() => {
    if (activeTab === "lgas" && selectedStateForLga) {
      async function fetchLgas() {
        setLoadingLgas(true);
        try {
          const data = await apiRequest<LGACoverageRow[]>(
            `/admin/super/coverage/lgas?state_id=${encodeURIComponent(selectedStateForLga)}`
          );
          setLgaCoverage(data || []);
        } catch (err: any) {
          console.error("Failed to load LGA coverage", err);
        } finally {
          setLoadingLgas(false);
        }
      }
      fetchLgas();
    }
  }, [activeTab, selectedStateForLga]);

  const handleSeedGeography = async () => {
    setSeedingGeography(true);
    setSeedSuccessMessage(null);
    try {
      const res = await apiRequest<{ status: string; message: string; zones: number; states: number; lgas: number }>(
        "/admin/geography/seed",
        { method: "POST" }
      );
      setSeedSuccessMessage(
        `Authoritative reference geography verified: ${res.zones} Zones, ${res.states} States/FCT, ${res.lgas} LGAs successfully synced.`
      );
      // Reload stats
      const updated = await apiRequest<SuperAdminDashboardStats>("/admin/super/dashboard");
      setStats(updated);
    } catch (err: any) {
      setError(err.message || "Failed to seed authoritative reference geography");
    } finally {
      setSeedingGeography(false);
    }
  };

  if (isLoading || loading) return <Spinner text="Loading administration workspace..." />;
  if (!user) return null;

  const isSuper = isSuperAdmin || user.primary_role === "super_admin";

  // ---------------------------------------------------------------------------
  // RENDER: Regional Coordinator Fallback View (Non-Super Admin)
  // ---------------------------------------------------------------------------
  if (!isSuper && overview) {
    const unverifiedQueue = overview.unverified_participations_queue ?? 0;
    const draftQueue = overview.draft_venues_review_queue ?? 0;
    const appealsQueue = overview.pending_appeals_queue ?? 0;
    const totalMembers = overview.total_members ?? 0;
    const pendingVerifications = overview.pending_member_verifications ?? 0;
    const activeCoordinators = overview.active_coordinators ?? 0;
    const publishedVenues = overview.published_activities ?? 0;

    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
<Link href="/coordinator" className="inline-block text-emerald-700 underline">Open coordinator operational workspace</Link>

        <div className="bg-emerald-950 text-white rounded-2xl p-6 sm:p-8 shadow-sm flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border border-emerald-800">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <span className="text-xs uppercase font-bold tracking-wider text-emerald-400">
                Administrative Context
              </span>
              <Badge variant="primary" size="sm">
                {(overview.current_role || user.primary_role || "COORDINATOR")
                  .replace(/_/g, " ")
                  .toUpperCase()}
              </Badge>
            </div>
            <h1 className="text-2xl sm:text-3xl font-extrabold flex items-center gap-2">
              <MapPin className="w-6 h-6 text-emerald-400" />
              {overview.current_jurisdiction_name || "Regional Jurisdiction"}
            </h1>
            <p className="text-xs text-emerald-200">
              Coordinator: {user.display_name} ({user.membership_reference})
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <Link
              href="/admin/activities"
              className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-semibold rounded-lg transition"
            >
              Create Venue
            </Link>
            <Link
              href="/admin/reviews"
              className="px-4 py-2 bg-emerald-400 hover:bg-emerald-300 text-emerald-950 text-xs font-bold rounded-lg transition"
            >
              Review Queue ({unverifiedQueue})
            </Link>
          </div>
        </div>

        {error && <Alert type="error">{error}</Alert>}

        {/* Attention Section */}
        <div className="space-y-4">
          <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
            <Clock className="w-5 h-5 text-emerald-700" />
            Needs Your Attention
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-gray-500 font-semibold uppercase">Participation Queue</span>
              <div className="text-2xl font-black text-gray-900">{unverifiedQueue}</div>
              <p className="text-xs text-gray-600">Volunteer activity attendance submissions awaiting review.</p>
              <Link href="/admin/reviews" className="inline-flex items-center text-xs font-bold text-emerald-800 hover:text-emerald-950">
                Open Review Queue <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>
            <div className="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-gray-500 font-semibold uppercase">Draft Cleanup Venues</span>
              <div className="text-2xl font-black text-gray-900">{draftQueue}</div>
              <p className="text-xs text-gray-600">Proposed local venues awaiting inspection & publish.</p>
              <Link href="/admin/activities" className="inline-flex items-center text-xs font-bold text-emerald-800 hover:text-emerald-950">
                Inspect Venues <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>
            <div className="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-gray-500 font-semibold uppercase">Appeals Queue</span>
              <div className="text-2xl font-black text-gray-900">{appealsQueue}</div>
              <p className="text-xs text-gray-600">Rejected claims appealed by members in jurisdiction.</p>
              <Link href="/admin/reviews" className="inline-flex items-center text-xs font-bold text-emerald-800 hover:text-emerald-950">
                Review Appeals <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>
          </div>
        </div>

        {/* Real Metrics */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Registered Members</span>
            <div className="text-xl font-bold text-gray-900">{totalMembers}</div>
          </div>
          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Pending Verification</span>
            <div className="text-xl font-bold text-gray-900">{pendingVerifications}</div>
          </div>
          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Active Coordinators</span>
            <div className="text-xl font-bold text-gray-900">{activeCoordinators}</div>
          </div>
          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Published Venues</span>
            <div className="text-xl font-bold text-gray-900">{publishedVenues}</div>
          </div>
        </div>
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // RENDER: Nationwide Super Admin Dashboard (Full Foundation)
  // ---------------------------------------------------------------------------
  if (!stats) return null;

  // Filtered Zones list
  const filteredZones = (stats.zone_coverage || []).filter((z) => {
    if (zoneStatusFilter === "all") return true;
    if (zoneStatusFilter === "active") return z.coordinator_name && z.appointment_status !== "suspended";
    if (zoneStatusFilter === "suspended") return z.appointment_status === "suspended";
    if (zoneStatusFilter === "vacant") return !z.coordinator_name;
    return true;
  });

  // Filtered States list
  const filteredStates = stateCoverage.filter((st) => {
    const matchesSearch =
      st.state_name.toLowerCase().includes(stateSearch.toLowerCase()) ||
      st.state_code.toLowerCase().includes(stateSearch.toLowerCase()) ||
      (st.coordinator_name || "").toLowerCase().includes(stateSearch.toLowerCase());
    const matchesZone = selectedZoneFilter === "all" || st.zone_code === selectedZoneFilter;
    const matchesStatus =
      stateStatusFilter === "all" ||
      (stateStatusFilter === "active" && st.coordinator_name && st.appointment_status !== "suspended") ||
      (stateStatusFilter === "suspended" && st.appointment_status === "suspended") ||
      (stateStatusFilter === "vacant" && !st.coordinator_name);
    return matchesSearch && matchesZone && matchesStatus;
  });

  // Filtered LGAs list
  const filteredLgas = lgaCoverage.filter((lg) => {
    const matchesSearch =
      lg.lga_name.toLowerCase().includes(lgaSearch.toLowerCase()) ||
      (lg.coordinator_name || "").toLowerCase().includes(lgaSearch.toLowerCase());
    const matchesStatus =
      lgaStatusFilter === "all" ||
      (lgaStatusFilter === "active" && lg.coordinator_name && lg.appointment_status !== "suspended") ||
      (lgaStatusFilter === "suspended" && lg.appointment_status === "suspended") ||
      (lgaStatusFilter === "vacant" && !lg.coordinator_name);
    return matchesSearch && matchesStatus;
  });

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
      {/* 1. Header Banner & Identity */}
      <div className="bg-gradient-to-r from-emerald-950 via-emerald-900 to-emerald-950 text-white rounded-2xl p-6 sm:p-8 shadow-md border border-emerald-800 flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="px-2.5 py-0.5 rounded-full text-[10px] font-black uppercase tracking-wider bg-emerald-500 text-emerald-950">
              National Command
            </span>
            <Badge variant="warning" size="sm">
              SUPER ADMIN PRIVILEGED
            </Badge>
            <span className="text-xs text-emerald-300 font-medium">
              Ratels Worldwide Management Platform
            </span>
          </div>
          <h1 className="text-2xl sm:text-4xl font-extrabold tracking-tight flex items-center gap-2">
            <Globe className="w-8 h-8 text-emerald-400" />
            Nationwide Operational Command Center
          </h1>
          <p className="text-xs sm:text-sm text-emerald-200">
            Authoritative national oversight: 6 Geopolitical Zones, 36 States + FCT, and 774 Local Government Areas.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <button
            onClick={() => {
              setRefreshing(true);
              loadData();
            }}
            disabled={refreshing}
            className="inline-flex items-center px-3.5 py-2 rounded-lg bg-emerald-800/80 hover:bg-emerald-700 text-white text-xs font-semibold transition border border-emerald-700 disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${refreshing ? "animate-spin" : ""}`} />
            Refresh Live Metrics
          </button>
          <Link
            href="/admin/leadership"
            className="inline-flex items-center px-4 py-2 rounded-lg bg-emerald-400 hover:bg-emerald-300 text-emerald-950 text-xs font-bold transition shadow-sm"
          >
            <ShieldCheck className="w-4 h-4 mr-1.5" />
            Appoint / Replace Coordinators
          </Link>
        </div>
      </div>

      {error && <Alert type="error">{error}</Alert>}
      {seedSuccessMessage && <Alert type="success">{seedSuccessMessage}</Alert>}
      {leadershipSuccess && <Alert type="success">{leadershipSuccess}</Alert>}

      {/* 2. Navigation Tabs (Desktop, Tablet, Mobile responsive) */}
      <div className="border-b border-gray-200 overflow-x-auto scrollbar-none">
        <nav className="flex space-x-2 sm:space-x-4 min-w-max pb-2" aria-label="Tabs">
          {[
            { id: "overview", label: "National Overview", icon: LayoutDashboard },
            { id: "zones", label: `Zones (${stats.active_zones}/6)`, icon: Globe },
            { id: "states", label: `States (${stats.covered_states}/37)`, icon: MapPin },
            { id: "lgas", label: `LGAs (${stats.covered_lgas}/774)`, icon: Building },
            { id: "members", label: `Members (${stats.total_members})`, icon: Users },
            { id: "activities", label: `Cleanups (${stats.upcoming_cleanups_count})`, icon: Calendar },
            { id: "marketplace", label: "Marketplace & Inventory", icon: Store },
            { id: "finance", label: "Financial Ledger", icon: DollarSign },
            { id: "audit", label: "Privileged Audit Logs", icon: History },
            { id: "system", label: "System Administration", icon: Settings },
          ].map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as AdminTab)}
                className={`flex items-center gap-2 px-3.5 py-2.5 rounded-xl text-xs font-bold transition-all ${
                  isActive
                    ? "bg-emerald-800 text-white shadow-sm"
                    : "text-gray-600 hover:text-gray-900 hover:bg-gray-100"
                }`}
              >
                <Icon className={`w-4 h-4 ${isActive ? "text-emerald-300" : "text-gray-400"}`} />
                {tab.label}
              </button>
            );
          })}
        </nav>
      </div>

      {/* 3. TAB CONTENT: Overview */}
      {activeTab === "overview" && (
        <div className="space-y-8">
          {/* Top Line Vital Statistics */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 sm:gap-6">
            {/* Total Members */}
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-2">
              <div className="flex justify-between items-center text-xs text-gray-500 font-semibold">
                <span>Total Registered Members</span>
                <Users className="w-4 h-4 text-emerald-600" />
              </div>
              <div className="text-3xl font-black text-gray-900 tracking-tight">
                {stats.total_members.toLocaleString()}
              </div>
              <div className="flex items-center gap-2 text-[11px]">
                <span className="text-emerald-700 font-semibold">
                  {stats.verified_members.toLocaleString()} verified
                </span>
                <span className="text-gray-300">•</span>
                <span className="text-amber-700">
                  {stats.pending_member_verifications.toLocaleString()} pending
                </span>
              </div>
            </div>

            {/* Active Coordinators */}
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-2">
              <div className="flex justify-between items-center text-xs text-gray-500 font-semibold">
                <span>Active Coordinators</span>
                <ShieldCheck className="w-4 h-4 text-emerald-600" />
              </div>
              <div className="text-3xl font-black text-gray-900 tracking-tight">
                {stats.active_coordinators.toLocaleString()}
              </div>
              <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-gray-500">
                <span className="bg-gray-100 px-1.5 py-0.5 rounded font-medium">
                  {stats.zonal_coordinators_count} Zonal
                </span>
                <span className="bg-gray-100 px-1.5 py-0.5 rounded font-medium">
                  {stats.state_coordinators_count} State
                </span>
                <span className="bg-gray-100 px-1.5 py-0.5 rounded font-medium">
                  {stats.lga_coordinators_count} LGA
                </span>
              </div>
            </div>

            {/* Geographic Coverage */}
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-2">
              <div className="flex justify-between items-center text-xs text-gray-500 font-semibold">
                <span>State & FCT Coverage</span>
                <MapPin className="w-4 h-4 text-emerald-600" />
              </div>
              <div className="text-3xl font-black text-gray-900 tracking-tight">
                {stats.covered_states} <span className="text-sm font-normal text-gray-500">/ 37</span>
              </div>
              <div className="w-full bg-gray-100 rounded-full h-1.5">
                <div
                  className="bg-emerald-600 h-1.5 rounded-full"
                  style={{ width: `${Math.min(100, (stats.covered_states / 37) * 100)}%` }}
                />
              </div>
              <div className="flex justify-between text-[10px] text-gray-400">
                <span>{stats.covered_lgas} of 774 LGAs represented</span>
                <span>{Math.round((stats.covered_states / 37) * 100)}%</span>
              </div>
            </div>

            {/* Upcoming Activities */}
            <div className="bg-white p-5 rounded-2xl border border-gray-200 shadow-sm space-y-2">
              <div className="flex justify-between items-center text-xs text-gray-500 font-semibold">
                <span>Upcoming Cleanups</span>
                <Calendar className="w-4 h-4 text-emerald-600" />
              </div>
              <div className="text-3xl font-black text-gray-900 tracking-tight">
                {stats.upcoming_cleanups_count}
              </div>
              <div className="flex items-center gap-2 text-[11px] text-gray-500">
                <span>{stats.published_activities} published total</span>
                <span className="text-gray-300">•</span>
                <span className="text-amber-700 font-medium">
                  {stats.draft_venues_review_queue} drafts pending
                </span>
              </div>
            </div>
          </div>

          {/* Operational Review Queues Grid */}
          <div className="bg-gray-50 p-6 rounded-2xl border border-gray-200 space-y-4">
            <h2 className="text-sm font-bold text-gray-900 flex items-center gap-2 uppercase tracking-wider">
              <Clock className="w-4 h-4 text-emerald-700" />
              Nationwide Operational Queues Requiring Triage
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex items-center justify-between">
                <div>
                  <div className="text-xs text-gray-500 font-semibold">Participation Claims Queue</div>
                  <div className="text-2xl font-black text-gray-900">{stats.unverified_participations_queue}</div>
                  <div className="text-[11px] text-gray-500">Volunteer cleanup attendance claims</div>
                </div>
                <Link
                  href="/admin/reviews"
                  className="px-3 py-1.5 rounded-lg bg-emerald-50 text-emerald-800 hover:bg-emerald-100 text-xs font-bold transition"
                >
                  Review
                </Link>
              </div>

              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex items-center justify-between">
                <div>
                  <div className="text-xs text-gray-500 font-semibold">Draft Venue Proposals</div>
                  <div className="text-2xl font-black text-gray-900">{stats.draft_venues_review_queue}</div>
                  <div className="text-[11px] text-gray-500">Proposed local cleanup venues</div>
                </div>
                <Link
                  href="/admin/activities"
                  className="px-3 py-1.5 rounded-lg bg-emerald-50 text-emerald-800 hover:bg-emerald-100 text-xs font-bold transition"
                >
                  Inspect
                </Link>
              </div>

              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex items-center justify-between">
                <div>
                  <div className="text-xs text-gray-500 font-semibold">Member Appeals Queue</div>
                  <div className="text-2xl font-black text-gray-900">{stats.pending_appeals_queue}</div>
                  <div className="text-[11px] text-gray-500">Contested participation claims</div>
                </div>
                <Link
                  href="/admin/reviews"
                  className="px-3 py-1.5 rounded-lg bg-amber-50 text-amber-800 hover:bg-amber-100 text-xs font-bold transition"
                >
                  Appeals
                </Link>
              </div>
            </div>
          </div>

          {/* Upcoming Cleanup Campaigns Table */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
            <div className="flex justify-between items-center">
              <div>
                <h3 className="text-base font-bold text-gray-900 flex items-center gap-2">
                  <Calendar className="w-5 h-5 text-emerald-700" />
                  Upcoming Published Cleanup Venues Across Nigeria
                </h3>
                <p className="text-xs text-gray-500">
                  Approved, public-facing cleanup activities scheduled across Nigerian States and LGAs.
                </p>
              </div>
              <Link
                href="/admin/activities"
                className="text-xs font-bold text-emerald-700 hover:text-emerald-900 inline-flex items-center"
              >
                Manage All Venues <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>

            {stats.upcoming_activities.length === 0 ? (
              <div className="text-center py-8 text-xs text-gray-500 bg-gray-50 rounded-xl">
                No upcoming published cleanups scheduled at this time.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200 text-xs">
                  <thead className="bg-gray-50 text-gray-500 font-bold uppercase">
                    <tr>
                      <th className="px-4 py-3 text-left">Activity / Venue</th>
                      <th className="px-4 py-3 text-left">Jurisdiction (State / LGA)</th>
                      <th className="px-4 py-3 text-left">Scheduled Date & Time</th>
                      <th className="px-4 py-3 text-left">Coordinator</th>
                      <th className="px-4 py-3 text-right">Volunteers</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {stats.upcoming_activities.map((act) => (
                      <tr key={act.id} className="hover:bg-gray-50 transition">
                        <td className="px-4 py-3 font-semibold text-gray-900">
                          {act.title}
                          <div className="text-[10px] text-gray-500 font-normal">{act.address}</div>
                        </td>
                        <td className="px-4 py-3 text-gray-700">
                          {act.state_name} — <span className="text-gray-500">{act.lga_name}</span>
                        </td>
                        <td className="px-4 py-3 text-gray-600">
                          {new Date(act.start_time).toLocaleDateString("en-NG", {
                            weekday: "short",
                            year: "numeric",
                            month: "short",
                            day: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </td>
                        <td className="px-4 py-3 text-gray-700">{act.coordinator_name}</td>
                        <td className="px-4 py-3 text-right font-bold text-emerald-700">
                          {act.registered_count} / {act.capacity}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Recent Privileged Administrative Audit Stream */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
            <div className="flex justify-between items-center">
              <div>
                <h3 className="text-base font-bold text-gray-900 flex items-center gap-2">
                  <History className="w-5 h-5 text-emerald-700" />
                  Recent Privileged Administrative Actions
                </h3>
                <p className="text-xs text-gray-500">
                  Immutable audit records of coordinator appointments, status transitions, and administrative approvals.
                </p>
              </div>
              <Link
                href="/admin/audit"
                className="text-xs font-bold text-emerald-700 hover:text-emerald-900 inline-flex items-center"
              >
                Full Audit Trail <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>

            {stats.recent_audit_actions.length === 0 ? (
              <div className="text-center py-8 text-xs text-gray-500 bg-gray-50 rounded-xl">
                No recent administrative actions recorded.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200 text-xs">
                  <thead className="bg-gray-50 text-gray-500 font-bold uppercase">
                    <tr>
                      <th className="px-4 py-3 text-left">Timestamp</th>
                      <th className="px-4 py-3 text-left">Acting Administrator</th>
                      <th className="px-4 py-3 text-left">Action</th>
                      <th className="px-4 py-3 text-left">Target Entity</th>
                      <th className="px-4 py-3 text-left">Details</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100 font-mono text-[11px]">
                    {stats.recent_audit_actions.map((ev) => (
                      <tr key={ev.id} className="hover:bg-gray-50 transition">
                        <td className="px-4 py-3 text-gray-500 whitespace-nowrap">
                          {new Date(ev.created_at).toLocaleTimeString("en-NG", {
                            month: "short",
                            day: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                            second: "2-digit",
                          })}
                        </td>
                        <td className="px-4 py-3 text-gray-900 font-sans font-semibold">
                          {ev.actor_name}
                          {ev.actor_role && (
                            <span className="block text-[10px] text-emerald-700 font-mono">
                              [{ev.actor_role}]
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          <span className="px-2 py-0.5 rounded bg-gray-100 text-gray-800 font-bold">
                            {ev.action}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-gray-700">
                          {ev.resource_type}: <span className="text-gray-500">{ev.resource_id.slice(0, 8)}...</span>
                        </td>
                        <td className="px-4 py-3 text-gray-600 font-sans max-w-xs truncate" title={ev.details || ""}>
                          {ev.details || "—"}
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

      {/* 4. TAB CONTENT: Zonal Coordinators (6 Zones) */}
      {activeTab === "zones" && (
        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-4">
            <div>
              <h2 className="text-xl font-bold text-gray-900">
                Geopolitical Zonal Leadership (6 Geopolitical Zones)
              </h2>
              <p className="text-xs text-gray-600">
                Direct oversight of the 6 Nigerian Geopolitical Zones. Only Super Admin may appoint or replace zonal coordinators.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() =>
                  openAppointModal({
                    roleType: "zonal_coordinator",
                    jurisdictionType: "zone",
                    jurisdictionId: allZones[0]?.id || "",
                    jurisdictionName: allZones[0]?.name || "",
                  })
                }
                className="inline-flex items-center px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
              >
                <PlusCircle className="w-4 h-4 mr-1.5" />
                Appoint Zonal Leader
              </button>
              <Link
                href="/admin/leadership"
                className="inline-flex items-center px-3.5 py-2 border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 text-xs font-semibold rounded-lg transition"
              >
                <ShieldCheck className="w-4 h-4 mr-1.5" />
                Full Governance Roster
              </Link>
            </div>
          </div>

          {/* Zone Status Filter Pills */}
          <div className="flex flex-wrap items-center gap-1.5 bg-white p-3 rounded-xl border border-gray-200">
            <span className="text-[11px] font-bold text-gray-500 uppercase mr-2">Filter:</span>
            {[
              { id: "all", label: "All Zones", count: stats.zone_coverage.length },
              {
                id: "active",
                label: "Active",
                count: stats.zone_coverage.filter((z) => z.coordinator_name && z.appointment_status !== "suspended").length,
              },
              {
                id: "vacant",
                label: "Vacant",
                count: stats.zone_coverage.filter((z) => !z.coordinator_name).length,
              },
              {
                id: "suspended",
                label: "Suspended",
                count: stats.zone_coverage.filter((z) => z.appointment_status === "suspended").length,
              },
            ].map((p) => {
              const isSelected = zoneStatusFilter === p.id;
              return (
                <button
                  key={p.id}
                  onClick={() => setZoneStatusFilter(p.id as any)}
                  className={`px-3 py-1 rounded-full text-xs font-semibold transition ${
                    isSelected
                      ? "bg-emerald-800 text-white shadow-sm"
                      : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                  }`}
                >
                  {p.label} ({p.count})
                </button>
              );
            })}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {filteredZones.map((z) => {
              const isAssigned = Boolean(z.coordinator_name);
              const isSuspended = z.appointment_status === "suspended";
              return (
                <div
                  key={z.zone_id}
                  className={`p-6 rounded-2xl border transition shadow-sm space-y-4 ${
                    isSuspended
                      ? "bg-yellow-50/50 border-yellow-300"
                      : isAssigned
                      ? "bg-white border-gray-200"
                      : "bg-amber-50/40 border-amber-300"
                  }`}
                >
                  <div className="flex justify-between items-start">
                    <div>
                      <span className="text-xs font-bold text-emerald-800 uppercase tracking-wider">
                        Zone {z.zone_code}
                      </span>
                      <h3 className="text-lg font-black text-gray-900">{z.zone_name}</h3>
                    </div>
                    <Badge
                      variant={isSuspended ? "warning" : isAssigned ? "success" : "warning"}
                      size="sm"
                    >
                      {isSuspended ? "SUSPENDED" : isAssigned ? "COORDINATOR ACTIVE" : "VACANT"}
                    </Badge>
                  </div>

                  {isAssigned ? (
                    <div className="space-y-3 text-xs border-t border-gray-100 pt-3">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2 text-gray-900 font-semibold">
                          <UserCheck className="w-4 h-4 text-emerald-700" />
                          <span>{z.coordinator_name}</span>
                        </div>
                        {z.coordinator_email && (
                          <div className="text-gray-500 text-[11px] pl-6">{z.coordinator_email}</div>
                        )}
                        {z.coordinator_phone && (
                          <div className="text-gray-500 text-[11px] pl-6">{z.coordinator_phone}</div>
                        )}
                        {z.start_date && (
                          <div className="text-gray-400 text-[10px] pl-6">
                            Appointed: {new Date(z.start_date).toLocaleDateString("en-NG")}
                          </div>
                        )}
                      </div>

                      <div className="flex flex-wrap items-center gap-1.5 pt-2 border-t border-gray-100">
                        {z.appointment_id && (
                          <button
                            type="button"
                            onClick={() =>
                              openHistoryModal({
                                appointmentId: z.appointment_id!,
                                roleTitle: `Zonal Coordinator - ${z.zone_name}`,
                                jurisdictionName: z.zone_name,
                              })
                            }
                            className="px-2 py-1 border border-gray-200 hover:bg-gray-100 text-gray-700 font-semibold rounded-lg text-[11px] transition"
                          >
                            History
                          </button>
                        )}

                        <button
                          type="button"
                          onClick={() =>
                            setReplaceTarget({
                              appointmentId: z.appointment_id || "",
                              currentHolder: z.coordinator_name || "",
                              roleTitle: `Zonal Coordinator - ${z.zone_name}`,
                              jurisdictionName: z.zone_name,
                            })
                          }
                          className="px-2 py-1 bg-amber-50 hover:bg-amber-100 text-amber-800 font-semibold rounded-lg text-[11px] transition"
                        >
                          Replace
                        </button>

                        {isSuspended ? (
                          <button
                            type="button"
                            onClick={() =>
                              setReactivateTarget({
                                appointmentId: z.appointment_id || "",
                                coordinatorName: z.coordinator_name || "",
                                roleTitle: `Zonal Coordinator - ${z.zone_name}`,
                              })
                            }
                            className="px-2 py-1 bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-semibold rounded-lg text-[11px] transition"
                          >
                            Reactivate
                          </button>
                        ) : (
                          <button
                            type="button"
                            onClick={() =>
                              setSuspendTarget({
                                appointmentId: z.appointment_id || "",
                                coordinatorName: z.coordinator_name || "",
                                roleTitle: `Zonal Coordinator - ${z.zone_name}`,
                              })
                            }
                            className="px-2 py-1 bg-yellow-50 hover:bg-yellow-100 text-yellow-800 font-semibold rounded-lg text-[11px] transition"
                          >
                            Suspend
                          </button>
                        )}

                        <button
                          type="button"
                          onClick={() =>
                            setRevokeTarget({
                              appointmentId: z.appointment_id || "",
                              coordinatorName: z.coordinator_name || "",
                              roleTitle: `Zonal Coordinator - ${z.zone_name}`,
                            })
                          }
                          className="px-2 py-1 bg-red-50 hover:bg-red-100 text-red-800 font-semibold rounded-lg text-[11px] transition"
                        >
                          Revoke
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-3 text-xs border-t border-amber-200 pt-3 text-amber-900">
                      <p className="text-[11px] text-amber-800">
                        No active zonal coordinator is assigned to {z.zone_name}.
                      </p>
                      <button
                        type="button"
                        onClick={() =>
                          openAppointModal({
                            roleType: "zonal_coordinator",
                            jurisdictionType: "zone",
                            jurisdictionId: z.zone_id,
                            jurisdictionName: z.zone_name,
                          })
                        }
                        className="inline-flex items-center px-3.5 py-1.5 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-bold text-xs transition"
                      >
                        <PlusCircle className="w-3.5 h-3.5 mr-1" />
                        Appoint Zonal Leader
                      </button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* 5. TAB CONTENT: State Coordinators (36 States + FCT) */}
      {activeTab === "states" && (
        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-4">
            <div>
              <h2 className="text-xl font-bold text-gray-900">
                State & FCT Leadership Oversight ({stats.covered_states} of 37 Assigned)
              </h2>
              <p className="text-xs text-gray-600">
                Complete roster of state coordinators across all 36 States and the Federal Capital Territory.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() =>
                  openAppointModal({
                    roleType: "state_coordinator",
                    jurisdictionType: "state",
                    jurisdictionId: allStates[0]?.id || "",
                    jurisdictionName: allStates[0]?.name || "",
                  })
                }
                className="inline-flex items-center px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
              >
                <PlusCircle className="w-4 h-4 mr-1.5" />
                Appoint State Coordinator
              </button>
              <Link
                href="/admin/leadership"
                className="inline-flex items-center px-3.5 py-2 border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 text-xs font-semibold rounded-lg transition"
              >
                <ShieldCheck className="w-4 h-4 mr-1.5" />
                Full Leadership Portal
              </Link>
            </div>
          </div>

          {/* Search, Zone Filter, & Status Filter Bar */}
          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-3">
            <div className="flex flex-col sm:flex-row gap-3">
              <div className="relative flex-1">
                <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
                <input
                  type="text"
                  placeholder="Search state name, code, or coordinator..."
                  value={stateSearch}
                  onChange={(e) => setStateSearch(e.target.value)}
                  className="w-full pl-9 pr-4 py-2 border border-gray-200 rounded-lg text-xs focus:ring-emerald-500 focus:border-emerald-500"
                />
              </div>
              <div className="flex items-center gap-2">
                <Filter className="w-4 h-4 text-gray-500" />
                <select
                  value={selectedZoneFilter}
                  onChange={(e) => setSelectedZoneFilter(e.target.value)}
                  className="border border-gray-200 rounded-lg text-xs py-2 px-3 focus:ring-emerald-500 focus:border-emerald-500 bg-white"
                >
                  <option value="all">All Geopolitical Zones</option>
                  <option value="NC">North Central (NC)</option>
                  <option value="NE">North East (NE)</option>
                  <option value="NW">North West (NW)</option>
                  <option value="SE">South East (SE)</option>
                  <option value="SS">South South (SS)</option>
                  <option value="SW">South West (SW)</option>
                </select>
              </div>
            </div>

            {/* Status Pills */}
            <div className="flex flex-wrap items-center gap-1.5 pt-2 border-t border-gray-100">
              <span className="text-[11px] font-bold text-gray-500 uppercase mr-2">Status:</span>
              {[
                { id: "all", label: "All States", count: stateCoverage.length },
                {
                  id: "active",
                  label: "Active",
                  count: stateCoverage.filter((st) => st.coordinator_name && st.appointment_status !== "suspended").length,
                },
                {
                  id: "vacant",
                  label: "Vacant",
                  count: stateCoverage.filter((st) => !st.coordinator_name).length,
                },
                {
                  id: "suspended",
                  label: "Suspended",
                  count: stateCoverage.filter((st) => st.appointment_status === "suspended").length,
                },
              ].map((p) => {
                const isSelected = stateStatusFilter === p.id;
                return (
                  <button
                    key={p.id}
                    onClick={() => setStateStatusFilter(p.id as any)}
                    className={`px-3 py-1 rounded-full text-xs font-semibold transition ${
                      isSelected
                        ? "bg-emerald-800 text-white shadow-sm"
                        : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                    }`}
                  >
                    {p.label} ({p.count})
                  </button>
                );
              })}
            </div>
          </div>

          {/* States Table */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 text-xs">
              <thead className="bg-gray-50 text-gray-500 font-bold uppercase">
                <tr>
                  <th className="px-4 py-3 text-left">State / Territory</th>
                  <th className="px-4 py-3 text-left">Zone</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-left">Appointed Coordinator</th>
                  <th className="px-4 py-3 text-left">Contact Info</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {filteredStates.map((st) => {
                  const isAssigned = Boolean(st.coordinator_name);
                  const isSuspended = st.appointment_status === "suspended";
                  return (
                    <tr key={st.state_id} className="hover:bg-gray-50 transition">
                      <td className="px-4 py-3 font-bold text-gray-900">
                        {st.state_name}
                        {st.is_fct && (
                          <span className="ml-1.5 px-1.5 py-0.5 rounded text-[10px] bg-emerald-100 text-emerald-800 font-semibold">
                            FCT
                          </span>
                        )}
                        <div className="text-[10px] text-gray-400 font-normal">Code: {st.state_code}</div>
                      </td>
                      <td className="px-4 py-3 font-semibold text-gray-600">{st.zone_code}</td>
                      <td className="px-4 py-3">
                        <Badge
                          variant={isSuspended ? "warning" : isAssigned ? "success" : "warning"}
                          size="sm"
                        >
                          {isSuspended ? "SUSPENDED" : isAssigned ? "ASSIGNED" : "VACANT"}
                        </Badge>
                      </td>
                      <td className="px-4 py-3 font-medium text-gray-800">
                        {st.coordinator_name || <span className="text-gray-400 italic">No coordinator assigned</span>}
                      </td>
                      <td className="px-4 py-3 text-gray-500">
                        {st.coordinator_email && <div>{st.coordinator_email}</div>}
                        {st.coordinator_phone && <div>{st.coordinator_phone}</div>}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          {isAssigned ? (
                            <>
                              {st.appointment_id && (
                                <button
                                  type="button"
                                  onClick={() =>
                                    openHistoryModal({
                                      appointmentId: st.appointment_id!,
                                      roleTitle: `State Coordinator - ${st.state_name}`,
                                      jurisdictionName: st.state_name,
                                    })
                                  }
                                  className="px-2 py-1 border border-gray-200 hover:bg-gray-100 text-gray-700 font-semibold rounded-lg text-[11px] transition"
                                >
                                  History
                                </button>
                              )}

                              <button
                                type="button"
                                onClick={() =>
                                  setReplaceTarget({
                                    appointmentId: st.appointment_id || "",
                                    currentHolder: st.coordinator_name || "",
                                    roleTitle: `State Coordinator - ${st.state_name}`,
                                    jurisdictionName: st.state_name,
                                  })
                                }
                                className="px-2 py-1 bg-amber-50 hover:bg-amber-100 text-amber-800 font-semibold rounded-lg text-[11px] transition"
                              >
                                Replace
                              </button>

                              {isSuspended ? (
                                <button
                                  type="button"
                                  onClick={() =>
                                    setReactivateTarget({
                                      appointmentId: st.appointment_id || "",
                                      coordinatorName: st.coordinator_name || "",
                                      roleTitle: `State Coordinator - ${st.state_name}`,
                                    })
                                  }
                                  className="px-2 py-1 bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-semibold rounded-lg text-[11px] transition"
                                >
                                  Reactivate
                                </button>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() =>
                                    setSuspendTarget({
                                      appointmentId: st.appointment_id || "",
                                      coordinatorName: st.coordinator_name || "",
                                      roleTitle: `State Coordinator - ${st.state_name}`,
                                    })
                                  }
                                  className="px-2 py-1 bg-yellow-50 hover:bg-yellow-100 text-yellow-800 font-semibold rounded-lg text-[11px] transition"
                                >
                                  Suspend
                                </button>
                              )}

                              <button
                                type="button"
                                onClick={() =>
                                  setRevokeTarget({
                                    appointmentId: st.appointment_id || "",
                                    coordinatorName: st.coordinator_name || "",
                                    roleTitle: `State Coordinator - ${st.state_name}`,
                                  })
                                }
                                className="px-2 py-1 bg-red-50 hover:bg-red-100 text-red-800 font-semibold rounded-lg text-[11px] transition"
                              >
                                Revoke
                              </button>
                            </>
                          ) : (
                            <button
                              type="button"
                              onClick={() =>
                                openAppointModal({
                                  roleType: "state_coordinator",
                                  jurisdictionType: "state",
                                  jurisdictionId: st.state_id,
                                  jurisdictionName: st.state_name,
                                })
                              }
                              className="px-3 py-1 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg text-xs transition"
                            >
                              + Appoint
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* 6. TAB CONTENT: LGA Coordinators (774 LGAs) */}
      {activeTab === "lgas" && (
        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-4">
            <div>
              <h2 className="text-xl font-bold text-gray-900">
                Local Government Area Leadership ({stats.covered_lgas} of 774 Represented)
              </h2>
              <p className="text-xs text-gray-600">
                Grassroots coordinator and representative coverage across Nigeria's 774 Local Government Areas.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() =>
                  openAppointModal({
                    roleType: "lga_representative",
                    jurisdictionType: "lga",
                    jurisdictionId: "",
                    jurisdictionName: "",
                  })
                }
                className="inline-flex items-center px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
              >
                <PlusCircle className="w-4 h-4 mr-1.5" />
                Appoint LGA Representative
              </button>
              <Link
                href="/admin/leadership"
                className="inline-flex items-center px-3.5 py-2 border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 text-xs font-semibold rounded-lg transition"
              >
                <ShieldCheck className="w-4 h-4 mr-1.5" />
                Manage LGA Appointments
              </Link>
            </div>
          </div>

          {/* State Selector & LGA Search */}
          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm space-y-3">
            <div className="flex flex-col sm:flex-row gap-4">
              <div className="w-full sm:w-72">
                <label className="block text-[11px] font-bold text-gray-700 uppercase mb-1">
                  Select State to Inspect LGAs:
                </label>
                <select
                  value={selectedStateForLga}
                  onChange={(e) => setSelectedStateForLga(e.target.value)}
                  className="w-full border border-gray-200 rounded-lg text-xs py-2 px-3 focus:ring-emerald-500 focus:border-emerald-500 bg-white"
                >
                  {stateCoverage.map((st) => (
                    <option key={st.state_id} value={st.state_id}>
                      {st.state_name} ({st.zone_code})
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex-1">
                <label className="block text-[11px] font-bold text-gray-700 uppercase mb-1">
                  Filter LGA by Name / Coordinator:
                </label>
                <div className="relative">
                  <Search className="w-4 h-4 text-gray-400 absolute left-3 top-2.5" />
                  <input
                    type="text"
                    placeholder="Filter local councils..."
                    value={lgaSearch}
                    onChange={(e) => setLgaSearch(e.target.value)}
                    className="w-full pl-9 pr-4 py-2 border border-gray-200 rounded-lg text-xs focus:ring-emerald-500 focus:border-emerald-500"
                  />
                </div>
              </div>
            </div>

            {/* Status Pills */}
            <div className="flex flex-wrap items-center gap-1.5 pt-2 border-t border-gray-100">
              <span className="text-[11px] font-bold text-gray-500 uppercase mr-2">Status:</span>
              {[
                { id: "all", label: "All Councils", count: lgaCoverage.length },
                {
                  id: "active",
                  label: "Active",
                  count: lgaCoverage.filter((lg) => lg.coordinator_name && lg.appointment_status !== "suspended").length,
                },
                {
                  id: "vacant",
                  label: "Vacant",
                  count: lgaCoverage.filter((lg) => !lg.coordinator_name).length,
                },
                {
                  id: "suspended",
                  label: "Suspended",
                  count: lgaCoverage.filter((lg) => lg.appointment_status === "suspended").length,
                },
              ].map((p) => {
                const isSelected = lgaStatusFilter === p.id;
                return (
                  <button
                    key={p.id}
                    onClick={() => setLgaStatusFilter(p.id as any)}
                    className={`px-3 py-1 rounded-full text-xs font-semibold transition ${
                      isSelected
                        ? "bg-emerald-800 text-white shadow-sm"
                        : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                    }`}
                  >
                    {p.label} ({p.count})
                  </button>
                );
              })}
            </div>
          </div>

          {/* LGA Table */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
            {loadingLgas ? (
              <div className="p-8">
                <Spinner text="Loading local government councils..." />
              </div>
            ) : filteredLgas.length === 0 ? (
              <div className="p-8 text-center text-xs text-gray-500">
                No local government councils match your search query.
              </div>
            ) : (
              <table className="min-w-full divide-y divide-gray-200 text-xs">
                <thead className="bg-gray-50 text-gray-500 font-bold uppercase">
                  <tr>
                    <th className="px-4 py-3 text-left">Local Council Name</th>
                    <th className="px-4 py-3 text-left">Type</th>
                    <th className="px-4 py-3 text-left">Status</th>
                    <th className="px-4 py-3 text-left">Coordinator / Representative</th>
                    <th className="px-4 py-3 text-left">Contact Info</th>
                    <th className="px-4 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredLgas.map((lg) => {
                    const isAssigned = Boolean(lg.coordinator_name);
                    const isSuspended = lg.appointment_status === "suspended";
                    return (
                      <tr key={lg.lga_id} className="hover:bg-gray-50 transition">
                        <td className="px-4 py-3 font-bold text-gray-900">{lg.lga_name}</td>
                        <td className="px-4 py-3 text-gray-500">
                          {lg.is_area_council ? "Area Council" : "LGA"}
                        </td>
                        <td className="px-4 py-3">
                          <Badge
                            variant={isSuspended ? "warning" : isAssigned ? "success" : "warning"}
                            size="sm"
                          >
                            {isSuspended ? "SUSPENDED" : isAssigned ? "ASSIGNED" : "VACANT"}
                          </Badge>
                        </td>
                        <td className="px-4 py-3 font-medium text-gray-800">
                          {lg.coordinator_name || (
                            <span className="text-gray-400 italic">No representative</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-gray-500">
                          {lg.coordinator_email && <div>{lg.coordinator_email}</div>}
                          {lg.coordinator_phone && <div>{lg.coordinator_phone}</div>}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-1.5">
                            {isAssigned ? (
                              <>
                                {lg.appointment_id && (
                                  <button
                                    type="button"
                                    onClick={() =>
                                      openHistoryModal({
                                        appointmentId: lg.appointment_id!,
                                        roleTitle: `LGA Representative - ${lg.lga_name}`,
                                        jurisdictionName: lg.lga_name,
                                      })
                                    }
                                    className="px-2 py-1 border border-gray-200 hover:bg-gray-100 text-gray-700 font-semibold rounded-lg text-[11px] transition"
                                  >
                                    History
                                  </button>
                                )}

                                <button
                                  type="button"
                                  onClick={() =>
                                    setReplaceTarget({
                                      appointmentId: lg.appointment_id || "",
                                      currentHolder: lg.coordinator_name || "",
                                      roleTitle: `LGA Representative - ${lg.lga_name}`,
                                      jurisdictionName: lg.lga_name,
                                    })
                                  }
                                  className="px-2 py-1 bg-amber-50 hover:bg-amber-100 text-amber-800 font-semibold rounded-lg text-[11px] transition"
                                >
                                  Replace
                                </button>

                                {isSuspended ? (
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setReactivateTarget({
                                        appointmentId: lg.appointment_id || "",
                                        coordinatorName: lg.coordinator_name || "",
                                        roleTitle: `LGA Representative - ${lg.lga_name}`,
                                      })
                                    }
                                    className="px-2 py-1 bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-semibold rounded-lg text-[11px] transition"
                                  >
                                    Reactivate
                                  </button>
                                ) : (
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setSuspendTarget({
                                        appointmentId: lg.appointment_id || "",
                                        coordinatorName: lg.coordinator_name || "",
                                        roleTitle: `LGA Representative - ${lg.lga_name}`,
                                      })
                                    }
                                    className="px-2 py-1 bg-yellow-50 hover:bg-yellow-100 text-yellow-800 font-semibold rounded-lg text-[11px] transition"
                                  >
                                    Suspend
                                  </button>
                                )}

                                <button
                                  type="button"
                                  onClick={() =>
                                    setRevokeTarget({
                                      appointmentId: lg.appointment_id || "",
                                      coordinatorName: lg.coordinator_name || "",
                                      roleTitle: `LGA Representative - ${lg.lga_name}`,
                                    })
                                  }
                                  className="px-2 py-1 bg-red-50 hover:bg-red-100 text-red-800 font-semibold rounded-lg text-[11px] transition"
                                >
                                  Revoke
                                </button>
                              </>
                            ) : (
                              <button
                                type="button"
                                onClick={() =>
                                  openAppointModal({
                                    roleType: "lga_representative",
                                    jurisdictionType: "lga",
                                    jurisdictionId: lg.lga_id,
                                    jurisdictionName: lg.lga_name,
                                  })
                                }
                                className="px-3 py-1 bg-emerald-800 hover:bg-emerald-900 text-white font-bold rounded-lg text-xs transition"
                              >
                                + Appoint
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {/* 7. TAB CONTENT: Member Management */}
      {activeTab === "members" && (
        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-4">
            <div>
              <h2 className="text-xl font-bold text-gray-900">Member Management Navigation</h2>
              <p className="text-xs text-gray-600">
                Nationwide member verification, identity safeguarding, and geographic registration distribution.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Link
                href="/admin/reviews"
                className="inline-flex items-center px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
              >
                <UserCheck className="w-4 h-4 mr-1.5" />
                Review Claims Queue ({stats.unverified_participations_queue})
              </Link>
              <Link
                href="/admin/audit"
                className="inline-flex items-center px-3.5 py-2 bg-gray-100 hover:bg-gray-200 text-gray-800 text-xs font-semibold rounded-lg transition"
              >
                <History className="w-3.5 h-3.5 mr-1.5" />
                Audit Events
              </Link>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-gray-500 font-bold uppercase">Total Registered</span>
              <div className="text-4xl font-black text-gray-900">{stats.total_members.toLocaleString()}</div>
              <p className="text-xs text-gray-600">All member accounts created across Nigerian States and LGAs.</p>
              <div className="text-[11px] text-gray-500 pt-2 border-t border-gray-100">
                Total event registrations: <span className="font-bold text-gray-900">{stats.total_registrations.toLocaleString()}</span>
              </div>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-emerald-600 font-bold uppercase">Fully Verified</span>
              <div className="text-4xl font-black text-emerald-700">{stats.verified_members.toLocaleString()}</div>
              <p className="text-xs text-gray-600">Verified via contact OTP or secure email confirmation.</p>
              <div className="w-full bg-gray-100 rounded-full h-2 mt-2">
                <div
                  className="bg-emerald-600 h-2 rounded-full transition-all"
                  style={{
                    width: `${stats.total_members > 0 ? Math.min(100, Math.round((stats.verified_members / stats.total_members) * 100)) : 0}%`,
                  }}
                />
              </div>
              <span className="text-[11px] text-emerald-800 font-semibold block">
                {stats.total_members > 0 ? Math.round((stats.verified_members / stats.total_members) * 100) : 0}% National Verification Rate
              </span>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-amber-600 font-bold uppercase">Pending Verification</span>
              <div className="text-4xl font-black text-amber-600">
                {stats.pending_member_verifications.toLocaleString()}
              </div>
              <p className="text-xs text-gray-600">Members awaiting phone or email token confirmation.</p>
              <div className="text-[11px] text-amber-700 font-medium pt-2 border-t border-amber-100">
                {stats.total_members > 0 ? Math.round((stats.pending_member_verifications / stats.total_members) * 100) : 0}% of accounts pending
              </div>
            </div>
          </div>

          {/* Member Governance & Operational Navigation Hub */}
          <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-4 shadow-sm">
            <h3 className="text-sm font-bold text-gray-900 uppercase tracking-wider flex items-center gap-2">
              <Users className="w-4 h-4 text-emerald-700" />
              Member Governance & Safeguarding Navigation Hub
            </h3>
            <p className="text-xs text-gray-600">
              Access nationwide administrative queues, appointed regional leadership, confidential safety investigations, and outbound member communications.
            </p>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 pt-2">
              <Link
                href="/admin/reviews"
                className="p-4 rounded-xl border border-gray-200 hover:border-emerald-600 hover:shadow-sm transition group bg-gray-50/50 hover:bg-white"
              >
                <div className="w-9 h-9 rounded-lg bg-emerald-100 text-emerald-800 flex items-center justify-center mb-3 group-hover:bg-emerald-800 group-hover:text-white transition">
                  <UserCheck className="w-5 h-5" />
                </div>
                <div className="text-xs font-bold text-gray-900">Verification & Appeals</div>
                <div className="text-[11px] text-gray-500 mt-1">
                  Queue: {stats.unverified_participations_queue} claims, {stats.pending_appeals_queue} appeals
                </div>
                <div className="mt-3 text-[11px] font-bold text-emerald-700 group-hover:text-emerald-900 flex items-center">
                  Open Queue <ArrowRight className="w-3 h-3 ml-1" />
                </div>
              </Link>

              <Link
                href="/admin/leadership"
                className="p-4 rounded-xl border border-gray-200 hover:border-emerald-600 hover:shadow-sm transition group bg-gray-50/50 hover:bg-white"
              >
                <div className="w-9 h-9 rounded-lg bg-emerald-100 text-emerald-800 flex items-center justify-center mb-3 group-hover:bg-emerald-800 group-hover:text-white transition">
                  <ShieldCheck className="w-5 h-5" />
                </div>
                <div className="text-xs font-bold text-gray-900">Leadership Roster</div>
                <div className="text-[11px] text-gray-500 mt-1">
                  {stats.active_coordinators} active coordinators across Nigeria
                </div>
                <div className="mt-3 text-[11px] font-bold text-emerald-700 group-hover:text-emerald-900 flex items-center">
                  Appoint Coordinators <ArrowRight className="w-3 h-3 ml-1" />
                </div>
              </Link>

              <Link
                href="/admin/safeguarding"
                className="p-4 rounded-xl border border-gray-200 hover:border-emerald-600 hover:shadow-sm transition group bg-gray-50/50 hover:bg-white"
              >
                <div className="w-9 h-9 rounded-lg bg-emerald-100 text-emerald-800 flex items-center justify-center mb-3 group-hover:bg-emerald-800 group-hover:text-white transition">
                  <ShieldAlert className="w-5 h-5 text-amber-700" />
                </div>
                <div className="text-xs font-bold text-gray-900">Safeguarding Cases</div>
                <div className="text-[11px] text-gray-500 mt-1">
                  Confidential reports, legal holds, and officer triage
                </div>
                <div className="mt-3 text-[11px] font-bold text-emerald-700 group-hover:text-emerald-900 flex items-center">
                  Manage Cases <ArrowRight className="w-3 h-3 ml-1" />
                </div>
              </Link>

              <Link
                href="/admin/communications"
                className="p-4 rounded-xl border border-gray-200 hover:border-emerald-600 hover:shadow-sm transition group bg-gray-50/50 hover:bg-white"
              >
                <div className="w-9 h-9 rounded-lg bg-emerald-100 text-emerald-800 flex items-center justify-center mb-3 group-hover:bg-emerald-800 group-hover:text-white transition">
                  <Send className="w-5 h-5" />
                </div>
                <div className="text-xs font-bold text-gray-900">Member Outbox</div>
                <div className="text-[11px] text-gray-500 mt-1">
                  Broadcasts, OTP delivery logs, and Termii/Brevo health
                </div>
                <div className="mt-3 text-[11px] font-bold text-emerald-700 group-hover:text-emerald-900 flex items-center">
                  View Outbox <ArrowRight className="w-3 h-3 ml-1" />
                </div>
              </Link>
            </div>
          </div>
        </div>
      )}

      {/* 8. TAB CONTENT: Activities & Cleanups */}
      {activeTab === "activities" && (
        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-4">
            <div>
              <h2 className="text-xl font-bold text-gray-900">Activities & Cleanups Oversight</h2>
              <p className="text-xs text-gray-600">
                Direct access to environmental cleanup campaigns, venue proposals, and volunteer participation reviews.
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Link
                href="/admin/activities"
                className="inline-flex items-center px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
              >
                Open Venue Manager <ArrowRight className="w-3.5 h-3.5 ml-1.5" />
              </Link>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-gray-500 font-bold uppercase">Upcoming Cleanups</span>
              <div className="text-3xl font-black text-gray-900">{stats.upcoming_cleanups_count}</div>
              <p className="text-xs text-gray-600">Scheduled activities open for public volunteer sign-up.</p>
              <Link href="/admin/activities" className="text-xs font-bold text-emerald-700 hover:text-emerald-900">
                View Venues &rarr;
              </Link>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-gray-500 font-bold uppercase">Draft Venues In Review</span>
              <div className="text-3xl font-black text-gray-900">{stats.draft_venues_review_queue}</div>
              <p className="text-xs text-gray-600">Local venue proposals awaiting coordinator publication.</p>
              <Link href="/admin/activities" className="text-xs font-bold text-emerald-700 hover:text-emerald-900">
                Inspect Drafts &rarr;
              </Link>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <span className="text-xs text-gray-500 font-bold uppercase">Participation Review Queue</span>
              <div className="text-3xl font-black text-gray-900">{stats.unverified_participations_queue}</div>
              <p className="text-xs text-gray-600">Volunteer attendance claims pending verification.</p>
              <Link href="/admin/reviews" className="text-xs font-bold text-emerald-700 hover:text-emerald-900">
                Review Claims &rarr;
              </Link>
            </div>
          </div>

          {/* Published Upcoming Venues Detail Table */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
            <div className="flex justify-between items-center">
              <div>
                <h3 className="text-base font-bold text-gray-900 flex items-center gap-2">
                  <Calendar className="w-5 h-5 text-emerald-700" />
                  Scheduled Environmental Cleanups Roster
                </h3>
                <p className="text-xs text-gray-500">
                  Published community venues with scheduled dates, volunteer capacities, and designated coordinators.
                </p>
              </div>
              <Link
                href="/admin/activities"
                className="text-xs font-bold text-emerald-700 hover:text-emerald-900 inline-flex items-center"
              >
                Create New Venue <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>

            {stats.upcoming_activities.length === 0 ? (
              <div className="text-center py-8 text-xs text-gray-500 bg-gray-50 rounded-xl">
                No upcoming cleanups scheduled. Coordinators can propose and publish new venues in the Venue Manager.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200 text-xs">
                  <thead className="bg-gray-50 text-gray-500 font-bold uppercase">
                    <tr>
                      <th className="px-4 py-3 text-left">Activity / Venue</th>
                      <th className="px-4 py-3 text-left">Jurisdiction</th>
                      <th className="px-4 py-3 text-left">Scheduled Date</th>
                      <th className="px-4 py-3 text-left">Assigned Coordinator</th>
                      <th className="px-4 py-3 text-right">Volunteers</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {stats.upcoming_activities.map((act) => (
                      <tr key={act.id} className="hover:bg-gray-50 transition">
                        <td className="px-4 py-3 font-semibold text-gray-900">
                          {act.title}
                          <div className="text-[10px] text-gray-500 font-normal">{act.address}</div>
                        </td>
                        <td className="px-4 py-3 text-gray-700">
                          {act.state_name} — <span className="text-gray-500">{act.lga_name}</span>
                        </td>
                        <td className="px-4 py-3 text-gray-600">
                          {new Date(act.start_time).toLocaleDateString("en-NG", {
                            weekday: "short",
                            year: "numeric",
                            month: "short",
                            day: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </td>
                        <td className="px-4 py-3 text-gray-700">{act.coordinator_name}</td>
                        <td className="px-4 py-3 text-right font-bold text-emerald-700">
                          {act.registered_count} / {act.capacity}
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

      {/* 9. TAB CONTENT: Marketplace & Inventory */}
      {activeTab === "marketplace" && (
        <div className="space-y-6">
          <div className="flex justify-between items-center">
            <div>
              <h2 className="text-xl font-bold text-gray-900">Marketplace & Inventory Oversight</h2>
              <p className="text-xs text-gray-600">
                Multi-vendor enterprise governance, movement sanitation supplies, warehouse stock, and dispute arbitration.
              </p>
            </div>
            <div className="flex gap-2">
              <Link
                href="/admin/marketplace"
                className="px-3.5 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
              >
                Marketplace Workspace
              </Link>
              <Link
                href="/admin/inventory"
                className="px-3.5 py-2 bg-emerald-100 hover:bg-emerald-200 text-emerald-900 text-xs font-bold rounded-lg transition"
              >
                Inventory Logistics
              </Link>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <div className="flex items-center gap-2">
                <Store className="w-5 h-5 text-emerald-700" />
                <h3 className="font-bold text-gray-900 text-sm">Ratels Multi-Vendor Marketplace</h3>
              </div>
              <p className="text-xs text-gray-600 leading-relaxed">
                Vendor business onboarding, product approvals, order fulfillment tracking, multi-vendor shipments, and customer dispute resolution.
              </p>
              <Link href="/admin/marketplace" className="inline-flex items-center text-xs font-bold text-emerald-700 hover:text-emerald-900">
                Open Marketplace Oversight <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <div className="flex items-center gap-2">
                <Boxes className="w-5 h-5 text-emerald-700" />
                <h3 className="font-bold text-gray-900 text-sm">Inventory & Materials Distribution</h3>
              </div>
              <p className="text-xs text-gray-600 leading-relaxed">
                Central movement supplies, warehouse stock monitoring, LGA branch allocations, and delivery transit verification.
              </p>
              <Link href="/admin/inventory" className="inline-flex items-center text-xs font-bold text-emerald-700 hover:text-emerald-900">
                Open Inventory Logistics <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>
          </div>
        </div>
      )}

      {/* 10. TAB CONTENT: Financial Reporting */}
      {activeTab === "finance" && (
        <div className="space-y-6">
          <div className="flex justify-between items-center">
            <div>
              <h2 className="text-xl font-bold text-gray-900">Financial Reporting & Accountability</h2>
              <p className="text-xs text-gray-600">
                Official double-entry general ledger, restricted financial statements, budget approvals, and audit trail.
              </p>
            </div>
            <Link
              href="/admin/finance"
              className="inline-flex items-center px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
            >
              <DollarSign className="w-4 h-4 mr-1.5" />
              General Ledger Workspace
            </Link>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <DollarSign className="w-5 h-5 text-emerald-700" />
                Super Admin Financial Authorizations
              </h3>
              <p className="text-xs text-gray-600 leading-relaxed">
                Only the Super Admin may authorize staff disbursements, approve expenditure commitments, activate bank accounts, and certify reconciliation periods.
              </p>
              <Link href="/admin/finance" className="inline-flex items-center text-xs font-bold text-emerald-700 hover:text-emerald-900">
                Review Financial Approvals <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>

            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-3">
              <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                <BarChart3 className="w-5 h-5 text-emerald-700" />
                Public Transparency Disclosures
              </h3>
              <p className="text-xs text-gray-600 leading-relaxed">
                Certified public disclosure reports generated from immutable general ledger balances across Nigeria.
              </p>
              <Link href="/admin/reports" className="inline-flex items-center text-xs font-bold text-emerald-700 hover:text-emerald-900">
                Public Reports Subsystem <ArrowRight className="w-3.5 h-3.5 ml-1" />
              </Link>
            </div>
          </div>
        </div>
      )}

      {/* 11. TAB CONTENT: Audit Logs */}
      {activeTab === "audit" && (
        <div className="space-y-6">
          <div className="flex justify-between items-center">
            <div>
              <h2 className="text-xl font-bold text-gray-900">Privileged System Audit Trail</h2>
              <p className="text-xs text-gray-600">
                Cryptographically recorded, append-only audit stream of administrative approvals and security events.
              </p>
            </div>
            <Link
              href="/admin/audit"
              className="inline-flex items-center px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition"
            >
              Full Interactive Audit Log <ArrowRight className="w-3.5 h-3.5 ml-1.5" />
            </Link>
          </div>

          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200 text-xs">
              <thead className="bg-gray-50 text-gray-500 font-bold uppercase">
                <tr>
                  <th className="px-4 py-3 text-left">Timestamp</th>
                  <th className="px-4 py-3 text-left">Acting Administrator</th>
                  <th className="px-4 py-3 text-left">Action</th>
                  <th className="px-4 py-3 text-left">Target Entity</th>
                  <th className="px-4 py-3 text-left">Scope / Jurisdiction</th>
                  <th className="px-4 py-3 text-left">Details</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 font-mono text-[11px]">
                {stats.recent_audit_actions.map((ev) => (
                  <tr key={ev.id} className="hover:bg-gray-50 transition">
                    <td className="px-4 py-3 text-gray-500 whitespace-nowrap font-mono">
                      {new Date(ev.created_at).toLocaleString("en-NG")}
                    </td>
                    <td className="px-4 py-3 font-sans font-semibold text-gray-900">
                      {ev.actor_name}
                      {ev.actor_role && (
                        <span className="block text-[10px] text-emerald-700 font-mono">
                          [{ev.actor_role}]
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <span className="px-2 py-0.5 rounded bg-gray-100 text-gray-800 font-bold">
                        {ev.action}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-700 font-mono">
                      {ev.resource_type}: {ev.resource_id.slice(0, 8)}...
                    </td>
                    <td className="px-4 py-3 text-gray-600 font-sans">
                      {ev.scope_type ? `${ev.scope_type} (${ev.scope_id || "national"})` : "National"}
                    </td>
                    <td className="px-4 py-3 text-gray-600 font-sans max-w-sm truncate" title={ev.details || ""}>
                      {ev.details || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* 12. TAB CONTENT: System Administration */}
      {activeTab === "system" && (
        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-4">
            <div>
              <h2 className="text-xl font-bold text-gray-900">System Administration & Infrastructure Controls</h2>
              <p className="text-xs text-gray-600">
                Authoritative geography verification, organization-wide policy settings, and database infrastructure status.
              </p>
            </div>
            <Badge variant="warning" size="sm">
              SUPER ADMIN PRIVILEGED
            </Badge>
          </div>

          {settingSuccess && <Alert type="success">{settingSuccess}</Alert>}

          {/* Organization-Wide Governance Settings Panel */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-6 space-y-4">
            <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-2">
              <div>
                <h3 className="text-base font-bold text-gray-900 flex items-center gap-2">
                  <Settings className="w-5 h-5 text-emerald-700" />
                  Organization-Wide Governance & Operational Settings
                </h3>
                <p className="text-xs text-gray-500">
                  National policy configurations strictly editable by Super Admin. Changes are immutably logged to the privileged audit trail.
                </p>
              </div>
            </div>

            {settings.length === 0 ? (
              <div className="text-center py-8 text-xs text-gray-500 bg-gray-50 rounded-xl">
                No organization-wide settings loaded or none configured in database.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200 text-xs">
                  <thead className="bg-gray-50 text-gray-500 font-bold uppercase">
                    <tr>
                      <th className="px-4 py-3 text-left">Category</th>
                      <th className="px-4 py-3 text-left">Setting Key & Description</th>
                      <th className="px-4 py-3 text-left">Current Value</th>
                      <th className="px-4 py-3 text-left">Last Modified</th>
                      <th className="px-4 py-3 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {settings.map((setting) => {
                      const isEditing = editingKey === setting.key;
                      return (
                        <tr key={setting.key} className="hover:bg-gray-50 transition">
                          <td className="px-4 py-3 whitespace-nowrap">
                            <span className="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-gray-100 text-gray-700">
                              {setting.category}
                            </span>
                          </td>
                          <td className="px-4 py-3">
                            <div className="font-mono font-bold text-gray-900">{setting.key}</div>
                            {setting.description && (
                              <div className="text-[11px] text-gray-500 mt-0.5 max-w-sm">{setting.description}</div>
                            )}
                          </td>
                          <td className="px-4 py-3 font-mono">
                            {isEditing ? (
                              <div className="flex items-center gap-2">
                                {setting.value === "true" || setting.value === "false" ? (
                                  <select
                                    value={editingValue}
                                    onChange={(e) => setEditingValue(e.target.value)}
                                    className="border border-emerald-500 rounded px-2 py-1 text-xs bg-white focus:outline-none"
                                  >
                                    <option value="true">true (Enabled)</option>
                                    <option value="false">false (Disabled)</option>
                                  </select>
                                ) : (
                                  <input
                                    type="text"
                                    value={editingValue}
                                    onChange={(e) => setEditingValue(e.target.value)}
                                    className="border border-emerald-500 rounded px-2 py-1 text-xs focus:outline-none w-56"
                                  />
                                )}
                              </div>
                            ) : (
                              <span
                                className={`px-2 py-0.5 rounded text-[11px] font-bold ${
                                  setting.value === "true"
                                    ? "bg-emerald-100 text-emerald-800"
                                    : setting.value === "false"
                                    ? "bg-amber-100 text-amber-800"
                                    : "bg-gray-100 text-gray-900"
                                }`}
                              >
                                {setting.is_sensitive ? "••••••••" : setting.value || "—"}
                              </span>
                            )}
                          </td>
                          <td className="px-4 py-3 text-gray-500 text-[11px] whitespace-nowrap">
                            {setting.updated_at
                              ? new Date(setting.updated_at).toLocaleDateString("en-NG", {
                                  month: "short",
                                  day: "numeric",
                                  year: "numeric",
                                  hour: "2-digit",
                                  minute: "2-digit",
                                })
                              : "Initial Seeding"}
                          </td>
                          <td className="px-4 py-3 text-right whitespace-nowrap">
                            {isEditing ? (
                              <div className="inline-flex items-center gap-1.5">
                                <button
                                  onClick={() => handleSaveSetting(setting.key, editingValue)}
                                  disabled={savingSetting}
                                  className="px-2.5 py-1 bg-emerald-800 hover:bg-emerald-700 text-white font-bold text-xs rounded transition disabled:opacity-50"
                                >
                                  {savingSetting ? "Saving..." : "Save"}
                                </button>
                                <button
                                  onClick={() => setEditingKey(null)}
                                  disabled={savingSetting}
                                  className="px-2 py-1 bg-gray-200 hover:bg-gray-300 text-gray-700 font-bold text-xs rounded transition"
                                >
                                  Cancel
                                </button>
                              </div>
                            ) : (
                              <button
                                onClick={() => {
                                  setEditingKey(setting.key);
                                  setEditingValue(setting.value);
                                }}
                                className="px-3 py-1 bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-bold text-xs rounded transition"
                              >
                                Edit
                              </button>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Geography Seeder Card */}
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-4">
              <div className="flex items-center gap-2">
                <Database className="w-5 h-5 text-emerald-700" />
                <h3 className="font-bold text-gray-900 text-sm">Authoritative Reference Geography</h3>
              </div>
              <p className="text-xs text-gray-600 leading-relaxed">
                Nigerian geopolitical reference dataset: 6 Geopolitical Zones, 36 States + FCT (37 total), and 774 Local Government Areas / Area Councils.
              </p>
              <div className="flex flex-wrap gap-2 text-xs">
                <Badge variant="neutral">{stats.total_zones} Zones</Badge>
                <Badge variant="neutral">{stats.total_states} States/FCT</Badge>
                <Badge variant="neutral">{stats.total_lgas} LGAs</Badge>
              </div>
              <div className="pt-2">
                <button
                  onClick={handleSeedGeography}
                  disabled={seedingGeography}
                  className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-bold rounded-lg transition disabled:opacity-50 inline-flex items-center gap-2"
                >
                  {seedingGeography ? (
                    <>
                      <Spinner size="sm" /> Seeding Geography...
                    </>
                  ) : (
                    <>
                      <CheckCircle2 className="w-4 h-4" /> Verify & Seed Reference Geography
                    </>
                  )}
                </button>
              </div>
            </div>

            {/* Operational Kill-Switch & Circuit Breakers */}
            <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm space-y-4">
              <div className="flex items-center gap-2">
                <ShieldAlert className="w-5 h-5 text-amber-700" />
                <h3 className="font-bold text-gray-900 text-sm">Operational Safety & Circuit Breakers</h3>
              </div>
              <p className="text-xs text-gray-600 leading-relaxed">
                Emergency operational controls configured on production services. In the event of upstream failure, payments can be disabled via the environment kill-switch.
              </p>
              <div className="space-y-2 text-xs">
                <div className="flex justify-between items-center py-1 border-b border-gray-100">
                  <span className="text-gray-600">Database Engine</span>
                  <span className="font-semibold text-gray-900">Neon Serverless PostgreSQL 16</span>
                </div>
                <div className="flex justify-between items-center py-1 border-b border-gray-100">
                  <span className="text-gray-600">Advisory Migration Lock</span>
                  <span className="text-emerald-700 font-bold flex items-center gap-1">
                    <Check className="w-3.5 h-3.5" /> Active (Distributed)
                  </span>
                </div>
                <div className="flex justify-between items-center py-1 border-b border-gray-100">
                  <span className="text-gray-600">Payment Circuit-Breaker</span>
                  <span className="text-emerald-700 font-bold flex items-center gap-1">
                    <Check className="w-3.5 h-3.5" /> Safe Disabled Mode Supported
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* =========================================================================
          MODAL 1: Appointment Workflow (Super Admin)
         ========================================================================= */}
      {showAppointModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm overflow-y-auto">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-5 shadow-2xl border border-gray-200 my-8">
            <div className="flex justify-between items-start border-b pb-3">
              <div>
                <span className="text-[10px] font-black uppercase tracking-wider text-emerald-700 block">
                  Super Admin Governance
                </span>
                <h2 className="text-lg font-black text-gray-900 mt-0.5">
                  {appointStep === "form" ? "Assign Leadership Position" : "Review & Confirm Appointment"}
                </h2>
              </div>
              <button
                onClick={() => {
                  setShowAppointModal(false);
                  setAssignStep("form");
                }}
                className="text-gray-400 hover:text-gray-600"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {appointStep === "form" ? (
              <div className="space-y-4 text-xs">
                {/* 1. Appointee Member Search & Select */}
                <div>
                  <label className="block font-bold text-gray-700 mb-1">
                    Search Registered Member *
                  </label>
                  {appointSelectedUser ? (
                    <div className="flex items-center justify-between p-3 rounded-xl bg-emerald-50 border border-emerald-300">
                      <div>
                        <div className="font-bold text-emerald-950 text-sm">
                          {appointSelectedUser.display_name}
                        </div>
                        <div className="text-gray-600 font-mono text-[11px]">
                          Ref: {appointSelectedUser.membership_reference} • Role: {appointSelectedUser.primary_role}
                        </div>
                        {appointSelectedUser.email && (
                          <div className="text-gray-500 text-[10px]">{appointSelectedUser.email}</div>
                        )}
                      </div>
                      <button
                        type="button"
                        onClick={() => setAppointSelectedUser(null)}
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
                          placeholder="Search members by name, email, or reference..."
                          value={appointMemberSearch}
                          onChange={(e) => setAppointMemberSearch(e.target.value)}
                          className="w-full pl-9 pr-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                        />
                      </div>
                      {appointSearching && (
                        <div className="text-[11px] text-gray-500 italic">Searching members...</div>
                      )}
                      {appointMemberResults.length > 0 && (
                        <div className="border border-gray-200 rounded-xl max-h-44 overflow-y-auto divide-y bg-white shadow-sm">
                          {appointMemberResults.map((m) => (
                            <button
                              key={m.id}
                              type="button"
                              onClick={() => {
                                setAppointSelectedUser(m);
                                setAppointMemberResults([]);
                                setAppointMemberSearch("");
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
                      value={appointRoleType}
                      onChange={(e) => {
                        const r = e.target.value;
                        setAppointRoleType(r);
                        if (r === "national_admin" || r === "super_admin") {
                          setAppointJurisdictionType("national");
                          setAppointJurisdictionId("");
                          setAppointJurisdictionLabel("National (All Nigeria)");
                        } else if (r === "zonal_coordinator") {
                          setAppointJurisdictionType("zone");
                          if (allZones.length > 0 && !appointJurisdictionId) {
                            setAppointJurisdictionId(allZones[0].id);
                            setAppointJurisdictionLabel(allZones[0].name);
                          }
                        } else if (r === "state_coordinator") {
                          setAppointJurisdictionType("state");
                          if (allStates.length > 0 && !appointJurisdictionId) {
                            setAppointJurisdictionId(allStates[0].id);
                            setAppointJurisdictionLabel(allStates[0].name);
                          }
                        } else if (r === "lga_representative") {
                          setAppointJurisdictionType("lga");
                          if (allStates.length > 0 && !appointStateForLga) {
                            setAppointStateForLga(allStates[0].id);
                          }
                        }
                      }}
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
                    <label className="block font-bold text-gray-700 mb-1">Official Role Title *</label>
                    <input
                      type="text"
                      required
                      value={appointRoleTitle}
                      onChange={(e) => setAppointRoleTitle(e.target.value)}
                      placeholder="e.g. State Coordinator - Kano"
                      className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                    />
                  </div>
                </div>

                {/* 3. Jurisdiction Selector */}
                {appointJurisdictionType === "zone" && (
                  <div>
                    <label className="block font-bold text-gray-700 mb-1">Geopolitical Zone *</label>
                    <select
                      required
                      value={appointJurisdictionId}
                      onChange={(e) => {
                        const id = e.target.value;
                        setAppointJurisdictionId(id);
                        const z = allZones.find((item) => item.id === id || item.code === id);
                        if (z) setAppointJurisdictionLabel(`${z.name} (${z.code})`);
                      }}
                      className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                    >
                      <option value="">Select geopolitical zone...</option>
                      {allZones.map((z) => (
                        <option key={z.id} value={z.id}>
                          {z.name} ({z.code})
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                {appointJurisdictionType === "state" && (
                  <div>
                    <label className="block font-bold text-gray-700 mb-1">State / FCT *</label>
                    <select
                      required
                      value={appointJurisdictionId}
                      onChange={(e) => {
                        const id = e.target.value;
                        setAppointJurisdictionId(id);
                        const s = allStates.find((item) => item.id === id || item.code === id);
                        if (s) setAppointJurisdictionLabel(`${s.name} (${s.code})`);
                      }}
                      className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                    >
                      <option value="">Select state or territory...</option>
                      {allStates.map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name} ({s.code}) {s.is_fct ? "— FCT" : ""}
                        </option>
                      ))}
                    </select>
                  </div>
                )}

                {appointJurisdictionType === "lga" && (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div>
                      <label className="block font-bold text-gray-700 mb-1">State for LGA *</label>
                      <select
                        value={appointStateForLga}
                        onChange={(e) => {
                          setAppointStateForLga(e.target.value);
                          setAppointJurisdictionId("");
                        }}
                        className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                      >
                        <option value="">Select State...</option>
                        {allStates.map((s) => (
                          <option key={s.id} value={s.id}>
                            {s.name} ({s.code})
                          </option>
                        ))}
                      </select>
                    </div>

                    <div>
                      <label className="block font-bold text-gray-700 mb-1">Local Council *</label>
                      <select
                        required
                        value={appointJurisdictionId}
                        onChange={(e) => {
                          const id = e.target.value;
                          setAppointJurisdictionId(id);
                          const l = stateLgas.find((item) => item.id === id);
                          if (l) setAppointJurisdictionLabel(l.name);
                        }}
                        className="w-full border border-gray-300 rounded-xl px-3 py-2 text-xs focus:ring-emerald-600 focus:border-emerald-600 bg-white"
                      >
                        <option value="">Select LGA...</option>
                        {stateLgas.map((l) => (
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
                      value={appointStartDate}
                      onChange={(e) => setAppointStartDate(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                    />
                  </div>

                  <div>
                    <label className="block font-bold text-gray-700 mb-1">Approval Reference</label>
                    <input
                      type="text"
                      value={appointRef}
                      onChange={(e) => setAppointRef(e.target.value)}
                      placeholder="e.g. APPT-2026-HQ-001"
                      className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs font-mono focus:ring-emerald-600 focus:border-emerald-600"
                    />
                  </div>
                </div>

                <div>
                  <label className="block font-bold text-gray-700 mb-1">Public Official Contact (Optional)</label>
                  <input
                    type="text"
                    value={appointContact}
                    onChange={(e) => setAppointContact(e.target.value)}
                    placeholder="e.g. lagos.ratels@example.org or official line"
                    className="w-full px-3 py-2 border border-gray-300 rounded-xl text-xs focus:ring-emerald-600 focus:border-emerald-600"
                  />
                </div>

                <div className="flex justify-end gap-2 pt-4 border-t">
                  <button
                    type="button"
                    onClick={() => setShowAppointModal(false)}
                    className="px-4 py-2 border border-gray-300 rounded-xl font-semibold text-gray-700 hover:bg-gray-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    disabled={!appointSelectedUser || (appointJurisdictionType !== "national" && !appointJurisdictionId)}
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
                      <span className="font-bold text-gray-900">{appointSelectedUser?.display_name}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Membership Reference:</span>
                      <span className="font-mono text-gray-800">{appointSelectedUser?.membership_reference}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Role Mandate:</span>
                      <span className="font-bold text-gray-900 capitalize">
                        {appointRoleType.replace(/_/g, " ")}
                      </span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Jurisdiction:</span>
                      <span className="font-bold text-emerald-900">{appointJurisdictionLabel || appointJurisdictionType}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Effective Date:</span>
                      <span className="font-medium text-gray-800">{appointStartDate}</span>
                    </div>
                    <div>
                      <span className="text-gray-500 block">Approval Reference:</span>
                      <span className="font-mono text-gray-800">
                        {appointRef || "Auto-generated"}
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
                    disabled={submittingAppoint}
                    onClick={handleAppointSubmit}
                    className="px-5 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-xl font-bold shadow-sm transition"
                  >
                    {submittingAppoint ? "Confirming Appointment..." : "Confirm & Appoint"}
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
      {replaceTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex justify-between items-start border-b pb-3">
              <div>
                <span className="text-[10px] font-black text-amber-700 uppercase tracking-wider block">
                  Coordinator Replacement Handover
                </span>
                <h2 className="text-base font-bold text-gray-900 mt-0.5">
                  Replace: {replaceTarget.roleTitle}
                </h2>
                <div className="text-xs text-gray-500 mt-0.5">
                  Current holder: <span className="font-semibold">{replaceTarget.currentHolder}</span> ({replaceTarget.jurisdictionName})
                </div>
              </div>
              <button onClick={() => setReplaceTarget(null)} className="text-gray-400 hover:text-gray-600">
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
                    {replaceSearching && (
                      <div className="text-[11px] text-gray-500 italic">Searching candidates...</div>
                    )}
                    {replaceMemberResults.length > 0 && (
                      <div className="border border-gray-200 rounded-xl max-h-40 overflow-y-auto divide-y bg-white shadow-sm">
                        {replaceMemberResults.map((m) => (
                          <button
                            key={m.id}
                            type="button"
                            onClick={() => {
                              setReplaceSelectedUser(m);
                              setReplaceMemberResults([]);
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
                  onClick={() => setReplaceTarget(null)}
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
      {suspendTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex items-center gap-2 text-yellow-700 font-bold">
              <PauseCircle className="w-5 h-5 text-yellow-600" />
              <span>Suspend Coordinator Mandate</span>
            </div>
            <p className="text-xs text-gray-600">
              You are about to suspend <span className="font-bold text-gray-900">{suspendTarget.coordinatorName}</span> from the role of <span className="font-bold text-gray-900">{suspendTarget.roleTitle}</span>. Their administrative permissions will be halted immediately.
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
                  onClick={() => setSuspendTarget(null)}
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
      {reactivateTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex items-center gap-2 text-emerald-800 font-bold">
              <PlayCircle className="w-5 h-5 text-emerald-700" />
              <span>Reactivate Suspended Coordinator</span>
            </div>
            <p className="text-xs text-gray-600">
              Restore active mandate and administrative privileges for <span className="font-bold text-gray-900">{reactivateTarget.coordinatorName}</span> as <span className="font-bold text-gray-900">{reactivateTarget.roleTitle}</span>.
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
                  placeholder="e.g. Investigation concluded with full clearance"
                  className="w-full border border-gray-300 rounded-xl p-2.5 text-xs focus:ring-emerald-600 focus:border-emerald-600"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setReactivateTarget(null)}
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
      {revokeTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex items-center gap-2 text-red-700 font-bold">
              <AlertCircle className="w-5 h-5 text-red-600" />
              <span>Permanent Revocation of Appointment</span>
            </div>
            <p className="text-xs text-gray-600">
              This action permanently terminates the appointment of <span className="font-bold text-gray-900">{revokeTarget.coordinatorName}</span> for <span className="font-bold text-gray-900">{revokeTarget.roleTitle}</span>. Historical audit logs will be permanently retained.
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
                  onClick={() => setRevokeTarget(null)}
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
          MODAL 6: Appointment History Modal
         ========================================================================= */}
      {historyTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-4 shadow-2xl border border-gray-200">
            <div className="flex justify-between items-start border-b pb-3">
              <div>
                <h2 className="text-base font-bold text-gray-900 flex items-center gap-1.5">
                  <History className="w-5 h-5 text-emerald-800" />
                  Appointment Audit History
                </h2>
                <span className="text-xs text-gray-500">
                  {historyTarget.roleTitle} • {historyTarget.jurisdictionName}
                </span>
              </div>
              <button onClick={() => setHistoryTarget(null)} className="text-gray-400 hover:text-gray-600">
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
                onClick={() => setHistoryTarget(null)}
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
