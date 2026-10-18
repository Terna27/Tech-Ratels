"use client";

import React, { useEffect, useState, use } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Building2,
  MapPin,
  Calendar,
  Briefcase,
  CheckCircle2,
  Clock,
  ShieldCheck,
  Send,
  FileText,
  AlertCircle,
  ExternalLink,
  User,
  Sparkles,
} from "lucide-react";
import { apiRequest, ApiError } from "@/lib/api";
import { CareersJob, CareersCandidateProfile, safeJsonParse } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function JobDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  const { id } = use(params);
  const { user } = useAuth();

  const [job, setJob] = useState<CareersJob | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Application Modal & Form State
  const [showApplyModal, setShowApplyModal] = useState(false);
  const [coverLetter, setCoverLetter] = useState("");
  const [qualificationsSummary, setQualificationsSummary] = useState("");
  const [resumeUrl, setResumeUrl] = useState("");
  const [resumeFilename, setResumeFilename] = useState("");
  const [useProfileResume, setUseProfileResume] = useState(true);

  // Candidate Profile if logged in
  const [candidateProfile, setCandidateProfile] = useState<CareersCandidateProfile | null>(null);

  // Submission State
  const [submitting, setSubmitting] = useState(false);
  const [submitSuccess, setSubmitSuccess] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    async function loadJob() {
      setLoading(true);
      setError(null);
      try {
        const data = await apiRequest<CareersJob>(`/careers/jobs/${id}`);
        setJob(data);
      } catch (err: any) {
        setError(err.message || "Failed to load job details");
      } finally {
        setLoading(false);
      }
    }
    loadJob();
  }, [id]);

  useEffect(() => {
    if (user) {
      async function loadProfile() {
        try {
          const profile = await apiRequest<CareersCandidateProfile>("/careers/candidate/profile");
          if (profile && profile.id) {
            setCandidateProfile(profile);
            if (profile.resume_url) {
              setResumeUrl(profile.resume_url);
              setResumeFilename(profile.resume_filename || "Candidate_CV.pdf");
            }
            if (profile.summary && !qualificationsSummary) {
              setQualificationsSummary(profile.summary);
            }
          }
        } catch {
          // It's ok if profile doesn't exist yet
        }
      }
      loadProfile();
    }
  }, [user]);

  const formatNaira = (kobo?: number) => {
    if (!kobo || kobo <= 0) return null;
    const naira = Math.floor(kobo / 100);
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: "NGN",
      maximumFractionDigits: 0,
    }).format(naira);
  };

  const formatSalaryRange = (j: CareersJob) => {
    const min = formatNaira(j.salary_min_kobo);
    const max = formatNaira(j.salary_max_kobo);
    if (min && max) {
      return `${min} – ${max}`;
    }
    if (min) return `From ${min}`;
    if (max) return `Up to ${max}`;
    if (j.is_salary_negotiable) return "Negotiable";
    return "Competitive Compensation";
  };

  const isExpired = () => {
    if (!job?.deadline_at) return false;
    return new Date(job.deadline_at).getTime() < Date.now();
  };

  const handleApplySubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) {
      router.push(`/login?redirect=/careers/jobs/${id}`);
      return;
    }

    if (!coverLetter.trim() || !qualificationsSummary.trim()) {
      setSubmitError("Please fill out both your cover statement and qualifications summary.");
      return;
    }

    setSubmitting(true);
    setSubmitError(null);

    try {
      await apiRequest(`/careers/jobs/${id}/apply`, {
        method: "POST",
        body: JSON.stringify({
          cover_letter: coverLetter.trim(),
          qualifications_summary: qualificationsSummary.trim(),
          resume_url: resumeUrl.trim() || (candidateProfile?.resume_url || ""),
          resume_filename: resumeFilename.trim() || (candidateProfile?.resume_filename || "Resume.pdf"),
        }),
      });

      setSubmitSuccess(true);
    } catch (err: any) {
      if (err.status === 409 || err.code === "DUPLICATE_APPLICATION") {
        setSubmitError("You have already submitted an active application for this opportunity.");
      } else {
        setSubmitError(err.message || "Failed to submit application. Please check your inputs.");
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center py-20">
        <Spinner text="Loading vacancy details..." />
      </div>
    );
  }

  if (error || !job) {
    return (
      <div className="min-h-screen bg-gray-50 py-16 px-4">
        <div className="max-w-3xl mx-auto space-y-4">
          <Alert type="error" title="Opportunity Unavailable">
            {error || "The requested job opportunity was not found or has expired."}
          </Alert>
          <Link
            href="/careers"
            className="inline-flex items-center gap-2 text-emerald-800 hover:text-emerald-900 font-medium text-sm"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Careers Discovery
          </Link>
        </div>
      </div>
    );
  }

  const skills: string[] = safeJsonParse<string[]>(job.skills_required, []);

  const workplaceLabel =
    job.workplace_type === "on_site"
      ? "On-site"
      : job.workplace_type === "remote"
      ? "Remote"
      : "Hybrid";

  const employmentLabel =
    job.employment_type === "full_time"
      ? "Full-time"
      : job.employment_type === "part_time"
      ? "Part-time"
      : job.employment_type === "contract"
      ? "Contract"
      : job.employment_type === "internship"
      ? "Internship"
      : "Volunteer";

  const hasExpired = isExpired();

  return (
    <div className="min-h-screen bg-gray-50 pb-20">
      {/* Top Breadcrumb & Navigation */}
      <div className="bg-white border-b border-gray-200 py-4 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <Link
            href="/careers"
            className="inline-flex items-center gap-2 text-gray-600 hover:text-emerald-800 text-sm font-medium transition"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to All Vacancies
          </Link>
          <div className="flex items-center gap-2">
            <Badge variant="primary" size="sm">
              {job.category.replace(/_/g, " ").toUpperCase()}
            </Badge>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Main Column: Job Details */}
          <div className="lg:col-span-2 space-y-6">
            {/* Header Card */}
            <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm">
              <div className="flex flex-wrap items-center gap-2 mb-3">
                <Badge variant="neutral" size="sm">
                  {employmentLabel}
                </Badge>
                <Badge variant="info" size="sm">
                  {workplaceLabel}
                </Badge>
                {job.experience_level && job.experience_level !== "no_experience" && (
                  <Badge variant="warning" size="sm">
                    {job.experience_level.toUpperCase()}
                  </Badge>
                )}
                {hasExpired && (
                  <Badge variant="danger" size="sm">
                    EXPIRED
                  </Badge>
                )}
              </div>

              <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight mb-2">
                {job.title}
              </h1>

              <div className="flex flex-wrap items-center gap-4 text-sm text-gray-600 mb-6">
                <span className="font-semibold text-gray-900 flex items-center gap-1.5">
                  <Building2 className="w-4 h-4 text-emerald-700" />
                  {job.employer_name || "Verified Organization"}
                  {job.employer_verification_status === "verified" && (
                    <span title="Verified Employer">
                      <CheckCircle2 className="w-4 h-4 text-emerald-600 inline" />
                    </span>
                  )}
                </span>
                <span>•</span>
                <span className="flex items-center gap-1 text-gray-500">
                  <MapPin className="w-4 h-4" />
                  {job.location_summary || "Nigeria"}
                </span>
                <span>•</span>
                <span className="font-bold text-emerald-800">{formatSalaryRange(job)}</span>
              </div>

              {/* Deadline & Expiry Notice */}
              {job.deadline_at && (
                <div
                  className={`p-3 rounded-lg flex items-center gap-2 text-xs font-medium ${
                    hasExpired
                      ? "bg-rose-50 text-rose-800 border border-rose-200"
                      : "bg-amber-50 text-amber-800 border border-amber-200"
                  }`}
                >
                  <Calendar className="w-4 h-4 flex-shrink-0" />
                  <span>
                    Application Deadline:{" "}
                    <strong>
                      {new Date(job.deadline_at).toLocaleDateString("en-NG", {
                        weekday: "short",
                        day: "numeric",
                        month: "long",
                        year: "numeric",
                      })}
                    </strong>
                    {hasExpired && " (This opportunity has closed and is no longer accepting submissions)"}
                  </span>
                </div>
              )}
            </div>

            {/* Description Card */}
            <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8 shadow-sm space-y-6">
              <div>
                <h2 className="text-lg font-bold text-gray-900 mb-3">About the Role</h2>
                <div className="text-sm text-gray-700 leading-relaxed whitespace-pre-line">
                  {job.description}
                </div>
              </div>

              {job.responsibilities && (
                <div className="border-t border-gray-100 pt-6">
                  <h2 className="text-lg font-bold text-gray-900 mb-3">Key Responsibilities</h2>
                  <div className="text-sm text-gray-700 leading-relaxed whitespace-pre-line">
                    {job.responsibilities}
                  </div>
                </div>
              )}

              {job.requirements && (
                <div className="border-t border-gray-100 pt-6">
                  <h2 className="text-lg font-bold text-gray-900 mb-3">
                    Qualifications & Requirements
                  </h2>
                  <div className="text-sm text-gray-700 leading-relaxed whitespace-pre-line">
                    {job.requirements}
                  </div>
                </div>
              )}

              {/* Required Skills */}
              {skills.length > 0 && (
                <div className="border-t border-gray-100 pt-6">
                  <h2 className="text-lg font-bold text-gray-900 mb-3">Required Competencies</h2>
                  <div className="flex flex-wrap gap-2">
                    {skills.map((skill, idx) => (
                      <span
                        key={idx}
                        className="text-xs bg-emerald-50 text-emerald-800 px-3 py-1.5 rounded-full font-medium border border-emerald-200"
                      >
                        {skill}
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Sidebar: Employer Card & Quick Action */}
          <div className="space-y-6">
            {/* Quick Action Apply Card */}
            <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4">
              <h3 className="font-bold text-gray-900 text-base">Ready to Apply?</h3>
              <p className="text-xs text-gray-600 leading-relaxed">
                Submit your credentials and statement for this opportunity. Your candidate profile
                and personal details remain strictly private until submitted.
              </p>

              {hasExpired ? (
                <button
                  disabled
                  className="w-full py-3 px-4 rounded-lg bg-gray-200 text-gray-500 font-medium text-sm cursor-not-allowed text-center"
                >
                  Opportunity Closed
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => setShowApplyModal(true)}
                  className="w-full py-3 px-4 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium text-sm transition shadow-sm flex items-center justify-center gap-2"
                >
                  <Send className="w-4 h-4" />
                  Apply for this Position
                </button>
              )}

              <div className="text-xs text-gray-500 flex items-center gap-1.5 pt-2 border-t border-gray-100">
                <ShieldCheck className="w-4 h-4 text-emerald-600 flex-shrink-0" />
                <span>Fair & explainable recruitment audit trail</span>
              </div>
            </div>

            {/* Employer Profile Card */}
            <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm space-y-4">
              <h3 className="font-bold text-gray-900 text-base flex items-center gap-2">
                <Building2 className="w-4 h-4 text-emerald-700" />
                About the Employer
              </h3>

              <div className="space-y-2 text-sm">
                <div className="font-semibold text-gray-900 flex items-center gap-1">
                  {job.employer_name || "Verified Organization"}
                  {job.employer_verification_status === "verified" && (
                    <Badge variant="success" size="sm">
                      Verified
                    </Badge>
                  )}
                </div>

                {job.employer_industry && (
                  <p className="text-xs text-gray-600">
                    Industry: <span className="font-medium text-gray-800">{job.employer_industry}</span>
                  </p>
                )}

                {job.employer_website && (
                  <div className="pt-1">
                    <a
                      href={
                        job.employer_website.startsWith("http")
                          ? job.employer_website
                          : `https://${job.employer_website}`
                      }
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs text-emerald-800 hover:text-emerald-900 font-medium inline-flex items-center gap-1"
                    >
                      Visit Organization Website
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Application Modal */}
      {showApplyModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl max-w-2xl w-full p-6 sm:p-8 shadow-2xl border border-gray-200 relative animate-in fade-in zoom-in-95 duration-150">
            {submitSuccess ? (
              <div className="text-center py-8 space-y-4">
                <div className="w-16 h-16 bg-emerald-100 text-emerald-700 rounded-full flex items-center justify-center mx-auto">
                  <CheckCircle2 className="w-8 h-8" />
                </div>
                <h3 className="text-2xl font-bold text-gray-900">Application Submitted!</h3>
                <p className="text-sm text-gray-600 max-w-md mx-auto">
                  Your application for <strong>{job.title}</strong> at{" "}
                  <strong>{job.employer_name}</strong> has been successfully submitted and logged
                  with an immutable audit reference.
                </p>
                <div className="pt-4 flex flex-col sm:flex-row gap-3 justify-center">
                  <Link
                    href="/careers/applications"
                    className="px-5 py-2.5 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium text-sm transition"
                  >
                    View in Application Tracker
                  </Link>
                  <button
                    type="button"
                    onClick={() => {
                      setShowApplyModal(false);
                      setSubmitSuccess(false);
                    }}
                    className="px-5 py-2.5 rounded-lg bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium text-sm transition"
                  >
                    Close
                  </button>
                </div>
              </div>
            ) : (
              <div>
                <div className="flex items-center justify-between pb-4 border-b border-gray-100 mb-6">
                  <div>
                    <h3 className="text-xl font-bold text-gray-900">Apply for Position</h3>
                    <p className="text-xs text-gray-500 mt-0.5">{job.title}</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setShowApplyModal(false)}
                    className="text-gray-400 hover:text-gray-600 text-sm font-semibold p-1"
                  >
                    ✕
                  </button>
                </div>

                {!user ? (
                  <div className="space-y-4 py-4 text-center">
                    <Alert type="warning" title="Authentication Required">
                      You must be signed in with your Nigerian Ratels member account to apply for
                      vacancies.
                    </Alert>
                    <div className="pt-2 flex justify-center gap-3">
                      <Link
                        href={`/login?redirect=/careers/jobs/${id}`}
                        className="px-5 py-2.5 rounded-lg bg-emerald-800 text-white font-medium text-sm hover:bg-emerald-900 transition"
                      >
                        Sign In
                      </Link>
                      <Link
                        href="/register"
                        className="px-5 py-2.5 rounded-lg bg-gray-100 text-gray-800 font-medium text-sm hover:bg-gray-200 transition"
                      >
                        Register
                      </Link>
                    </div>
                  </div>
                ) : (
                  <form onSubmit={handleApplySubmit} className="space-y-5">
                    {submitError && (
                      <Alert type="error" title="Submission Error">
                        {submitError}
                      </Alert>
                    )}

                    {/* Qualifications Summary */}
                    <div>
                      <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                        Qualifications & Relevant Background *
                      </label>
                      <textarea
                        rows={3}
                        required
                        value={qualificationsSummary}
                        onChange={(e) => setQualificationsSummary(e.target.value)}
                        placeholder="Highlight your relevant certifications, past experience, and field competencies..."
                        className="w-full text-sm p-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-emerald-700 focus:border-emerald-700"
                      />
                    </div>

                    {/* Cover Letter */}
                    <div>
                      <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                        Cover Statement / Motivation *
                      </label>
                      <textarea
                        rows={4}
                        required
                        value={coverLetter}
                        onChange={(e) => setCoverLetter(e.target.value)}
                        placeholder="Explain why you are interested in this position and how you can contribute to the mission..."
                        className="w-full text-sm p-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-emerald-700 focus:border-emerald-700"
                      />
                    </div>

                    {/* Resume / Document URL */}
                    <div className="space-y-2 bg-gray-50 p-4 rounded-xl border border-gray-200">
                      <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider">
                        Curriculum Vitae / Résumé
                      </label>

                      {candidateProfile?.resume_url && (
                        <div className="flex items-center gap-2 text-xs text-emerald-800 font-medium mb-2">
                          <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                          <span>Using saved CV from Candidate Profile: </span>
                          <span className="underline">{candidateProfile.resume_filename || "Resume.pdf"}</span>
                        </div>
                      )}

                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                        <div>
                          <input
                            type="text"
                            placeholder="Document link (e.g. Google Drive, PDF URL)"
                            value={resumeUrl}
                            onChange={(e) => setResumeUrl(e.target.value)}
                            className="w-full text-xs p-2.5 rounded-lg border border-gray-300 focus:ring-emerald-700 focus:border-emerald-700"
                          />
                        </div>
                        <div>
                          <input
                            type="text"
                            placeholder="Document label (e.g. My_CV_2026.pdf)"
                            value={resumeFilename}
                            onChange={(e) => setResumeFilename(e.target.value)}
                            className="w-full text-xs p-2.5 rounded-lg border border-gray-300 focus:ring-emerald-700 focus:border-emerald-700"
                          />
                        </div>
                      </div>
                      <p className="text-[11px] text-gray-500">
                        Provide a verifiable document link or attach via your{" "}
                        <Link href="/careers/profile" className="text-emerald-800 underline font-medium">
                          Candidate Profile
                        </Link>
                        .
                      </p>
                    </div>

                    {/* Footer Actions */}
                    <div className="flex items-center justify-end gap-3 pt-4 border-t border-gray-100">
                      <button
                        type="button"
                        onClick={() => setShowApplyModal(false)}
                        className="px-4 py-2 text-sm font-medium text-gray-700 hover:text-gray-900 transition"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        disabled={submitting}
                        className="px-6 py-2.5 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium text-sm transition shadow-sm flex items-center gap-2 disabled:opacity-50"
                      >
                        {submitting ? (
                          <>
                            <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                            Submitting...
                          </>
                        ) : (
                          <>
                            <Send className="w-4 h-4" />
                            Submit Application
                          </>
                        )}
                      </button>
                    </div>
                  </form>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
