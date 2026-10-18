"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Building2,
  Briefcase,
  Users,
  Calendar,
  CheckCircle2,
  Clock,
  AlertCircle,
  Plus,
  Send,
  Sparkles,
  ChevronRight,
  ShieldCheck,
  ExternalLink,
  MapPin,
  X,
  UserCheck,
  Phone,
  Mail,
  Video,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import {
  CareersEmployer,
  CareersEmployerOverview,
  CareersJob,
  CareersApplication,
  CareersInterview,
  State,
} from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";

const JOB_CATEGORIES = [
  { id: "sanitation_environment", label: "Sanitation & Environment" },
  { id: "logistics_supply_chain", label: "Logistics & Supply Chain" },
  { id: "safety_security", label: "Health, Safety & PPE" },
  { id: "operations_management", label: "Operations & Management" },
  { id: "community_advocacy", label: "Community & Advocacy" },
  { id: "administration_hr", label: "Administration & HR" },
  { id: "technology_data", label: "Technology & Data" },
];

export default function EmployerPortalPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<"overview" | "jobs" | "applicants" | "interviews">("overview");
  const [employer, setEmployer] = useState<CareersEmployer | null>(null);
  const [overview, setOverview] = useState<CareersEmployerOverview | null>(null);
  const [jobs, setJobs] = useState<CareersJob[]>([]);
  const [interviews, setInterviews] = useState<CareersInterview[]>([]);

  // Selected Job for Applicants Tab
  const [selectedJobId, setSelectedJobId] = useState<string>("");
  const [applicants, setApplicants] = useState<CareersApplication[]>([]);
  const [applicantsLoading, setApplicantsLoading] = useState(false);

  // Status & Notifications
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // States
  const [states, setStates] = useState<State[]>([]);

  // Register Employer Form State
  const [regOrgName, setRegOrgName] = useState("");
  const [regNumber, setRegNumber] = useState("");
  const [regIndustry, setRegIndustry] = useState("Environmental & Sanitation Services");
  const [regDescription, setRegDescription] = useState("");
  const [regWebsite, setRegWebsite] = useState("");
  const [regEmail, setRegEmail] = useState("");
  const [regPhone, setRegPhone] = useState("");
  const [regStateId, setRegStateId] = useState("");
  const [regAddress, setRegAddress] = useState("");
  const [regSubmitting, setRegSubmitting] = useState(false);

  // Create Job Modal State
  const [showJobModal, setShowJobModal] = useState(false);
  const [jobTitle, setJobTitle] = useState("");
  const [jobCategory, setJobCategory] = useState("sanitation_environment");
  const [jobEmploymentType, setJobEmploymentType] = useState<any>("full_time");
  const [jobWorkplaceType, setJobWorkplaceType] = useState<any>("on_site");
  const [jobLocation, setJobLocation] = useState("");
  const [jobSalaryMinNaira, setJobSalaryMinNaira] = useState("");
  const [jobSalaryMaxNaira, setJobSalaryMaxNaira] = useState("");
  const [jobNegotiable, setJobNegotiable] = useState(false);
  const [jobDescription, setJobDescription] = useState("");
  const [jobResponsibilities, setJobResponsibilities] = useState("");
  const [jobRequirements, setJobRequirements] = useState("");
  const [jobSkillsText, setJobSkillsText] = useState("");
  const [jobExperienceLevel, setJobExperienceLevel] = useState<any>("entry");
  const [jobDeadline, setJobDeadline] = useState("");
  const [jobSubmitting, setJobSubmitting] = useState(false);

  // Status Change Modal State for Applications
  const [statusApp, setStatusApp] = useState<CareersApplication | null>(null);
  const [targetStatus, setTargetStatus] = useState<string>("under_review");
  const [statusReason, setStatusReason] = useState("");
  const [statusSubmitting, setStatusSubmitting] = useState(false);

  // Schedule Interview Modal State
  const [interviewApp, setInterviewApp] = useState<CareersApplication | null>(null);
  const [intTime, setIntTime] = useState("");
  const [intFormat, setIntFormat] = useState<"video" | "in_person" | "phone">("video");
  const [intLocation, setIntLocation] = useState("");
  const [intInstructions, setIntInstructions] = useState("");
  const [intSubmitting, setIntSubmitting] = useState(false);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login?redirect=/careers/employer");
      return;
    }

    if (user) {
      fetchStates();
      loadEmployerData();
    }
  }, [user, authLoading]);

  const fetchStates = async () => {
    try {
      const data = await apiRequest<State[]>("/geography/states");
      setStates(Array.isArray(data) ? data : []);
      if (Array.isArray(data) && data.length > 0 && !regStateId) {
        setRegStateId(data[0].id);
      }
    } catch (_) {}
  };

  const loadEmployerData = async () => {
    setLoading(true);
    setError(null);
    try {
      const emp = await apiRequest<CareersEmployer>("/careers/employer/me");
      setEmployer(emp);

      if (emp && emp.id) {
        // Load overview, jobs, and interviews
        const [ovRes, jobsRes, intsRes] = await Promise.allSettled([
          apiRequest<CareersEmployerOverview>(`/careers/employer/${emp.id}/overview`),
          apiRequest<CareersJob[]>(`/careers/employer/${emp.id}/jobs`),
          apiRequest<CareersInterview[]>(`/careers/employer/${emp.id}/interviews`),
        ]);

        if (ovRes.status === "fulfilled") setOverview(ovRes.value);
        if (jobsRes.status === "fulfilled") {
          const list = Array.isArray(jobsRes.value) ? jobsRes.value : [];
          setJobs(list);
          if (list.length > 0 && !selectedJobId) {
            setSelectedJobId(list[0].id);
          }
        }
        if (intsRes.status === "fulfilled") {
          setInterviews(Array.isArray(intsRes.value) ? intsRes.value : []);
        }
      }
    } catch (err: any) {
      if (err.status === 404 || err.code === "NOT_FOUND") {
        setEmployer(null);
      } else {
        setError(err.message || "Failed to load employer records");
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (employer && selectedJobId && activeTab === "applicants") {
      loadApplicants(selectedJobId);
    }
  }, [selectedJobId, activeTab, employer]);

  const loadApplicants = async (jobId: string) => {
    setApplicantsLoading(true);
    try {
      const res = await apiRequest<CareersApplication[]>(
        `/careers/employer/jobs/${jobId}/applications`
      );
      setApplicants(Array.isArray(res) ? res : []);
    } catch (err: any) {
      setError(err.message || "Failed to load job applicants");
    } finally {
      setApplicantsLoading(false);
    }
  };

  const handleRegisterEmployer = async (e: React.FormEvent) => {
    e.preventDefault();
    setRegSubmitting(true);
    setError(null);
    setSuccess(null);

    try {
      const created = await apiRequest<CareersEmployer>("/careers/employer/register", {
        method: "POST",
        body: JSON.stringify({
          organization_name: regOrgName.trim(),
          registration_number: regNumber.trim(),
          industry: regIndustry.trim(),
          description: regDescription.trim(),
          website_url: regWebsite.trim(),
          contact_email: regEmail.trim(),
          contact_phone: regPhone.trim(),
          state_id: regStateId,
          address: regAddress.trim(),
        }),
      });

      setEmployer(created);
      setSuccess("Your employer organization has been registered and submitted for review.");
      loadEmployerData();
    } catch (err: any) {
      setError(err.message || "Failed to register employer organization");
    } finally {
      setRegSubmitting(false);
    }
  };

  const handleCreateJob = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!employer) return;

    setJobSubmitting(true);
    setError(null);

    const skills = jobSkillsText
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    const minKobo = jobSalaryMinNaira ? Math.round(parseFloat(jobSalaryMinNaira) * 100) : undefined;
    const maxKobo = jobSalaryMaxNaira ? Math.round(parseFloat(jobSalaryMaxNaira) * 100) : undefined;

    try {
      await apiRequest(`/careers/employer/${employer.id}/jobs`, {
        method: "POST",
        body: JSON.stringify({
          title: jobTitle.trim(),
          category: jobCategory,
          employment_type: jobEmploymentType,
          workplace_type: jobWorkplaceType,
          location_summary: jobLocation.trim(),
          salary_min_kobo: minKobo,
          salary_max_kobo: maxKobo,
          is_salary_negotiable: jobNegotiable,
          description: jobDescription.trim(),
          responsibilities: jobResponsibilities.trim(),
          requirements: jobRequirements.trim(),
          skills_required: skills,
          experience_level: jobExperienceLevel,
          deadline_at: jobDeadline ? new Date(jobDeadline).toISOString() : undefined,
          status: "published",
        }),
      });

      setShowJobModal(false);
      resetJobForm();
      setSuccess("New job vacancy has been posted successfully.");
      loadEmployerData();
    } catch (err: any) {
      setError(err.message || "Failed to create job vacancy");
    } finally {
      setJobSubmitting(false);
    }
  };

  const resetJobForm = () => {
    setJobTitle("");
    setJobLocation("");
    setJobSalaryMinNaira("");
    setJobSalaryMaxNaira("");
    setJobNegotiable(false);
    setJobDescription("");
    setJobResponsibilities("");
    setJobRequirements("");
    setJobSkillsText("");
    setJobDeadline("");
  };

  const handleUpdateJobStatus = async (jobId: string, status: string) => {
    try {
      await apiRequest(`/careers/employer/jobs/${jobId}/status`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
      });
      setSuccess(`Job status updated to ${status}`);
      loadEmployerData();
    } catch (err: any) {
      setError(err.message || "Failed to update job status");
    }
  };

  const handleUpdateAppStatus = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!statusApp) return;

    setStatusSubmitting(true);
    try {
      await apiRequest(`/careers/employer/applications/${statusApp.id}/status`, {
        method: "PATCH",
        body: JSON.stringify({
          status: targetStatus,
          reason: statusReason.trim(),
        }),
      });

      setStatusApp(null);
      setStatusReason("");
      setSuccess(`Application marked as ${targetStatus}`);
      if (selectedJobId) loadApplicants(selectedJobId);
      loadEmployerData();
    } catch (err: any) {
      setError(err.message || "Failed to update application status");
    } finally {
      setStatusSubmitting(false);
    }
  };

  const handleScheduleInterview = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!interviewApp) return;

    setIntSubmitting(true);
    try {
      await apiRequest(`/careers/employer/applications/${interviewApp.id}/interviews`, {
        method: "POST",
        body: JSON.stringify({
          scheduled_time: new Date(intTime).toISOString(),
          format: intFormat,
          location_or_link: intLocation.trim(),
          instructions: intInstructions.trim(),
        }),
      });

      setInterviewApp(null);
      setIntTime("");
      setIntLocation("");
      setIntInstructions("");
      setSuccess("Interview appointment successfully scheduled and notification sent to candidate.");
      if (selectedJobId) loadApplicants(selectedJobId);
      loadEmployerData();
    } catch (err: any) {
      setError(err.message || "Failed to schedule interview");
    } finally {
      setIntSubmitting(false);
    }
  };

  if (authLoading || loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center py-20">
        <Spinner text="Loading employer portal..." />
      </div>
    );
  }

  // Not Registered Employer State
  if (!employer) {
    return (
      <div className="min-h-screen bg-gray-50 pb-20">
        <section className="bg-emerald-900 text-white py-12 px-4 sm:px-6 lg:px-8 border-b border-emerald-800">
          <div className="max-w-4xl mx-auto">
            <h1 className="text-3xl font-extrabold tracking-tight">Register Employer Organization</h1>
            <p className="text-emerald-100 text-sm mt-1">
              Connect your verified organization to recruit qualified personnel for environmental
              sanitation, logistics, and community development projects.
            </p>
          </div>
        </section>

        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 pt-8">
          {error && (
            <div className="mb-6">
              <Alert type="error" title="Registration Error">
                {error}
              </Alert>
            </div>
          )}

          <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm">
            <form onSubmit={handleRegisterEmployer} className="space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    Organization / Company Name *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. Clean City Environmental Services Ltd"
                    value={regOrgName}
                    onChange={(e) => setRegOrgName(e.target.value)}
                    className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    CAC Registration Number *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. RC-1849202"
                    value={regNumber}
                    onChange={(e) => setRegNumber(e.target.value)}
                    className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    Primary Industry *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. Environmental Waste Management & Logistics"
                    value={regIndustry}
                    onChange={(e) => setRegIndustry(e.target.value)}
                    className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    Website URL
                  </label>
                  <input
                    type="text"
                    placeholder="e.g. https://cleancity.ng"
                    value={regWebsite}
                    onChange={(e) => setRegWebsite(e.target.value)}
                    className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                  Organization Description
                </label>
                <textarea
                  rows={3}
                  placeholder="Describe your organization's mission, operational scope, and projects..."
                  value={regDescription}
                  onChange={(e) => setRegDescription(e.target.value)}
                  className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    Contact Email *
                  </label>
                  <input
                    type="email"
                    required
                    placeholder="careers@cleancity.ng"
                    value={regEmail}
                    onChange={(e) => setRegEmail(e.target.value)}
                    className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    Contact Phone *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="+234 803 000 0000"
                    value={regPhone}
                    onChange={(e) => setRegPhone(e.target.value)}
                    className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    State Location *
                  </label>
                  <select
                    value={regStateId}
                    onChange={(e) => setRegStateId(e.target.value)}
                    className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                  >
                    {states.map((st) => (
                      <option key={st.id} value={st.id}>
                        {st.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                  Office Address
                </label>
                <input
                  type="text"
                  placeholder="e.g. Plot 14, Commercial Avenue, Victoria Island, Lagos"
                  value={regAddress}
                  onChange={(e) => setRegAddress(e.target.value)}
                  className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="pt-4 flex justify-end">
                <button
                  type="submit"
                  disabled={regSubmitting}
                  className="px-6 py-2.5 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium text-sm transition shadow-sm flex items-center gap-2 disabled:opacity-50"
                >
                  {regSubmitting ? "Submitting Registration..." : "Register Organization"}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    );
  }

  // Registered Employer Portal
  return (
    <div className="min-h-screen bg-gray-50 pb-20">
      {/* Top Banner */}
      <section className="bg-emerald-900 text-white py-10 px-4 sm:px-6 lg:px-8 border-b border-emerald-800">
        <div className="max-w-7xl mx-auto flex flex-col md:flex-row md:items-center justify-between gap-6">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <Badge
                variant={
                  employer.verification_status === "verified"
                    ? "success"
                    : employer.verification_status === "rejected"
                    ? "danger"
                    : "warning"
                }
              >
                {employer.verification_status.toUpperCase()}
              </Badge>
              <span className="text-xs text-emerald-200">RC: {employer.registration_number}</span>
            </div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
              {employer.organization_name}
            </h1>
            <p className="text-emerald-100 text-xs sm:text-sm mt-1">
              Recruitment Workspace • Ratels Careers Module
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => setShowJobModal(true)}
              className="px-4 py-2.5 rounded-lg bg-emerald-700 hover:bg-emerald-600 text-white font-medium text-xs sm:text-sm transition shadow-sm flex items-center gap-2 border border-emerald-600"
            >
              <Plus className="w-4 h-4" />
              Post Job Vacancy
            </button>
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
            Overview
          </button>
          <button
            onClick={() => setActiveTab("jobs")}
            className={`py-4 text-xs sm:text-sm font-semibold border-b-2 transition ${
              activeTab === "jobs"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Job Vacancies ({jobs.length})
          </button>
          <button
            onClick={() => setActiveTab("applicants")}
            className={`py-4 text-xs sm:text-sm font-semibold border-b-2 transition ${
              activeTab === "applicants"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Review Applicants
          </button>
          <button
            onClick={() => setActiveTab("interviews")}
            className={`py-4 text-xs sm:text-sm font-semibold border-b-2 transition ${
              activeTab === "interviews"
                ? "border-emerald-800 text-emerald-900"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Interviews ({interviews.length})
          </button>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-8 space-y-6">
        {/* Verification Status Warning if not verified */}
        {employer.verification_status !== "verified" && (
          <Alert type="warning" title="Verification Pending">
            Your organization is currently undergoing administrative review. Once verified by the
            national recruitment desk, your published positions will immediately appear on the public
            discovery portal.
          </Alert>
        )}

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

        {/* Tab 1: Overview */}
        {activeTab === "overview" && (
          <div className="space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-xs text-gray-500 uppercase font-semibold">Total Jobs</span>
                <p className="text-2xl font-extrabold text-gray-900 mt-1">
                  {overview?.total_jobs ?? jobs.length}
                </p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-xs text-gray-500 uppercase font-semibold">Active Jobs</span>
                <p className="text-2xl font-extrabold text-emerald-800 mt-1">
                  {overview?.active_jobs ?? jobs.filter((j) => j.status === "published").length}
                </p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-xs text-gray-500 uppercase font-semibold">Applications</span>
                <p className="text-2xl font-extrabold text-gray-900 mt-1">
                  {overview?.total_applications ?? 0}
                </p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-xs text-gray-500 uppercase font-semibold">Under Review</span>
                <p className="text-2xl font-extrabold text-amber-600 mt-1">
                  {overview?.pending_applications ?? 0}
                </p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-xs text-gray-500 uppercase font-semibold">Shortlisted</span>
                <p className="text-2xl font-extrabold text-purple-700 mt-1">
                  {overview?.shortlisted_applications ?? 0}
                </p>
              </div>
              <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-center">
                <span className="text-xs text-gray-500 uppercase font-semibold">Hired</span>
                <p className="text-2xl font-extrabold text-emerald-700 mt-1">
                  {overview?.hired_applications ?? 0}
                </p>
              </div>
            </div>

            <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm">
              <h3 className="font-bold text-gray-900 mb-2">Organization Overview</h3>
              <p className="text-xs text-gray-600 leading-relaxed max-w-3xl">
                {employer.description || "No description provided."}
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 pt-4 mt-4 border-t border-gray-100 text-xs text-gray-600">
                <div>
                  <span className="font-semibold text-gray-800 block">Industry:</span>
                  {employer.industry}
                </div>
                <div>
                  <span className="font-semibold text-gray-800 block">Official Email:</span>
                  {employer.contact_email}
                </div>
                <div>
                  <span className="font-semibold text-gray-800 block">Phone:</span>
                  {employer.contact_phone}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 2: Jobs Management */}
        {activeTab === "jobs" && (
          <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-4 border-b border-gray-100">
              <h3 className="font-bold text-gray-900 text-base">Your Job Vacancies</h3>
              <button
                type="button"
                onClick={() => setShowJobModal(true)}
                className="px-3 py-1.5 rounded-lg bg-emerald-800 text-white font-medium text-xs hover:bg-emerald-900 transition flex items-center gap-1"
              >
                <Plus className="w-3.5 h-3.5" />
                New Vacancy
              </button>
            </div>

            {jobs.length === 0 ? (
              <EmptyState
                icon={<Briefcase className="w-8 h-8 text-gray-400" />}
                title="No Vacancies Posted Yet"
                description="Create your first job listing to start receiving qualified candidates."
                actionText="Create Vacancy"
                onAction={() => setShowJobModal(true)}
              />
            ) : (
              <div className="divide-y divide-gray-100">
                {jobs.map((j) => (
                  <div key={j.id} className="py-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <Badge
                          variant={
                            j.status === "published"
                              ? "success"
                              : j.status === "pending_approval"
                              ? "warning"
                              : j.status === "paused"
                              ? "info"
                              : "neutral"
                          }
                        >
                          {j.status.toUpperCase()}
                        </Badge>
                        <span className="text-xs text-gray-500 font-medium">{j.category}</span>
                      </div>
                      <h4 className="font-bold text-gray-900 text-sm sm:text-base">{j.title}</h4>
                      <p className="text-xs text-gray-500">
                        {j.location_summary || "Nigeria"} • Applications:{" "}
                        <strong>{j.applications_count || 0}</strong>
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => {
                          setSelectedJobId(j.id);
                          setActiveTab("applicants");
                        }}
                        className="px-3 py-1.5 rounded-lg border border-gray-200 bg-gray-50 hover:bg-gray-100 text-xs font-medium text-gray-700 transition"
                      >
                        View Applicants ({j.applications_count || 0})
                      </button>

                      {j.status === "published" ? (
                        <button
                          type="button"
                          onClick={() => handleUpdateJobStatus(j.id, "paused")}
                          className="px-3 py-1.5 rounded-lg border border-amber-200 bg-amber-50 hover:bg-amber-100 text-xs font-medium text-amber-800 transition"
                        >
                          Pause
                        </button>
                      ) : j.status === "paused" ? (
                        <button
                          type="button"
                          onClick={() => handleUpdateJobStatus(j.id, "published")}
                          className="px-3 py-1.5 rounded-lg border border-emerald-200 bg-emerald-50 hover:bg-emerald-100 text-xs font-medium text-emerald-800 transition"
                        >
                          Publish
                        </button>
                      ) : null}

                      {j.status !== "closed" && (
                        <button
                          type="button"
                          onClick={() => handleUpdateJobStatus(j.id, "closed")}
                          className="px-3 py-1.5 rounded-lg border border-rose-200 bg-rose-50 hover:bg-rose-100 text-xs font-medium text-rose-700 transition"
                        >
                          Close
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Tab 3: Applicants Review */}
        {activeTab === "applicants" && (
          <div className="space-y-6">
            {/* Job Selector Bar */}
            <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <label className="text-xs font-semibold text-gray-700 uppercase">Select Vacancy:</label>
                <select
                  value={selectedJobId}
                  onChange={(e) => setSelectedJobId(e.target.value)}
                  className="text-xs font-medium p-2 rounded-lg border border-gray-300 bg-gray-50 focus:ring-emerald-700 focus:border-emerald-700"
                >
                  {jobs.map((j) => (
                    <option key={j.id} value={j.id}>
                      {j.title} ({j.applications_count || 0} applicants)
                    </option>
                  ))}
                </select>
              </div>

              <div className="text-xs text-gray-500 font-medium">
                Showing {applicants.length} candidates
              </div>
            </div>

            {/* Applicants List */}
            {applicantsLoading ? (
              <div className="py-12 bg-white rounded-xl border border-gray-200">
                <Spinner text="Loading candidates..." />
              </div>
            ) : applicants.length === 0 ? (
              <div className="bg-white rounded-xl border border-gray-200 p-8">
                <EmptyState
                  icon={<Users className="w-8 h-8 text-gray-400" />}
                  title="No Applications Yet"
                  description="No candidates have applied for this position yet."
                />
              </div>
            ) : (
              <div className="space-y-4">
                {applicants.map((app) => (
                  <div
                    key={app.id}
                    className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4"
                  >
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                      <div>
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant="primary" size="sm">
                            {app.status.replace(/_/g, " ").toUpperCase()}
                          </Badge>
                          <span className="text-xs font-bold px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-800 border border-emerald-200 flex items-center gap-1">
                            <Sparkles className="w-3 h-3 text-emerald-600" />
                            {app.match_score}% Match
                          </span>
                        </div>
                        <h4 className="text-base font-bold text-gray-900">
                          {app.candidate_name || "Applicant"}
                        </h4>
                        <div className="flex flex-wrap items-center gap-3 text-xs text-gray-600 mt-1">
                          <span className="flex items-center gap-1">
                            <Mail className="w-3.5 h-3.5 text-gray-400" />
                            {app.candidate_email}
                          </span>
                          {app.candidate_phone && (
                            <span className="flex items-center gap-1">
                              <Phone className="w-3.5 h-3.5 text-gray-400" />
                              {app.candidate_phone}
                            </span>
                          )}
                          <span>• Applied {new Date(app.created_at).toLocaleDateString("en-NG")}</span>
                        </div>
                      </div>

                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          onClick={() => {
                            setStatusApp(app);
                            setTargetStatus(app.status);
                            setStatusReason("");
                          }}
                          className="px-3 py-1.5 rounded-lg border border-gray-200 bg-gray-50 hover:bg-gray-100 text-xs font-medium text-gray-700 transition"
                        >
                          Update Status
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setInterviewApp(app);
                            setIntTime("");
                            setIntLocation("");
                            setIntInstructions("");
                          }}
                          className="px-3 py-1.5 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white text-xs font-medium transition flex items-center gap-1"
                        >
                          <Calendar className="w-3.5 h-3.5" />
                          Schedule Interview
                        </button>
                      </div>
                    </div>

                    {/* Qualifications & Cover Note */}
                    <div className="bg-gray-50 p-4 rounded-xl border border-gray-100 space-y-2 text-xs text-gray-700">
                      <div>
                        <span className="font-semibold text-gray-900 block">
                          Qualifications Summary:
                        </span>
                        <p className="mt-0.5 whitespace-pre-line">{app.qualifications_summary}</p>
                      </div>
                      <div className="pt-2 border-t border-gray-200/60">
                        <span className="font-semibold text-gray-900 block">Statement / Motivation:</span>
                        <p className="mt-0.5 whitespace-pre-line">{app.cover_letter}</p>
                      </div>
                    </div>

                    {/* Resume & Explainable Breakdown */}
                    <div className="flex flex-wrap items-center justify-between gap-4 text-xs pt-1">
                      {app.resume_url ? (
                        <a
                          href={app.resume_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-emerald-800 hover:underline font-medium inline-flex items-center gap-1"
                        >
                          <ExternalLink className="w-3 h-3" />
                          View Résumé ({app.resume_filename || "Document"})
                        </a>
                      ) : (
                        <span className="text-gray-400">No CV attachment</span>
                      )}

                      {app.match_factors?.criteria_explanation && (
                        <span className="text-gray-500 italic">
                          {app.match_factors.criteria_explanation}
                        </span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Tab 4: Interviews */}
        {activeTab === "interviews" && (
          <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4">
            <h3 className="font-bold text-gray-900 text-base pb-3 border-b border-gray-100">
              Scheduled Candidate Interviews ({interviews.length})
            </h3>

            {interviews.length === 0 ? (
              <EmptyState
                icon={<Calendar className="w-8 h-8 text-gray-400" />}
                title="No Interviews Scheduled"
                description="Select an applicant from the Review Applicants tab to schedule an interview."
              />
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {interviews.map((item) => (
                  <div key={item.id} className="p-4 rounded-xl border border-gray-200 bg-gray-50 space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-bold text-gray-900">{item.candidate_name}</span>
                      <Badge variant="success" size="sm">
                        {item.format.toUpperCase()}
                      </Badge>
                    </div>
                    <p className="text-xs text-gray-600">{item.job_title}</p>
                    <div className="text-xs text-emerald-900 font-semibold flex items-center gap-1 pt-1">
                      <Clock className="w-3.5 h-3.5" />
                      {new Date(item.scheduled_time).toLocaleString("en-NG")}
                    </div>
                    {item.location_or_link && (
                      <p className="text-xs text-gray-600 break-all">
                        <strong>Venue / Link:</strong> {item.location_or_link}
                      </p>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Modal: Create Job Vacancy */}
      {showJobModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-3xl w-full p-6 sm:p-8 shadow-2xl border border-gray-200 my-8">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100 mb-6">
              <h3 className="text-xl font-bold text-gray-900">Post New Job Vacancy</h3>
              <button
                type="button"
                onClick={() => setShowJobModal(false)}
                className="text-gray-400 hover:text-gray-600 text-sm font-semibold p-1"
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleCreateJob} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">Job Title *</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Lead Environmental Health Officer"
                  value={jobTitle}
                  onChange={(e) => setJobTitle(e.target.value)}
                  className="w-full text-sm p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Category *</label>
                  <select
                    value={jobCategory}
                    onChange={(e) => setJobCategory(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  >
                    {JOB_CATEGORIES.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Type *</label>
                  <select
                    value={jobEmploymentType}
                    onChange={(e: any) => setJobEmploymentType(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  >
                    <option value="full_time">Full-time</option>
                    <option value="part_time">Part-time</option>
                    <option value="contract">Contract</option>
                    <option value="internship">Internship</option>
                    <option value="volunteer">Volunteer</option>
                  </select>
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Workplace *</label>
                  <select
                    value={jobWorkplaceType}
                    onChange={(e: any) => setJobWorkplaceType(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  >
                    <option value="on_site">On-site</option>
                    <option value="remote">Remote</option>
                    <option value="hybrid">Hybrid</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Location Summary</label>
                  <input
                    type="text"
                    placeholder="e.g. Ikeja, Lagos"
                    value={jobLocation}
                    onChange={(e) => setJobLocation(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Min Salary (₦/mo)</label>
                  <input
                    type="number"
                    placeholder="e.g. 150000"
                    value={jobSalaryMinNaira}
                    onChange={(e) => setJobSalaryMinNaira(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Max Salary (₦/mo)</label>
                  <input
                    type="number"
                    placeholder="e.g. 250000"
                    value={jobSalaryMaxNaira}
                    onChange={(e) => setJobSalaryMaxNaira(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">
                  Required Competencies & Skills (comma separated)
                </label>
                <input
                  type="text"
                  placeholder="e.g. Waste Management, OSHA 30, Safety Audits, First Aid"
                  value={jobSkillsText}
                  onChange={(e) => setJobSkillsText(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">Role Description *</label>
                <textarea
                  rows={3}
                  required
                  placeholder="Provide a clear description of the role and team objectives..."
                  value={jobDescription}
                  onChange={(e) => setJobDescription(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Responsibilities</label>
                  <textarea
                    rows={3}
                    placeholder="List core day-to-day duties..."
                    value={jobResponsibilities}
                    onChange={(e) => setJobResponsibilities(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Requirements</label>
                  <textarea
                    rows={3}
                    placeholder="Certifications, degrees, or years of experience required..."
                    value={jobRequirements}
                    onChange={(e) => setJobRequirements(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold text-gray-700 uppercase mb-1">Application Deadline</label>
                  <input
                    type="date"
                    value={jobDeadline}
                    onChange={(e) => setJobDeadline(e.target.value)}
                    className="w-full p-2.5 rounded-lg border border-gray-300"
                  />
                </div>
                <div className="flex items-center pt-5">
                  <label className="flex items-center gap-2 cursor-pointer text-gray-700 font-medium">
                    <input
                      type="checkbox"
                      checked={jobNegotiable}
                      onChange={(e) => setJobNegotiable(e.target.checked)}
                      className="rounded text-emerald-800 focus:ring-emerald-700"
                    />
                    <span>Compensation is negotiable</span>
                  </label>
                </div>
              </div>

              <div className="pt-4 border-t border-gray-100 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setShowJobModal(false)}
                  className="px-4 py-2 text-gray-700 font-medium hover:text-gray-900 transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={jobSubmitting}
                  className="px-6 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium transition disabled:opacity-50"
                >
                  {jobSubmitting ? "Posting..." : "Publish Vacancy"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Update Application Status */}
      {statusApp && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-2xl border border-gray-200">
            <h3 className="text-lg font-bold text-gray-900 mb-1">Update Recruitment Status</h3>
            <p className="text-xs text-gray-600 mb-4">
              Candidate: <strong>{statusApp.candidate_name}</strong>
            </p>

            <form onSubmit={handleUpdateAppStatus} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">New Status</label>
                <select
                  value={targetStatus}
                  onChange={(e) => setTargetStatus(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                >
                  <option value="under_review">Under Review</option>
                  <option value="shortlisted">Shortlisted</option>
                  <option value="interview">Interview</option>
                  <option value="offered">Job Offer Extended</option>
                  <option value="hired">Hired</option>
                  <option value="rejected">Not Selected (Rejected)</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">
                  Transition Notes / Feedback
                </label>
                <textarea
                  rows={3}
                  value={statusReason}
                  onChange={(e) => setStatusReason(e.target.value)}
                  placeholder="Notes for the audit log or feedback for the candidate..."
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setStatusApp(null)}
                  className="px-4 py-2 text-gray-700 font-medium hover:text-gray-900"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={statusSubmitting}
                  className="px-4 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium disabled:opacity-50"
                >
                  {statusSubmitting ? "Updating..." : "Save Status"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Schedule Interview */}
      {interviewApp && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6 shadow-2xl border border-gray-200">
            <h3 className="text-lg font-bold text-gray-900 mb-1">Schedule Interview</h3>
            <p className="text-xs text-gray-600 mb-4">
              Candidate: <strong>{interviewApp.candidate_name}</strong>
            </p>

            <form onSubmit={handleScheduleInterview} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">
                  Date & Time *
                </label>
                <input
                  type="datetime-local"
                  required
                  value={intTime}
                  onChange={(e) => setIntTime(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">Format *</label>
                <select
                  value={intFormat}
                  onChange={(e: any) => setIntFormat(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                >
                  <option value="video">Video Call (Google Meet / Zoom)</option>
                  <option value="in_person">In-person Interview</option>
                  <option value="phone">Phone Screening</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">
                  Meeting Link or Venue Address *
                </label>
                <input
                  type="text"
                  required
                  placeholder="e.g. https://meet.google.com/xyz or Office Conference Room"
                  value={intLocation}
                  onChange={(e) => setIntLocation(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 uppercase mb-1">
                  Instructions for Candidate
                </label>
                <textarea
                  rows={2}
                  placeholder="e.g. Please bring original identification documents and certifications..."
                  value={intInstructions}
                  onChange={(e) => setIntInstructions(e.target.value)}
                  className="w-full p-2.5 rounded-lg border border-gray-300"
                />
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setInterviewApp(null)}
                  className="px-4 py-2 text-gray-700 font-medium hover:text-gray-900"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={intSubmitting}
                  className="px-4 py-2 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium disabled:opacity-50"
                >
                  {intSubmitting ? "Scheduling..." : "Confirm Schedule"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
