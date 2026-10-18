"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Briefcase,
  Building2,
  Users,
  CheckCircle2,
  XCircle,
  Clock,
  ShieldCheck,
  Search,
  Filter,
  ArrowRight,
  RefreshCw,
  AlertCircle,
  ExternalLink,
  ChevronRight,
  Sparkles,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import {
  CareersAdminOverview,
  CareersEmployer,
  CareersJob,
  CareersApplication,
} from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";

type AdminTab = "overview" | "employers" | "jobs" | "applications";

export default function AdminCareersPage() {
  const router = useRouter();
  const { user, isCoordinator, isAdmin, isSuperAdmin, isLoading: authLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<AdminTab>("overview");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Data states
  const [overview, setOverview] = useState<CareersAdminOverview | null>(null);
  const [employers, setEmployers] = useState<CareersEmployer[]>([]);
  const [jobs, setJobs] = useState<CareersJob[]>([]);
  const [applications, setApplications] = useState<CareersApplication[]>([]);

  // Filter states
  const [empStatusFilter, setEmpStatusFilter] = useState<string>("");
  const [jobStatusFilter, setJobStatusFilter] = useState<string>("");
  const [appStatusFilter, setAppStatusFilter] = useState<string>("");

  // Verification modal state
  const [verifyingEmployer, setVerifyingEmployer] = useState<CareersEmployer | null>(null);
  const [verifyApproved, setVerifyApproved] = useState<boolean>(true);
  const [verifyReason, setVerifyReason] = useState<string>("");
  const [submittingVerify, setSubmittingVerify] = useState<boolean>(false);

  // Moderation modal state
  const [moderatingJob, setModeratingJob] = useState<CareersJob | null>(null);
  const [modStatus, setModStatus] = useState<string>("published");
  const [modNotes, setModNotes] = useState<string>("");
  const [submittingMod, setSubmittingMod] = useState<boolean>(false);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/admin/careers");
      return;
    }

    if (user && !isCoordinator && !isAdmin && !isSuperAdmin) {
      setError("Unauthorized. Administrative privileges required.");
      setLoading(false);
      return;
    }

    if (user) {
      loadAllAdminData();
    }
  }, [user, authLoading, isCoordinator, isAdmin, isSuperAdmin]);

  const loadAllAdminData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [ovRes, empRes, jobRes, appRes] = await Promise.allSettled([
        apiRequest<CareersAdminOverview>("/careers/admin/overview"),
        apiRequest<CareersEmployer[]>("/careers/admin/employers"),
        apiRequest<CareersJob[]>("/careers/admin/jobs"),
        apiRequest<CareersApplication[]>("/careers/admin/applications"),
      ]);

      if (ovRes.status === "fulfilled") setOverview(ovRes.value);
      if (empRes.status === "fulfilled") setEmployers(Array.isArray(empRes.value) ? empRes.value : []);
      if (jobRes.status === "fulfilled") setJobs(Array.isArray(jobRes.value) ? jobRes.value : []);
      if (appRes.status === "fulfilled") setApplications(Array.isArray(appRes.value) ? appRes.value : []);
    } catch (err: any) {
      setError(err.message || "Failed to load careers administrative data");
    } finally {
      setLoading(false);
    }
  };

  const handleVerifySubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!verifyingEmployer) return;

    setSubmittingVerify(true);
    try {
      await apiRequest(`/careers/admin/employers/${verifyingEmployer.id}/verify`, {
        method: "POST",
        body: JSON.stringify({
          approved: verifyApproved,
          reason: verifyReason.trim(),
        }),
      });

      setVerifyingEmployer(null);
      setVerifyReason("");
      setSuccess(
        `Employer organization ${verifyingEmployer.organization_name} ${
          verifyApproved ? "verified successfully" : "rejected"
        }.`
      );
      loadAllAdminData();
    } catch (err: any) {
      setError(err.message || "Failed to verify employer");
    } finally {
      setSubmittingVerify(false);
    }
  };

  const handleModerateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!moderatingJob) return;

    setSubmittingMod(true);
    try {
      await apiRequest(`/careers/admin/jobs/${moderatingJob.id}/moderate`, {
        method: "POST",
        body: JSON.stringify({
          status: modStatus,
          notes: modNotes.trim(),
        }),
      });

      setModeratingJob(null);
      setModNotes("");
      setSuccess(`Job vacancy status updated to ${modStatus}.`);
      loadAllAdminData();
    } catch (err: any) {
      setError(err.message || "Failed to moderate job vacancy");
    } finally {
      setSubmittingMod(false);
    }
  };

  if (authLoading || loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center py-20">
        <Spinner text="Loading Careers Administration Workspace..." />
      </div>
    );
  }

  const filteredEmployers = empStatusFilter
    ? employers.filter((e) => e.verification_status === empStatusFilter)
    : employers;

  const filteredJobs = jobStatusFilter
    ? jobs.filter((j) => j.status === jobStatusFilter)
    : jobs;

  const filteredApps = appStatusFilter
    ? applications.filter((a) => a.status === appStatusFilter)
    : applications;

  return (
    <div className="min-h-screen bg-gray-50 pb-20">
      {/* Top Banner */}
      <section className="bg-emerald-950 text-white py-10 px-4 sm:px-6 lg:px-8 border-b border-emerald-900">
        <div className="max-w-7xl mx-auto flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-emerald-800 text-emerald-200 uppercase tracking-wider">
                Recruitment & Jobs Oversight
              </span>
              <span className="text-xs text-emerald-300">• Blueprint v2.0 Section 09</span>
            </div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
              Ratels Careers Administration
            </h1>
            <p className="text-emerald-100 text-xs sm:text-sm mt-1">
              Verify employer organizations, moderate vacancies, and audit platform recruitment integrity.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={loadAllAdminData}
              className="px-3.5 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-700 text-white font-medium text-xs transition flex items-center gap-1.5 border border-emerald-700"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              Refresh Data
            </button>
            <Link
              href="/admin"
              className="px-3.5 py-2 rounded-lg bg-white/10 hover:bg-white/20 text-white font-medium text-xs transition border border-emerald-800"
            >
              Admin Dashboard
            </Link>
          </div>
        </div>
      </section>

      {/* Tabs Navigation */}
      <div className="bg-white border-b border-gray-200 sticky top-16 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex space-x-8">
          <button
            onClick={() => setActiveTab("overview")}
            className={`py-4 text-xs sm:text-sm font-semibold border-b-2 transition ${
              activeTab === "overview"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Overview & Metrics
          </button>
          <button
            onClick={() => setActiveTab("employers")}
            className={`py-4 text-xs sm:text-sm font-semibold border-b-2 transition ${
              activeTab === "employers"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Employer Verification ({employers.filter((e) => e.verification_status === "pending").length} Pending)
          </button>
          <button
            onClick={() => setActiveTab("jobs")}
            className={`py-4 text-xs sm:text-sm font-semibold border-b-2 transition ${
              activeTab === "jobs"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Job Moderation ({jobs.length})
          </button>
          <button
            onClick={() => setActiveTab("applications")}
            className={`py-4 text-xs sm:text-sm font-semibold border-b-2 transition ${
              activeTab === "applications"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Applications Audit ({applications.length})
          </button>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-8 space-y-6">
        {error && (
          <Alert type="error" title="Notice">
            {error}
          </Alert>
        )}
        {success && (
          <Alert type="success" title="Success">
            {success}
          </Alert>
        )}

        {/* Tab 1: Overview & Metrics */}
        {activeTab === "overview" && (
          <div className="space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-3">
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-gray-500 font-semibold uppercase">Employers</span>
                <p className="text-xl font-extrabold text-gray-900 mt-1">{overview?.total_employers ?? employers.length}</p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-emerald-700 font-semibold uppercase">Verified</span>
                <p className="text-xl font-extrabold text-emerald-700 mt-1">{overview?.verified_employers ?? 0}</p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-amber-700 font-semibold uppercase">Pending</span>
                <p className="text-xl font-extrabold text-amber-600 mt-1">{overview?.pending_employers ?? 0}</p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-gray-500 font-semibold uppercase">Total Jobs</span>
                <p className="text-xl font-extrabold text-gray-900 mt-1">{overview?.total_jobs ?? jobs.length}</p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-emerald-700 font-semibold uppercase">Active Jobs</span>
                <p className="text-xl font-extrabold text-emerald-800 mt-1">{overview?.active_jobs ?? 0}</p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-sky-700 font-semibold uppercase">Moderation</span>
                <p className="text-xl font-extrabold text-sky-700 mt-1">{overview?.pending_approval_jobs ?? 0}</p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-gray-500 font-semibold uppercase">Applications</span>
                <p className="text-xl font-extrabold text-gray-900 mt-1">{overview?.total_applications ?? applications.length}</p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-[11px] text-emerald-800 font-semibold uppercase">Total Hires</span>
                <p className="text-xl font-extrabold text-emerald-800 mt-1">{overview?.total_hires ?? 0}</p>
              </div>
            </div>

            {/* Quick Summary Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm space-y-3">
                <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                  <Building2 className="w-4 h-4 text-emerald-700" />
                  Pending Employer Verifications
                </h3>
                {employers.filter((e) => e.verification_status === "pending").length === 0 ? (
                  <p className="text-xs text-gray-500 italic">No employer organizations currently awaiting verification.</p>
                ) : (
                  <div className="space-y-2">
                    {employers
                      .filter((e) => e.verification_status === "pending")
                      .slice(0, 3)
                      .map((emp) => (
                        <div key={emp.id} className="p-3 bg-gray-50 rounded-lg flex items-center justify-between text-xs">
                          <div>
                            <span className="font-bold text-gray-900 block">{emp.organization_name}</span>
                            <span className="text-gray-500">RC: {emp.registration_number}</span>
                          </div>
                          <button
                            type="button"
                            onClick={() => {
                              setVerifyingEmployer(emp);
                              setVerifyApproved(true);
                              setVerifyReason("");
                            }}
                            className="px-2.5 py-1 bg-emerald-800 text-white rounded font-medium hover:bg-emerald-900 transition"
                          >
                            Review
                          </button>
                        </div>
                      ))}
                  </div>
                )}
              </div>

              <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm space-y-3">
                <h3 className="font-bold text-gray-900 text-sm flex items-center gap-2">
                  <Briefcase className="w-4 h-4 text-emerald-700" />
                  Vacancies Moderation Queue
                </h3>
                {jobs.filter((j) => j.status === "pending_approval").length === 0 ? (
                  <p className="text-xs text-gray-500 italic">All published vacancies are currently moderated and approved.</p>
                ) : (
                  <div className="space-y-2">
                    {jobs
                      .filter((j) => j.status === "pending_approval")
                      .slice(0, 3)
                      .map((jb) => (
                        <div key={jb.id} className="p-3 bg-gray-50 rounded-lg flex items-center justify-between text-xs">
                          <div>
                            <span className="font-bold text-gray-900 block">{jb.title}</span>
                            <span className="text-gray-500">{jb.employer_name}</span>
                          </div>
                          <button
                            type="button"
                            onClick={() => {
                              setModeratingJob(jb);
                              setModStatus("published");
                              setModNotes("");
                            }}
                            className="px-2.5 py-1 bg-emerald-800 text-white rounded font-medium hover:bg-emerald-900 transition"
                          >
                            Moderate
                          </button>
                        </div>
                      ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Tab 2: Employers */}
        {activeTab === "employers" && (
          <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-gray-100">
              <h3 className="font-bold text-gray-900 text-base">Employer Organizations</h3>
              <div className="flex items-center gap-2">
                <select
                  value={empStatusFilter}
                  onChange={(e) => setEmpStatusFilter(e.target.value)}
                  className="text-xs p-2 rounded-lg border border-gray-300 bg-gray-50"
                >
                  <option value="">All Statuses</option>
                  <option value="pending">Pending</option>
                  <option value="verified">Verified</option>
                  <option value="rejected">Rejected</option>
                  <option value="suspended">Suspended</option>
                </select>
              </div>
            </div>

            {filteredEmployers.length === 0 ? (
              <EmptyState title="No Employers Found" description="No employer organizations match the current filter." />
            ) : (
              <div className="divide-y divide-gray-100">
                {filteredEmployers.map((emp) => (
                  <div key={emp.id} className="py-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4 text-xs">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <Badge
                          variant={
                            emp.verification_status === "verified"
                              ? "success"
                              : emp.verification_status === "rejected"
                              ? "danger"
                              : "warning"
                          }
                        >
                          {emp.verification_status.toUpperCase()}
                        </Badge>
                        <span className="text-gray-500 font-medium">RC: {emp.registration_number}</span>
                      </div>
                      <h4 className="font-bold text-gray-900 text-sm sm:text-base">{emp.organization_name}</h4>
                      <p className="text-gray-500">
                        {emp.industry} • Contact: {emp.contact_email} • {emp.contact_phone}
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => {
                          setVerifyingEmployer(emp);
                          setVerifyApproved(emp.verification_status !== "verified");
                          setVerifyReason("");
                        }}
                        className="px-3 py-1.5 rounded-lg border border-gray-200 bg-gray-50 hover:bg-gray-100 font-medium text-gray-700 transition"
                      >
                        {emp.verification_status === "verified" ? "Manage Verification" : "Review & Verify"}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Tab 3: Jobs Moderation */}
        {activeTab === "jobs" && (
          <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-gray-100">
              <h3 className="font-bold text-gray-900 text-base">Job Vacancies</h3>
              <select
                value={jobStatusFilter}
                onChange={(e) => setJobStatusFilter(e.target.value)}
                className="text-xs p-2 rounded-lg border border-gray-300 bg-gray-50"
              >
                <option value="">All Statuses</option>
                <option value="pending_approval">Pending Approval</option>
                <option value="published">Published</option>
                <option value="paused">Paused</option>
                <option value="closed">Closed</option>
                <option value="rejected">Rejected</option>
              </select>
            </div>

            {filteredJobs.length === 0 ? (
              <EmptyState title="No Jobs Found" description="No job vacancies match the selected criteria." />
            ) : (
              <div className="divide-y divide-gray-100">
                {filteredJobs.map((j) => (
                  <div key={j.id} className="py-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4 text-xs">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <Badge
                          variant={
                            j.status === "published"
                              ? "success"
                              : j.status === "pending_approval"
                              ? "warning"
                              : "neutral"
                          }
                        >
                          {j.status.toUpperCase()}
                        </Badge>
                        <span className="text-gray-500 font-medium">{j.category}</span>
                      </div>
                      <h4 className="font-bold text-gray-900 text-sm sm:text-base">{j.title}</h4>
                      <p className="text-gray-500">
                        Employer: <span className="font-semibold text-gray-800">{j.employer_name}</span> •{" "}
                        {j.location_summary || "Nigeria"} • Views: {j.views_count} • Applications: {j.applications_count}
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => {
                          setModeratingJob(j);
                          setModStatus(j.status === "published" ? "paused" : "published");
                          setModNotes("");
                        }}
                        className="px-3 py-1.5 rounded-lg border border-gray-200 bg-gray-50 hover:bg-gray-100 font-medium text-gray-700 transition"
                      >
                        Moderate Status
                      </button>
                      <Link
                        href={`/careers/jobs/${j.id}`}
                        target="_blank"
                        className="px-3 py-1.5 rounded-lg bg-emerald-800 text-white font-medium hover:bg-emerald-900 transition flex items-center gap-1"
                      >
                        Preview
                        <ExternalLink className="w-3 h-3" />
                      </Link>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Tab 4: Applications Audit */}
        {activeTab === "applications" && (
          <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-gray-100">
              <h3 className="font-bold text-gray-900 text-base">Recruitment Audit Trail</h3>
              <select
                value={appStatusFilter}
                onChange={(e) => setAppStatusFilter(e.target.value)}
                className="text-xs p-2 rounded-lg border border-gray-300 bg-gray-50"
              >
                <option value="">All Statuses</option>
                <option value="submitted">Submitted</option>
                <option value="under_review">Under Review</option>
                <option value="shortlisted">Shortlisted</option>
                <option value="interview">Interview</option>
                <option value="offered">Offered</option>
                <option value="hired">Hired</option>
                <option value="rejected">Rejected</option>
                <option value="withdrawn">Withdrawn</option>
              </select>
            </div>

            {filteredApps.length === 0 ? (
              <EmptyState title="No Applications Found" description="No platform applications match the current filter." />
            ) : (
              <div className="divide-y divide-gray-100">
                {filteredApps.map((a) => (
                  <div key={a.id} className="py-3.5 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
                    <div>
                      <div className="flex items-center gap-2 mb-1">
                        <Badge variant="primary">{a.status.toUpperCase()}</Badge>
                        {a.match_score > 0 && (
                          <span className="font-semibold text-emerald-800 flex items-center gap-1">
                            <Sparkles className="w-3 h-3 text-emerald-600" />
                            {a.match_score}% Match
                          </span>
                        )}
                      </div>
                      <h4 className="font-bold text-gray-900">{a.job_title}</h4>
                      <p className="text-gray-500">
                        Employer: <span className="font-medium text-gray-700">{a.employer_name}</span> • Candidate:{" "}
                        <span className="font-medium text-gray-700">{a.candidate_name}</span>
                      </p>
                    </div>

                    <div className="text-right text-gray-400">
                      <span>{new Date(a.created_at).toLocaleDateString("en-NG")}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Verification Modal */}
      {verifyingEmployer && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-2xl border border-gray-200">
            <h3 className="text-lg font-bold text-gray-900 mb-1">Verify Employer Organization</h3>
            <p className="text-xs text-gray-600 mb-4">
              Organization: <strong>{verifyingEmployer.organization_name}</strong> (RC:{" "}
              {verifyingEmployer.registration_number})
            </p>

            <form onSubmit={handleVerifySubmit} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">Decision</label>
                <div className="flex gap-4">
                  <label className="flex items-center gap-1.5 cursor-pointer font-medium text-emerald-800">
                    <input
                      type="radio"
                      name="decision"
                      checked={verifyApproved}
                      onChange={() => setVerifyApproved(true)}
                    />
                    Approve & Verify
                  </label>
                  <label className="flex items-center gap-1.5 cursor-pointer font-medium text-rose-800">
                    <input
                      type="radio"
                      name="decision"
                      checked={!verifyApproved}
                      onChange={() => setVerifyApproved(false)}
                    />
                    Reject / Suspend
                  </label>
                </div>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">
                  Audit Notes / Reason
                </label>
                <textarea
                  rows={3}
                  value={verifyReason}
                  onChange={(e) => setVerifyReason(e.target.value)}
                  placeholder="Notes explaining verification confirmation or deficiency findings..."
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setVerifyingEmployer(null)}
                  className="px-4 py-2 text-gray-700 font-medium hover:text-gray-900"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingVerify}
                  className="px-4 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium disabled:opacity-50"
                >
                  {submittingVerify ? "Saving..." : "Save Verification"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Moderation Modal */}
      {moderatingJob && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-2xl border border-gray-200">
            <h3 className="text-lg font-bold text-gray-900 mb-1">Moderate Job Vacancy</h3>
            <p className="text-xs text-gray-600 mb-4">
              Vacancy: <strong>{moderatingJob.title}</strong>
            </p>

            <form onSubmit={handleModerateSubmit} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">Status</label>
                <select
                  value={modStatus}
                  onChange={(e) => setModStatus(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                >
                  <option value="published">Published (Active)</option>
                  <option value="pending_approval">Pending Approval</option>
                  <option value="paused">Paused</option>
                  <option value="closed">Closed</option>
                  <option value="rejected">Rejected</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">Moderator Notes</label>
                <textarea
                  rows={3}
                  value={modNotes}
                  onChange={(e) => setModNotes(e.target.value)}
                  placeholder="Audit reason for status change or moderation feedback..."
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setModeratingJob(null)}
                  className="px-4 py-2 text-gray-700 font-medium hover:text-gray-900"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submittingMod}
                  className="px-4 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium disabled:opacity-50"
                >
                  {submittingMod ? "Saving..." : "Save Moderation"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
