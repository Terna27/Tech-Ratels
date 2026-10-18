"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import {
  FolderKanban,
  CheckCircle2,
  Clock,
  Coins,
  TrendingUp,
  MapPin,
  Calendar,
  AlertCircle,
  PlusCircle,
  Search,
  Filter,
  Eye,
  FileCheck,
  X,
  Upload,
  ArrowRight,
  ShieldCheck,
  Building,
  UserCheck,
  DollarSign,
  AlertTriangle,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  CommunityRequest,
  CommunityProject,
  CommunityProjectsOverviewStats,
  State,
  LGA,
  ProjectMilestone,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";

export default function AdminCommunityProjectsPage() {
  const { user, isCoordinator } = useAuth();

  const [activeTab, setActiveTab] = useState<"overview" | "requests" | "projects">("overview");

  // Overview Stats
  const [stats, setStats] = useState<CommunityProjectsOverviewStats | null>(null);

  // States & LGAs
  const [states, setStates] = useState<State[]>([]);
  const [lgas, setLgas] = useState<LGA[]>([]);

  // Requests Tab
  const [requests, setRequests] = useState<CommunityRequest[]>([]);
  const [reqTotal, setReqTotal] = useState(0);
  const [reqPage, setReqPage] = useState(0);
  const [reqStatusFilter, setReqStatusFilter] = useState("");
  const [reqStateFilter, setReqStateFilter] = useState("");
  const [loadingRequests, setLoadingRequests] = useState(false);

  // Projects Tab
  const [projects, setProjects] = useState<CommunityProject[]>([]);
  const [projTotal, setProjTotal] = useState(0);
  const [projPage, setProjPage] = useState(0);
  const [projStatusFilter, setProjStatusFilter] = useState("");
  const [projStateFilter, setProjStateFilter] = useState("");
  const [loadingProjects, setLoadingProjects] = useState(false);

  // Review Modal State
  const [reviewingReq, setReviewingReq] = useState<CommunityRequest | null>(null);
  const [reviewDecision, setReviewDecision] = useState<"verify" | "approve" | "reject" | "request_info" | "escalate">("verify");
  const [reviewNotes, setReviewNotes] = useState("");
  const [reviewRejectionReason, setReviewRejectionReason] = useState("");
  const [reviewInfoNotes, setReviewInfoNotes] = useState("");
  const [reviewLoading, setReviewLoading] = useState(false);
  const [reviewError, setReviewError] = useState<string | null>(null);

  // Create Project Modal State
  const [creatingProject, setCreatingProject] = useState(false);
  const [createReqRef, setCreateReqRef] = useState<CommunityRequest | null>(null);
  const [newTitle, setNewTitle] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [newCategory, setNewCategory] = useState("environmental_sanitation");
  const [newStateId, setNewStateId] = useState("");
  const [newLgaId, setNewLgaId] = useState("");
  const [newWard, setNewWard] = useState("");
  const [newLocation, setNewLocation] = useState("");
  const [newDept, setNewDept] = useState("Community Development & Sanitation");
  const [newBudgetNaira, setNewBudgetNaira] = useState("");
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // Manage Milestones Modal
  const [activeProjectForMilestones, setActiveProjectForMilestones] = useState<CommunityProject | null>(null);
  const [milestones, setMilestones] = useState<ProjectMilestone[]>([]);
  const [loadingMilestones, setLoadingMilestones] = useState(false);
  const [newMilestoneTitle, setNewMilestoneTitle] = useState("");
  const [newMilestoneDesc, setNewMilestoneDesc] = useState("");
  const [newMilestoneTargetDate, setNewMilestoneTargetDate] = useState("");
  const [newMilestoneAssignee, setNewMilestoneAssignee] = useState("");
  const [addMilestoneLoading, setAddMilestoneLoading] = useState(false);

  // Milestone Verification Modal
  const [verifyingMilestone, setVerifyingMilestone] = useState<ProjectMilestone | null>(null);
  const [milestoneDecision, setMilestoneDecision] = useState<"verified" | "rejected">("verified");
  const [milestoneVerifyNotes, setMilestoneVerifyNotes] = useState("");
  const [verifyMilestoneLoading, setVerifyMilestoneLoading] = useState(false);

  // Manage Funding Modal
  const [fundingProject, setFundingProject] = useState<CommunityProject | null>(null);
  const [fundingAction, setFundingAction] = useState<"allocate" | "expense">("allocate");
  const [fundingAmountNaira, setFundingAmountNaira] = useState("");
  const [fundingNotes, setFundingNotes] = useState("");
  const [fundingEvidenceUrl, setFundingEvidenceUrl] = useState("");
  const [fundingLoading, setFundingLoading] = useState(false);
  const [fundingError, setFundingError] = useState<string | null>(null);

  // Execution Status Modal
  const [execProject, setExecProject] = useState<CommunityProject | null>(null);
  const [execStatus, setExecStatus] = useState("in_progress");
  const [execProgress, setExecProgress] = useState(0);
  const [execReason, setExecReason] = useState("");
  const [execEvidence, setExecEvidence] = useState("");
  const [execLoading, setExecLoading] = useState(false);
  const [execError, setExecError] = useState<string | null>(null);

  const formatNaira = (kobo: number) => {
    const naira = Math.floor(kobo / 100);
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: "NGN",
      maximumFractionDigits: 0,
    }).format(naira);
  };

  // Initial load
  useEffect(() => {
    async function loadInitial() {
      try {
        const [statsRes, statesRes] = await Promise.all([
          apiRequest<CommunityProjectsOverviewStats>("/admin/community/overview").catch(() => null),
          apiRequest<State[]>("/geography/states").catch(() => []),
        ]);
        if (statsRes) setStats(statsRes);
        if (statesRes) setStates(statesRes);
      } catch (err) {
        console.error("Failed to load initial admin data", err);
      }
    }
    loadInitial();
  }, []);

  // Load Requests
  const loadRequests = async () => {
    setLoadingRequests(true);
    try {
      const q = new URLSearchParams();
      if (reqStatusFilter) q.set("status", reqStatusFilter);
      if (reqStateFilter) q.set("state_id", reqStateFilter);
      q.set("page", reqPage.toString());
      q.set("limit", "20");

      const res = await apiRequest<{ requests: CommunityRequest[]; total: number }>(
        `/admin/community/requests?${q.toString()}`
      );
      setRequests(res?.requests || []);
      setReqTotal(res?.total || 0);
    } catch (err) {
      console.error("Failed to load requests", err);
    } finally {
      setLoadingRequests(false);
    }
  };

  useEffect(() => {
    if (activeTab === "requests") {
      loadRequests();
    }
  }, [activeTab, reqStatusFilter, reqStateFilter, reqPage]);

  // Load Projects
  const loadProjects = async () => {
    setLoadingProjects(true);
    try {
      const q = new URLSearchParams();
      if (projStatusFilter) q.set("execution_status", projStatusFilter);
      if (projStateFilter) q.set("state_id", projStateFilter);
      q.set("page", projPage.toString());
      q.set("limit", "20");

      const res = await apiRequest<{ projects: CommunityProject[]; total: number }>(
        `/admin/community/projects?${q.toString()}`
      );
      setProjects(res?.projects || []);
      setProjTotal(res?.total || 0);
    } catch (err) {
      console.error("Failed to load projects", err);
    } finally {
      setLoadingProjects(false);
    }
  };

  useEffect(() => {
    if (activeTab === "projects") {
      loadProjects();
    }
  }, [activeTab, projStatusFilter, projStateFilter, projPage]);

  // Handle Review Request
  const handleReviewSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reviewingReq) return;
    setReviewLoading(true);
    setReviewError(null);

    try {
      await apiRequest(`/admin/community/requests/${reviewingReq.id}/review`, {
        method: "POST",
        body: JSON.stringify({
          decision: reviewDecision,
          review_notes: reviewNotes.trim() ? reviewNotes.trim() : undefined,
          rejection_reason: reviewRejectionReason.trim() ? reviewRejectionReason.trim() : undefined,
          info_request_notes: reviewInfoNotes.trim() ? reviewInfoNotes.trim() : undefined,
        }),
      });

      setReviewingReq(null);
      await loadRequests();
      // refresh stats
      const statsRes = await apiRequest<CommunityProjectsOverviewStats>("/admin/community/overview").catch(() => null);
      if (statsRes) setStats(statsRes);
    } catch (err: any) {
      console.error("Failed to review request", err);
      setReviewError(err?.message || "Failed to submit review.");
    } finally {
      setReviewLoading(false);
    }
  };

  // Convert Request to Project
  const handleOpenCreateProject = (req?: CommunityRequest) => {
    if (req) {
      setCreateReqRef(req);
      setNewTitle(req.title);
      setNewDesc(req.description);
      setNewCategory(req.category);
      setNewStateId(req.state_id);
      setNewLgaId(req.lga_id);
      setNewWard(req.community_ward);
      setNewLocation(req.location_details);
    } else {
      setCreateReqRef(null);
      setNewTitle("");
      setNewDesc("");
      setNewCategory("environmental_sanitation");
      setNewStateId("");
      setNewLgaId("");
      setNewWard("");
      setNewLocation("");
    }
    setNewBudgetNaira("");
    setCreateError(null);
    setCreatingProject(true);
  };

  const handleCreateProjectSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateLoading(true);
    setCreateError(null);

    try {
      const budgetKobo = Math.floor(parseFloat(newBudgetNaira || "0") * 100);

      const payload = {
        community_request_id: createReqRef ? createReqRef.id : undefined,
        title: newTitle.trim(),
        description: newDesc.trim(),
        category: newCategory,
        state_id: newStateId,
        lga_id: newLgaId,
        community_ward: newWard.trim(),
        location_details: newLocation.trim(),
        responsible_department: newDept,
        proposed_budget: budgetKobo,
        approved_budget: budgetKobo,
        is_public: true,
      };

      await apiRequest("/admin/community/projects", {
        method: "POST",
        body: JSON.stringify(payload),
      });

      setCreatingProject(false);
      setActiveTab("projects");
      await loadProjects();
    } catch (err: any) {
      console.error("Failed to create project", err);
      setCreateError(err?.message || "Failed to create project.");
    } finally {
      setCreateLoading(false);
    }
  };

  // Load Milestones for a project
  const handleOpenMilestones = async (p: CommunityProject) => {
    setActiveProjectForMilestones(p);
    setLoadingMilestones(true);
    try {
      const res = await apiRequest<{ milestones: ProjectMilestone[] }>(`/admin/community/projects/${p.id}`);
      setMilestones(res?.milestones || []);
    } catch (err) {
      console.error("Failed to load milestones", err);
    } finally {
      setLoadingMilestones(false);
    }
  };

  const handleAddMilestone = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!activeProjectForMilestones) return;
    setAddMilestoneLoading(true);

    try {
      await apiRequest(`/admin/community/projects/${activeProjectForMilestones.id}/milestones`, {
        method: "POST",
        body: JSON.stringify({
          sequence_order: milestones.length + 1,
          title: newMilestoneTitle.trim(),
          description: newMilestoneDesc.trim(),
          target_date: newMilestoneTargetDate,
          assigned_person_name: newMilestoneAssignee.trim() ? newMilestoneAssignee.trim() : undefined,
          status: "planned",
        }),
      });

      setNewMilestoneTitle("");
      setNewMilestoneDesc("");
      setNewMilestoneTargetDate("");
      setNewMilestoneAssignee("");

      // reload
      const res = await apiRequest<{ milestones: ProjectMilestone[] }>(
        `/admin/community/projects/${activeProjectForMilestones.id}`
      );
      setMilestones(res?.milestones || []);
    } catch (err: any) {
      alert(err?.message || "Failed to add milestone");
    } finally {
      setAddMilestoneLoading(false);
    }
  };

  const handleVerifyMilestoneSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!verifyingMilestone || !activeProjectForMilestones) return;
    setVerifyMilestoneLoading(true);

    try {
      await apiRequest(
        `/admin/community/projects/${activeProjectForMilestones.id}/milestones/${verifyingMilestone.id}/verify`,
        {
          method: "POST",
          body: JSON.stringify({
            decision: milestoneDecision,
            verification_notes: milestoneVerifyNotes.trim(),
          }),
        }
      );

      setVerifyingMilestone(null);
      setMilestoneVerifyNotes("");

      // reload milestones
      const res = await apiRequest<{ milestones: ProjectMilestone[] }>(
        `/admin/community/projects/${activeProjectForMilestones.id}`
      );
      setMilestones(res?.milestones || []);
    } catch (err: any) {
      alert(err?.message || "Failed to verify milestone.");
    } finally {
      setVerifyMilestoneLoading(false);
    }
  };

  // Funding Submit
  const handleFundingSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!fundingProject) return;
    setFundingLoading(true);
    setFundingError(null);

    try {
      const amountKobo = Math.floor(parseFloat(fundingAmountNaira || "0") * 100);
      await apiRequest(`/admin/community/projects/${fundingProject.id}/funding`, {
        method: "POST",
        body: JSON.stringify({
          action: fundingAction,
          amount_kobo: amountKobo,
          notes: fundingNotes.trim() ? fundingNotes.trim() : undefined,
          evidence_url: fundingEvidenceUrl.trim() ? fundingEvidenceUrl.trim() : undefined,
        }),
      });

      setFundingProject(null);
      await loadProjects();
    } catch (err: any) {
      console.error("Failed to record funding", err);
      setFundingError(err?.message || "Failed to record funding transaction.");
    } finally {
      setFundingLoading(false);
    }
  };

  // Execution Status Submit
  const handleExecSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!execProject) return;
    setExecLoading(true);
    setExecError(null);

    try {
      await apiRequest(`/admin/community/projects/${execProject.id}/execution`, {
        method: "PATCH",
        body: JSON.stringify({
          execution_status: execStatus,
          progress_percentage: Number(execProgress),
          pause_or_cancel_reason: execReason.trim() ? execReason.trim() : undefined,
          verification_notes: execReason.trim() ? execReason.trim() : undefined,
          completion_evidence_urls: execEvidence.trim() ? [execEvidence.trim()] : [],
        }),
      });

      setExecProject(null);
      await loadProjects();
    } catch (err: any) {
      console.error("Failed to update execution", err);
      setExecError(err?.message || "Failed to update execution status.");
    } finally {
      setExecLoading(false);
    }
  };

  return (
    <div className="space-y-8 pb-16">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold uppercase tracking-wider text-emerald-800 bg-emerald-50 px-2.5 py-1 rounded">
              Blueprint Section 10 Operational Management
            </span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
            Community Projects & Requests Console
          </h1>
          <p className="text-sm text-gray-500">
            Verify community requests, convert approved needs into projects, track milestones, and manage ledger funding.
          </p>
        </div>

        <button
          type="button"
          onClick={() => handleOpenCreateProject()}
          className="inline-flex items-center gap-2 px-4 py-2.5 text-xs font-bold rounded-xl text-white bg-emerald-900 hover:bg-emerald-800 shadow-sm transition"
        >
          <PlusCircle className="w-4 h-4" />
          Initiate New Project
        </button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 gap-6 text-sm font-semibold">
        <button
          type="button"
          onClick={() => setActiveTab("overview")}
          className={`pb-3 border-b-2 transition ${
            activeTab === "overview"
              ? "border-emerald-600 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          Overview & KPIs
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("requests")}
          className={`pb-3 border-b-2 transition flex items-center gap-2 ${
            activeTab === "requests"
              ? "border-emerald-600 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          <span>Community Requests Queue</span>
          {stats && stats.pending_requests > 0 && (
            <span className="px-2 py-0.5 rounded-full text-xs bg-amber-100 text-amber-800 font-bold">
              {stats.pending_requests}
            </span>
          )}
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("projects")}
          className={`pb-3 border-b-2 transition ${
            activeTab === "projects"
              ? "border-emerald-600 text-emerald-800"
              : "border-transparent text-gray-500 hover:text-gray-700"
          }`}
        >
          Managed Projects & Finances
        </button>
      </div>

      {/* Tab 1: Overview */}
      {activeTab === "overview" && stats && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 uppercase tracking-wider font-semibold">
                Total Requests
              </span>
              <p className="text-2xl font-bold text-gray-900">{stats.total_requests}</p>
              <p className="text-xs text-gray-400">Neighborhood submissions</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-amber-700 uppercase tracking-wider font-semibold">
                Pending Verification
              </span>
              <p className="text-2xl font-bold text-amber-900">{stats.pending_requests}</p>
              <p className="text-xs text-gray-400">Awaiting coordinator review</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-blue-700 uppercase tracking-wider font-semibold">
                Active Projects
              </span>
              <p className="text-2xl font-bold text-blue-900">{stats.active_projects}</p>
              <p className="text-xs text-gray-400">Executing in field</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-emerald-700 uppercase tracking-wider font-semibold">
                Completed Interventions
              </span>
              <p className="text-2xl font-bold text-emerald-900">{stats.completed_projects}</p>
              <p className="text-xs text-gray-400">100% verified milestones</p>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-gray-500 font-semibold uppercase">Authorized Budget</span>
              <p className="text-xl font-bold text-gray-900">{formatNaira(stats.total_approved_budget)}</p>
              <p className="text-xs text-gray-400">Total approved project budgets</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-blue-700 font-semibold uppercase">Allocated Funding</span>
              <p className="text-xl font-bold text-blue-900">{formatNaira(stats.total_allocated_funding)}</p>
              <p className="text-xs text-gray-400">Moved into project funds</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <span className="text-xs text-purple-700 font-semibold uppercase">Recorded Expenditure</span>
              <p className="text-xl font-bold text-purple-900">{formatNaira(stats.total_recorded_expenditure)}</p>
              <p className="text-xs text-gray-400">Disbursed with proof of work</p>
            </div>
          </div>
        </div>
      )}

      {/* Tab 2: Community Requests Queue */}
      {activeTab === "requests" && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3 p-4 bg-white rounded-xl border border-gray-200">
            <div className="flex flex-wrap gap-2 text-xs">
              <select
                value={reqStatusFilter}
                onChange={(e) => setReqStatusFilter(e.target.value)}
                className="px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
              >
                <option value="">All Statuses</option>
                <option value="submitted">Submitted (New)</option>
                <option value="under_review">Under Review</option>
                <option value="info_requested">Info Requested</option>
                <option value="verified">Verified on Ground</option>
                <option value="approved">Approved</option>
                <option value="rejected">Rejected</option>
              </select>

              <select
                value={reqStateFilter}
                onChange={(e) => setReqStateFilter(e.target.value)}
                className="px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
              >
                <option value="">All States</option>
                {states.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </div>

            <span className="text-xs text-gray-500">
              Showing {requests.length} of {reqTotal} requests
            </span>
          </div>

          {loadingRequests ? (
            <div className="p-12 text-center text-gray-500 text-xs">Loading requests queue...</div>
          ) : requests.length === 0 ? (
            <div className="p-12 bg-white rounded-xl border border-dashed border-gray-300 text-center text-xs text-gray-500">
              No requests found matching filters.
            </div>
          ) : (
            <div className="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-700">
                  <thead className="bg-gray-50 border-b border-gray-200 text-gray-600 font-bold uppercase tracking-wider">
                    <tr>
                      <th className="p-3">Reference</th>
                      <th className="p-3">Title & Category</th>
                      <th className="p-3">Location</th>
                      <th className="p-3">Requester</th>
                      <th className="p-3">Urgency</th>
                      <th className="p-3">Status</th>
                      <th className="p-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {requests.map((r) => (
                      <tr key={r.id} className="hover:bg-gray-50/60 transition">
                        <td className="p-3 font-mono font-bold text-emerald-900">{r.reference}</td>
                        <td className="p-3 space-y-0.5 max-w-xs">
                          <div className="font-bold text-gray-900 truncate">{r.title}</div>
                          <div className="text-gray-400 text-xxs capitalize">{r.category.replace(/_/g, " ")}</div>
                        </td>
                        <td className="p-3">
                          {r.community_ward}, {r.lga_name || "LGA"}, {r.state_name || "State"}
                        </td>
                        <td className="p-3 space-y-0.5">
                          <div className="font-semibold text-gray-900">{r.requester_name || "Member"}</div>
                          <div className="text-gray-400 text-xxs">{r.requester_phone || r.requester_email}</div>
                        </td>
                        <td className="p-3">
                          <span
                            className={`px-2 py-0.5 rounded font-bold uppercase text-xxs ${
                              r.estimated_urgency === "critical"
                                ? "bg-red-100 text-red-800"
                                : r.estimated_urgency === "high"
                                ? "bg-amber-100 text-amber-800"
                                : "bg-gray-100 text-gray-700"
                            }`}
                          >
                            {r.estimated_urgency}
                          </span>
                        </td>
                        <td className="p-3">
                          <Badge variant={r.status === "approved" ? "success" : r.status === "rejected" ? "danger" : "neutral"}>
                            {r.status.replace(/_/g, " ")}
                          </Badge>
                        </td>
                        <td className="p-3 text-right space-x-1.5 whitespace-nowrap">
                          <button
                            type="button"
                            onClick={() => {
                              setReviewingReq(r);
                              setReviewDecision("verify");
                              setReviewNotes("");
                              setReviewRejectionReason("");
                              setReviewInfoNotes("");
                              setReviewError(null);
                            }}
                            className="px-2.5 py-1 text-xs font-semibold rounded bg-emerald-50 text-emerald-800 hover:bg-emerald-100 transition"
                          >
                            Review / Verify
                          </button>
                          {r.status === "approved" && !r.converted_to_project_id && (
                            <button
                              type="button"
                              onClick={() => handleOpenCreateProject(r)}
                              className="px-2.5 py-1 text-xs font-bold rounded bg-emerald-900 text-white hover:bg-emerald-800 transition"
                            >
                              Convert to Project
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Tab 3: Managed Projects & Finances */}
      {activeTab === "projects" && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3 p-4 bg-white rounded-xl border border-gray-200">
            <div className="flex flex-wrap gap-2 text-xs">
              <select
                value={projStatusFilter}
                onChange={(e) => setProjStatusFilter(e.target.value)}
                className="px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
              >
                <option value="">All Statuses</option>
                <option value="planned">Planned</option>
                <option value="in_progress">In Progress</option>
                <option value="awaiting_verification">Awaiting Verification</option>
                <option value="completed">Completed</option>
                <option value="paused">Paused</option>
                <option value="cancelled">Cancelled</option>
              </select>

              <select
                value={projStateFilter}
                onChange={(e) => setProjStateFilter(e.target.value)}
                className="px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
              >
                <option value="">All States</option>
                {states.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </div>

            <span className="text-xs text-gray-500">
              Showing {projects.length} of {projTotal} projects
            </span>
          </div>

          {loadingProjects ? (
            <div className="p-12 text-center text-gray-500 text-xs">Loading projects list...</div>
          ) : projects.length === 0 ? (
            <div className="p-12 bg-white rounded-xl border border-dashed border-gray-300 text-center text-xs text-gray-500">
              No projects found.
            </div>
          ) : (
            <div className="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-700">
                  <thead className="bg-gray-50 border-b border-gray-200 text-gray-600 font-bold uppercase tracking-wider">
                    <tr>
                      <th className="p-3">Reference</th>
                      <th className="p-3">Project Title</th>
                      <th className="p-3">Location</th>
                      <th className="p-3">Status</th>
                      <th className="p-3">Progress</th>
                      <th className="p-3">Budget & Spent</th>
                      <th className="p-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {projects.map((p) => (
                      <tr key={p.id} className="hover:bg-gray-50/60 transition">
                        <td className="p-3 font-mono font-bold text-emerald-900">{p.reference}</td>
                        <td className="p-3 space-y-0.5 max-w-xs">
                          <div className="font-bold text-gray-900 truncate">{p.title}</div>
                          <div className="text-gray-400 text-xxs capitalize">{p.category.replace(/_/g, " ")}</div>
                        </td>
                        <td className="p-3">
                          {p.community_ward}, {p.lga_name || "LGA"}, {p.state_name || "State"}
                        </td>
                        <td className="p-3">
                          <Badge variant={p.execution_status === "completed" ? "success" : "neutral"}>
                            {p.execution_status.replace(/_/g, " ")}
                          </Badge>
                        </td>
                        <td className="p-3">
                          <div className="space-y-1 w-24">
                            <div className="font-bold text-emerald-800">{p.progress_percentage}%</div>
                            <div className="w-full bg-gray-100 rounded-full h-1.5">
                              <div
                                className="bg-emerald-600 h-1.5 rounded-full"
                                style={{ width: `${p.progress_percentage}%` }}
                              />
                            </div>
                          </div>
                        </td>
                        <td className="p-3 space-y-0.5">
                          <div className="font-semibold text-gray-900">
                            Budget: {formatNaira(p.approved_budget)}
                          </div>
                          <div className="text-emerald-700 font-medium">
                            Spent: {formatNaira(p.recorded_expenditure)}
                          </div>
                        </td>
                        <td className="p-3 text-right space-x-1 whitespace-nowrap">
                          <button
                            type="button"
                            onClick={() => handleOpenMilestones(p)}
                            className="px-2 py-1 text-xs font-semibold rounded bg-blue-50 text-blue-800 hover:bg-blue-100 transition"
                          >
                            Milestones
                          </button>
                          <button
                            type="button"
                            onClick={() => {
                              setFundingProject(p);
                              setFundingAction("allocate");
                              setFundingAmountNaira("");
                              setFundingNotes("");
                              setFundingEvidenceUrl("");
                              setFundingError(null);
                            }}
                            className="px-2 py-1 text-xs font-semibold rounded bg-emerald-50 text-emerald-800 hover:bg-emerald-100 transition"
                          >
                            Funding
                          </button>
                          <button
                            type="button"
                            onClick={() => {
                              setExecProject(p);
                              setExecStatus(p.execution_status);
                              setExecProgress(p.progress_percentage);
                              setExecReason("");
                              setExecEvidence("");
                              setExecError(null);
                            }}
                            className="px-2 py-1 text-xs font-semibold rounded bg-gray-100 text-gray-800 hover:bg-gray-200 transition"
                          >
                            Status
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Review Request Modal */}
      {reviewingReq && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-5 shadow-2xl max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-xs font-mono font-bold text-emerald-800 bg-emerald-50 px-2 py-0.5 rounded">
                  {reviewingReq.reference}
                </span>
                <h3 className="text-base font-bold text-gray-900 pt-1">
                  Official Request Review
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setReviewingReq(null)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {reviewError && (
              <div className="p-3 bg-red-50 text-red-800 rounded-lg text-xs">{reviewError}</div>
            )}

            <div className="p-4 bg-gray-50 rounded-xl space-y-2 text-xs border border-gray-100">
              <h4 className="font-bold text-gray-900">{reviewingReq.title}</h4>
              <p className="text-gray-600 whitespace-pre-line">{reviewingReq.description}</p>
              <div className="pt-2 border-t border-gray-200 flex flex-wrap gap-4 text-gray-500">
                <span>Location: {reviewingReq.community_ward}, {reviewingReq.lga_name}, {reviewingReq.state_name}</span>
                <span>Urgency: <strong className="capitalize">{reviewingReq.estimated_urgency}</strong></span>
                <span>Requester: <strong>{reviewingReq.requester_name}</strong> ({reviewingReq.requester_phone})</span>
              </div>
            </div>

            <form onSubmit={handleReviewSubmit} className="space-y-4 text-xs">
              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Review Decision:</label>
                <select
                  value={reviewDecision}
                  onChange={(e) => setReviewDecision(e.target.value as any)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg focus:ring-2 focus:ring-emerald-500 font-semibold"
                >
                  <option value="verify">Verify on Ground (Verified Status)</option>
                  <option value="approve">Approve for Intervention (Approved Status)</option>
                  <option value="request_info">Request Additional Information / Resubmission</option>
                  <option value="reject">Decline / Reject Request</option>
                  <option value="escalate">Escalate to Higher Administrative Level</option>
                </select>
              </div>

              {reviewDecision === "request_info" && (
                <div className="space-y-1">
                  <label className="font-bold text-amber-900 block">Information Required from Requester *:</label>
                  <textarea
                    required
                    rows={3}
                    placeholder="Describe what details, photos, or measurements are required before verification can proceed..."
                    value={reviewInfoNotes}
                    onChange={(e) => setReviewInfoNotes(e.target.value)}
                    className="w-full px-3 py-2 bg-amber-50 border border-amber-300 rounded-lg text-amber-950"
                  />
                </div>
              )}

              {reviewDecision === "reject" && (
                <div className="space-y-1">
                  <label className="font-bold text-red-900 block">Documented Reason for Rejection *:</label>
                  <textarea
                    required
                    rows={3}
                    placeholder="Provide clear documented reasons why this request cannot be approved..."
                    value={reviewRejectionReason}
                    onChange={(e) => setReviewRejectionReason(e.target.value)}
                    className="w-full px-3 py-2 bg-red-50 border border-red-300 rounded-lg text-red-950"
                  />
                </div>
              )}

              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Internal Review & Inspection Notes:</label>
                <textarea
                  rows={2}
                  placeholder="Record on-ground inspection observations, volunteer availability, and logistic notes..."
                  value={reviewNotes}
                  onChange={(e) => setReviewNotes(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setReviewingReq(null)}
                  className="px-4 py-2 font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={reviewLoading}
                  className="px-4 py-2 font-bold text-white bg-emerald-900 hover:bg-emerald-800 rounded-lg disabled:opacity-50"
                >
                  {reviewLoading ? "Saving Review..." : "Confirm Review Decision"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Create Project Modal */}
      {creatingProject && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-xl w-full p-6 space-y-4 shadow-2xl max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-bold text-gray-900">
                {createReqRef ? `Convert Request ${createReqRef.reference} to Project` : "Initiate Managed Community Project"}
              </h3>
              <button
                type="button"
                onClick={() => setCreatingProject(false)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {createError && (
              <div className="p-3 bg-red-50 text-red-800 rounded-lg text-xs">{createError}</div>
            )}

            <form onSubmit={handleCreateProjectSubmit} className="space-y-4 text-xs">
              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Project Title *</label>
                <input
                  type="text"
                  required
                  value={newTitle}
                  onChange={(e) => setNewTitle(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                />
              </div>

              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Description & Scope *</label>
                <textarea
                  required
                  rows={3}
                  value={newDesc}
                  onChange={(e) => setNewDesc(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1">
                  <label className="font-bold text-gray-700 block">State *</label>
                  <select
                    required
                    value={newStateId}
                    onChange={(e) => setNewStateId(e.target.value)}
                    className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                  >
                    <option value="">Select State</option>
                    {states.map((s) => (
                      <option key={s.id} value={s.id}>{s.name}</option>
                    ))}
                  </select>
                </div>

                <div className="space-y-1">
                  <label className="font-bold text-gray-700 block">Approved Budget (NGN) *</label>
                  <input
                    type="number"
                    required
                    placeholder="e.g. 500000"
                    value={newBudgetNaira}
                    onChange={(e) => setNewBudgetNaira(e.target.value)}
                    className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg font-semibold"
                  />
                </div>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setCreatingProject(false)}
                  className="px-4 py-2 font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createLoading}
                  className="px-4 py-2 font-bold text-white bg-emerald-900 hover:bg-emerald-800 rounded-lg disabled:opacity-50"
                >
                  {createLoading ? "Creating..." : "Authorize Project Creation"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Milestones Management Modal */}
      {activeProjectForMilestones && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-2xl w-full p-6 space-y-5 shadow-2xl max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-xs font-mono font-bold text-emerald-800">
                  {activeProjectForMilestones.reference}
                </span>
                <h3 className="text-base font-bold text-gray-900">
                  Project Milestones & Verification Trail
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setActiveProjectForMilestones(null)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* List Existing Milestones */}
            <div className="space-y-3">
              <h4 className="text-xs font-bold text-gray-700 uppercase">Existing Milestones</h4>
              {loadingMilestones ? (
                <div className="text-xs text-gray-500 py-4 text-center">Loading milestones...</div>
              ) : milestones.length === 0 ? (
                <div className="text-xs text-gray-500 py-4 text-center bg-gray-50 rounded-lg">
                  No milestones added yet. Add the first phase below.
                </div>
              ) : (
                milestones.map((m) => (
                  <div key={m.id} className="p-3 bg-gray-50 rounded-xl border border-gray-200 space-y-2 text-xs">
                    <div className="flex items-center justify-between">
                      <div className="font-bold text-gray-900">
                        Phase {m.sequence_order}: {m.title}
                      </div>
                      <div className="flex items-center gap-1.5">
                        <Badge variant={m.status === "completed" ? "success" : "neutral"}>
                          {m.status.replace(/_/g, " ")}
                        </Badge>
                        <span
                          className={`px-2 py-0.5 rounded text-xxs font-bold ${
                            m.verification_status === "verified"
                              ? "bg-emerald-100 text-emerald-800"
                              : "bg-amber-100 text-amber-800"
                          }`}
                        >
                          {m.verification_status === "verified" ? "Verified" : "Unverified"}
                        </span>
                      </div>
                    </div>
                    <p className="text-gray-600">{m.description}</p>
                    <div className="flex items-center justify-between text-gray-400 text-xxs pt-1 border-t border-gray-200/60">
                      <span>Target: {new Date(m.target_date).toLocaleDateString()}</span>
                      {m.verification_status !== "verified" && (
                        <button
                          type="button"
                          onClick={() => {
                            setVerifyingMilestone(m);
                            setMilestoneDecision("verified");
                            setMilestoneVerifyNotes("");
                          }}
                          className="px-2 py-0.5 font-bold rounded bg-emerald-800 text-white hover:bg-emerald-700"
                        >
                          Verify Milestone
                        </button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* Add Milestone Form */}
            <form onSubmit={handleAddMilestone} className="p-4 bg-emerald-50/50 rounded-xl border border-emerald-100 space-y-3 text-xs">
              <h4 className="font-bold text-emerald-950 uppercase tracking-wider">Add New Milestone</h4>
              <div className="grid grid-cols-2 gap-3">
                <input
                  type="text"
                  required
                  placeholder="Milestone Title (e.g. Excavation Phase 1)"
                  value={newMilestoneTitle}
                  onChange={(e) => setNewMilestoneTitle(e.target.value)}
                  className="px-3 py-2 bg-white border border-gray-300 rounded-lg col-span-2"
                />
                <textarea
                  required
                  rows={2}
                  placeholder="Milestone scope description..."
                  value={newMilestoneDesc}
                  onChange={(e) => setNewMilestoneDesc(e.target.value)}
                  className="px-3 py-2 bg-white border border-gray-300 rounded-lg col-span-2"
                />
                <input
                  type="date"
                  required
                  value={newMilestoneTargetDate}
                  onChange={(e) => setNewMilestoneTargetDate(e.target.value)}
                  className="px-3 py-2 bg-white border border-gray-300 rounded-lg"
                />
                <input
                  type="text"
                  placeholder="Assigned Person / Unit Name"
                  value={newMilestoneAssignee}
                  onChange={(e) => setNewMilestoneAssignee(e.target.value)}
                  className="px-3 py-2 bg-white border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end pt-1">
                <button
                  type="submit"
                  disabled={addMilestoneLoading}
                  className="px-4 py-2 font-bold text-white bg-emerald-900 hover:bg-emerald-800 rounded-lg disabled:opacity-50"
                >
                  {addMilestoneLoading ? "Adding..." : "Add Milestone"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Verify Milestone Modal */}
      {verifyingMilestone && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-bold text-gray-900">
                Independent Milestone Verification
              </h3>
              <button
                type="button"
                onClick={() => setVerifyingMilestone(null)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <p className="text-xs text-gray-600">
              Verifying: <strong>{verifyingMilestone.title}</strong>
            </p>

            <form onSubmit={handleVerifyMilestoneSubmit} className="space-y-4 text-xs">
              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Inspection Decision:</label>
                <select
                  value={milestoneDecision}
                  onChange={(e) => setMilestoneDecision(e.target.value as any)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                >
                  <option value="verified">Verified (Confirm Completion)</option>
                  <option value="rejected">Rejected (Unsatisfactory Execution)</option>
                </select>
              </div>

              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Inspection Notes *:</label>
                <textarea
                  required
                  rows={3}
                  placeholder="Document physical site observations, photographic evidence confirmation, and quality of work..."
                  value={milestoneVerifyNotes}
                  onChange={(e) => setMilestoneVerifyNotes(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setVerifyingMilestone(null)}
                  className="px-4 py-2 font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={verifyMilestoneLoading}
                  className="px-4 py-2 font-bold text-white bg-emerald-900 hover:bg-emerald-800 rounded-lg disabled:opacity-50"
                >
                  {verifyMilestoneLoading ? "Verifying..." : "Confirm Verification"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Record Funding Modal */}
      {fundingProject && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-xs font-mono font-bold text-emerald-800">
                  {fundingProject.reference}
                </span>
                <h3 className="text-base font-bold text-gray-900">
                  Record Project Funding & Ledger
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setFundingProject(null)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {fundingError && (
              <div className="p-3 bg-red-50 text-red-800 rounded-lg text-xs">{fundingError}</div>
            )}

            <form onSubmit={handleFundingSubmit} className="space-y-4 text-xs">
              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Transaction Type:</label>
                <select
                  value={fundingAction}
                  onChange={(e) => setFundingAction(e.target.value as any)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                >
                  <option value="allocate">Allocate Funding (Credit Project Fund)</option>
                  <option value="expense">Record Expenditure / Disbursement (Debit Project Fund)</option>
                </select>
              </div>

              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Amount in Naira (NGN) *:</label>
                <input
                  type="number"
                  required
                  placeholder="e.g. 250000"
                  value={fundingAmountNaira}
                  onChange={(e) => setFundingAmountNaira(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg font-bold"
                />
              </div>

              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Supporting Evidence URL (Receipt/Invoice):</label>
                <input
                  type="url"
                  placeholder="https://pub-...r2.dev/finance/receipt.jpg"
                  value={fundingEvidenceUrl}
                  onChange={(e) => setFundingEvidenceUrl(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                />
              </div>

              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Ledger Notes / Justification:</label>
                <textarea
                  rows={2}
                  placeholder="Details of allocation or expense description..."
                  value={fundingNotes}
                  onChange={(e) => setFundingNotes(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setFundingProject(null)}
                  className="px-4 py-2 font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={fundingLoading}
                  className="px-4 py-2 font-bold text-white bg-emerald-900 hover:bg-emerald-800 rounded-lg disabled:opacity-50"
                >
                  {fundingLoading ? "Recording..." : "Post to General Ledger"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Execution Status Modal */}
      {execProject && (
        <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-xs font-mono font-bold text-emerald-800">
                  {execProject.reference}
                </span>
                <h3 className="text-base font-bold text-gray-900">
                  Update Execution Status
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setExecProject(null)}
                className="p-1 text-gray-400 hover:text-gray-600 rounded-lg"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {execError && (
              <div className="p-3 bg-red-50 text-red-800 rounded-lg text-xs">{execError}</div>
            )}

            <form onSubmit={handleExecSubmit} className="space-y-4 text-xs">
              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Execution Status:</label>
                <select
                  value={execStatus}
                  onChange={(e) => setExecStatus(e.target.value)}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg font-semibold"
                >
                  <option value="planned">Planned</option>
                  <option value="in_progress">In Progress</option>
                  <option value="awaiting_verification">Awaiting Verification</option>
                  <option value="completed">Completed (Requires 100% Verified Milestones)</option>
                  <option value="paused">Paused</option>
                  <option value="cancelled">Cancelled</option>
                </select>
              </div>

              <div className="space-y-1">
                <label className="font-bold text-gray-700 block">Progress Percentage ({execProgress}%):</label>
                <input
                  type="range"
                  min="0"
                  max="100"
                  value={execProgress}
                  onChange={(e) => setExecProgress(Number(e.target.value))}
                  className="w-full"
                />
              </div>

              {(execStatus === "paused" || execStatus === "cancelled") && (
                <div className="space-y-1">
                  <label className="font-bold text-amber-900 block">Reason for Pause / Cancellation *:</label>
                  <textarea
                    required
                    rows={2}
                    placeholder="Documented reason is strictly required..."
                    value={execReason}
                    onChange={(e) => setExecReason(e.target.value)}
                    className="w-full px-3 py-2 bg-amber-50 border border-amber-300 rounded-lg"
                  />
                </div>
              )}

              {execStatus === "completed" && (
                <div className="space-y-2">
                  <div className="space-y-1">
                    <label className="font-bold text-emerald-900 block">Completion Verification Notes *:</label>
                    <textarea
                      required
                      rows={2}
                      placeholder="Notes certifying final physical inspection and handover..."
                      value={execReason}
                      onChange={(e) => setExecReason(e.target.value)}
                      className="w-full px-3 py-2 bg-emerald-50 border border-emerald-300 rounded-lg"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="font-bold text-emerald-900 block">Completion Evidence URL:</label>
                    <input
                      type="url"
                      placeholder="https://pub-...r2.dev/community/projects/completed.jpg"
                      value={execEvidence}
                      onChange={(e) => setExecEvidence(e.target.value)}
                      className="w-full px-3 py-2 bg-gray-50 border border-gray-300 rounded-lg"
                    />
                  </div>
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setExecProject(null)}
                  className="px-4 py-2 font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={execLoading}
                  className="px-4 py-2 font-bold text-white bg-emerald-900 hover:bg-emerald-800 rounded-lg disabled:opacity-50"
                >
                  {execLoading ? "Updating..." : "Update Status"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
