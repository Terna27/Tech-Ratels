"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  Briefcase,
  Search,
  MapPin,
  Building2,
  Clock,
  Filter,
  CheckCircle2,
  ArrowRight,
  ShieldCheck,
  UserCheck,
  Sparkles,
  Calendar,
  Layers,
  FileText,
  PlusCircle,
  X,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { CareersJob, safeJsonParse } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";

const JOB_CATEGORIES = [
  { id: "", label: "All Categories" },
  { id: "sanitation_environment", label: "Sanitation & Environment" },
  { id: "logistics_supply_chain", label: "Logistics & Supply Chain" },
  { id: "safety_security", label: "Health, Safety & PPE" },
  { id: "operations_management", label: "Operations & Management" },
  { id: "community_advocacy", label: "Community & Advocacy" },
  { id: "administration_hr", label: "Administration & HR" },
  { id: "technology_data", label: "Technology & Data" },
];

const WORKPLACE_TYPES = [
  { id: "", label: "All Workplaces" },
  { id: "on_site", label: "On-site" },
  { id: "remote", label: "Remote" },
  { id: "hybrid", label: "Hybrid" },
];

const EMPLOYMENT_TYPES = [
  { id: "", label: "All Types" },
  { id: "full_time", label: "Full-time" },
  { id: "part_time", label: "Part-time" },
  { id: "contract", label: "Contract" },
  { id: "internship", label: "Internship" },
  { id: "volunteer", label: "Volunteer" },
];

export default function CareersDiscoveryPage() {
  const { user } = useAuth();

  const [jobs, setJobs] = useState<CareersJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [selectedCategory, setSelectedCategory] = useState<string>("");
  const [selectedWorkplace, setSelectedWorkplace] = useState<string>("");
  const [selectedEmployment, setSelectedEmployment] = useState<string>("");
  const [searchQuery, setSearchQuery] = useState<string>("");

  useEffect(() => {
    fetchJobs();
  }, [selectedCategory, selectedWorkplace, selectedEmployment]);

  const fetchJobs = async (query = searchQuery) => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (selectedCategory) params.set("category", selectedCategory);
      if (selectedWorkplace) params.set("workplace_type", selectedWorkplace);
      if (selectedEmployment) params.set("employment_type", selectedEmployment);
      if (query.trim()) params.set("q", query.trim());

      const res = await apiRequest<CareersJob[]>(`/careers/jobs?${params.toString()}`);
      setJobs(Array.isArray(res) ? res : []);
    } catch (err: any) {
      setError(err.message || "Failed to load job listings");
    } finally {
      setLoading(false);
    }
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    fetchJobs(searchQuery);
  };

  const clearFilters = () => {
    setSelectedCategory("");
    setSelectedWorkplace("");
    setSelectedEmployment("");
    setSearchQuery("");
    fetchJobs("");
  };

  const formatNaira = (kobo?: number) => {
    if (!kobo || kobo <= 0) return null;
    const naira = Math.floor(kobo / 100);
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: "NGN",
      maximumFractionDigits: 0,
    }).format(naira);
  };

  const formatSalaryRange = (job: CareersJob) => {
    const min = formatNaira(job.salary_min_kobo);
    const max = formatNaira(job.salary_max_kobo);
    if (min && max) {
      return `${min} – ${max}`;
    }
    if (min) return `From ${min}`;
    if (max) return `Up to ${max}`;
    if (job.is_salary_negotiable) return "Negotiable";
    return "Competitive";
  };

  const formatCategoryLabel = (cat: string) => {
    const found = JOB_CATEGORIES.find((c) => c.id === cat);
    return found ? found.label : cat.replace(/_/g, " ").replace(/\b\w/g, (l) => l.toUpperCase());
  };

  return (
    <div className="min-h-screen bg-gray-50 pb-20">
      {/* Hero Header */}
      <section className="bg-emerald-900 text-white py-14 px-4 sm:px-6 lg:px-8 border-b border-emerald-800">
        <div className="max-w-7xl mx-auto">
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-8">
            <div className="max-w-3xl">
              <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-800/80 text-emerald-200 text-xs font-semibold uppercase tracking-wider mb-4 border border-emerald-700/60">
                <ShieldCheck className="w-3.5 h-3.5 text-emerald-300" />
                Verified Opportunities • Ratels Network
              </div>
              <h1 className="text-3xl sm:text-4xl lg:text-5xl font-extrabold tracking-tight text-white mb-4">
                Ratels Careers
              </h1>
              <p className="text-emerald-100 text-base sm:text-lg leading-relaxed max-w-2xl">
                Discover verified career and volunteer opportunities in environmental sanitation,
                logistics, health and safety, and community leadership across Nigeria.
              </p>
            </div>

            {/* Quick Actions Portal Links */}
            <div className="flex flex-col sm:flex-row md:flex-col gap-3 min-w-[240px]">
              <Link
                href="/careers/profile"
                className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-emerald-700 hover:bg-emerald-600 text-white font-medium text-sm transition shadow-sm border border-emerald-600"
              >
                <UserCheck className="w-4 h-4 text-emerald-300" />
                Candidate Profile
              </Link>
              <Link
                href="/careers/applications"
                className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-white/10 hover:bg-white/20 text-white font-medium text-sm transition border border-emerald-700/50"
              >
                <FileText className="w-4 h-4 text-emerald-200" />
                Track Applications
              </Link>
              <Link
                href="/careers/employer"
                className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-white/10 hover:bg-white/20 text-white font-medium text-sm transition border border-emerald-700/50"
              >
                <Building2 className="w-4 h-4 text-emerald-200" />
                Employer Portal
              </Link>
            </div>
          </div>

          {/* Search Box */}
          <form
            onSubmit={handleSearchSubmit}
            className="mt-8 bg-white p-2 rounded-xl shadow-lg border border-gray-100 flex flex-col sm:flex-row gap-2 max-w-4xl"
          >
            <div className="flex-1 flex items-center px-3 gap-2">
              <Search className="w-5 h-5 text-gray-400" />
              <input
                type="text"
                placeholder="Search job title, skills (e.g. OSHA, Logistics, Waste Management)..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full text-sm py-2 text-gray-900 placeholder-gray-500 focus:outline-none"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => {
                    setSearchQuery("");
                    fetchJobs("");
                  }}
                  className="text-gray-400 hover:text-gray-600 p-1"
                >
                  <X className="w-4 h-4" />
                </button>
              )}
            </div>
            <button
              type="submit"
              className="bg-emerald-800 hover:bg-emerald-700 text-white font-medium px-6 py-2.5 rounded-lg text-sm transition flex items-center justify-center gap-2"
            >
              <Search className="w-4 h-4" />
              Search Vacancies
            </button>
          </form>
        </div>
      </section>

      {/* Main Content Area */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-8">
        {/* Category Pills & Filters Bar */}
        <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm mb-8 space-y-4">
          <div>
            <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2">
              Browse by Sector
            </span>
            <div className="flex items-center gap-2 overflow-x-auto pb-1 scrollbar-thin">
              {JOB_CATEGORIES.map((cat) => (
                <button
                  key={cat.id}
                  onClick={() => setSelectedCategory(cat.id)}
                  className={`text-xs px-3 py-1.5 rounded-full font-medium whitespace-nowrap transition ${
                    selectedCategory === cat.id
                      ? "bg-emerald-900 text-white shadow-sm"
                      : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                  }`}
                >
                  {cat.label}
                </button>
              ))}
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 pt-3 border-t border-gray-100 text-sm">
            <div className="flex flex-wrap items-center gap-3">
              <span className="text-xs font-medium text-gray-500 flex items-center gap-1">
                <Filter className="w-3.5 h-3.5" />
                Filters:
              </span>

              {/* Workplace Filter */}
              <select
                value={selectedWorkplace}
                onChange={(e) => setSelectedWorkplace(e.target.value)}
                className="bg-gray-50 border border-gray-300 text-gray-700 text-xs rounded-lg px-2.5 py-1.5 focus:ring-emerald-500 focus:border-emerald-500"
              >
                {WORKPLACE_TYPES.map((wp) => (
                  <option key={wp.id} value={wp.id}>
                    {wp.label}
                  </option>
                ))}
              </select>

              {/* Employment Type Filter */}
              <select
                value={selectedEmployment}
                onChange={(e) => setSelectedEmployment(e.target.value)}
                className="bg-gray-50 border border-gray-300 text-gray-700 text-xs rounded-lg px-2.5 py-1.5 focus:ring-emerald-500 focus:border-emerald-500"
              >
                {EMPLOYMENT_TYPES.map((emp) => (
                  <option key={emp.id} value={emp.id}>
                    {emp.label}
                  </option>
                ))}
              </select>

              {(selectedCategory || selectedWorkplace || selectedEmployment || searchQuery) && (
                <button
                  onClick={clearFilters}
                  className="text-xs text-rose-600 hover:text-rose-800 font-medium flex items-center gap-1 underline underline-offset-2 ml-1"
                >
                  Clear all filters
                </button>
              )}
            </div>

            <div className="text-xs text-gray-500 font-medium">
              Showing <span className="font-semibold text-gray-900">{jobs.length}</span> published{" "}
              {jobs.length === 1 ? "position" : "positions"}
            </div>
          </div>
        </div>

        {/* Error Alert */}
        {error && (
          <div className="mb-6">
            <Alert type="error" title="Could not load opportunities">
              {error}
            </Alert>
          </div>
        )}

        {/* Jobs List */}
        {loading ? (
          <div className="py-16">
            <Spinner text="Loading published job vacancies..." />
          </div>
        ) : jobs.length === 0 ? (
          <EmptyState
            icon={<Briefcase className="w-8 h-8 text-gray-400" />}
            title="No Vacancies Found"
            description="There are currently no published opportunities matching your search criteria. Try modifying your filters or check back soon."
            actionText="Clear Filters"
            onAction={clearFilters}
          />
        ) : (
          <div className="grid grid-cols-1 gap-4">
            {jobs.map((job) => {
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

              return (
                <div
                  key={job.id}
                  className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm hover:shadow-md transition flex flex-col md:flex-row md:items-center justify-between gap-6"
                >
                  {/* Job Details */}
                  <div className="flex-1 space-y-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant="primary" size="sm">
                        {formatCategoryLabel(job.category)}
                      </Badge>
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
                    </div>

                    <div>
                      <Link
                        href={`/careers/jobs/${job.id}`}
                        className="text-lg sm:text-xl font-bold text-gray-900 hover:text-emerald-800 transition block"
                      >
                        {job.title}
                      </Link>
                      <div className="flex flex-wrap items-center gap-3 text-xs sm:text-sm text-gray-600 mt-1">
                        <span className="font-medium text-gray-900 flex items-center gap-1">
                          <Building2 className="w-4 h-4 text-emerald-700" />
                          {job.employer_name || "Verified Organization"}
                          {job.employer_verification_status === "verified" && (
                            <span title="Verified Employer" className="inline-block">
                              <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600 inline ml-0.5" />
                            </span>
                          )}
                        </span>
                        <span>•</span>
                        <span className="flex items-center gap-1 text-gray-500">
                          <MapPin className="w-3.5 h-3.5" />
                          {job.location_summary || "Nigeria"}
                        </span>
                        <span>•</span>
                        <span className="font-semibold text-emerald-800">
                          {formatSalaryRange(job)}
                        </span>
                      </div>
                    </div>

                    {/* Brief Excerpt */}
                    <p className="text-sm text-gray-600 line-clamp-2">
                      {job.description}
                    </p>

                    {/* Skill Tags */}
                    {skills.length > 0 && (
                      <div className="flex flex-wrap items-center gap-1.5 pt-1">
                        {skills.slice(0, 5).map((skill, idx) => (
                          <span
                            key={idx}
                            className="text-xs bg-gray-100 text-gray-700 px-2 py-0.5 rounded border border-gray-200"
                          >
                            {skill}
                          </span>
                        ))}
                        {skills.length > 5 && (
                          <span className="text-xs text-gray-400">
                            +{skills.length - 5} more
                          </span>
                        )}
                      </div>
                    )}
                  </div>

                  {/* Right Actions */}
                  <div className="flex flex-col sm:flex-row md:flex-col items-start md:items-end justify-between gap-3 border-t md:border-t-0 pt-4 md:pt-0 border-gray-100 min-w-[180px]">
                    {job.deadline_at && (
                      <div className="text-xs text-gray-500 flex items-center gap-1">
                        <Calendar className="w-3.5 h-3.5 text-amber-600" />
                        Deadline:{" "}
                        <span className="font-medium text-gray-700">
                          {new Date(job.deadline_at).toLocaleDateString("en-NG", {
                            day: "numeric",
                            month: "short",
                            year: "numeric",
                          })}
                        </span>
                      </div>
                    )}

                    <Link
                      href={`/careers/jobs/${job.id}`}
                      className="inline-flex items-center justify-center gap-2 w-full sm:w-auto px-5 py-2.5 rounded-lg bg-emerald-800 hover:bg-emerald-900 text-white font-medium text-sm transition shadow-sm"
                    >
                      View & Apply
                      <ArrowRight className="w-4 h-4" />
                    </Link>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Informational Callout: Transparent Recruitment & Privacy Guarantee */}
        <div className="mt-12 bg-white rounded-xl border border-emerald-100 p-6 sm:p-8 shadow-sm">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
            <div className="space-y-2 max-w-2xl">
              <h3 className="text-lg font-bold text-gray-900 flex items-center gap-2">
                <ShieldCheck className="w-5 h-5 text-emerald-700" />
                Fair, Explainable & Privacy-Preserving Recruitment
              </h3>
              <p className="text-sm text-gray-600 leading-relaxed">
                Ratels Careers provides objective, explainable qualification matching based on your
                verified skills, certifications, and experience. Candidate profiles are strictly
                private and never shared with employers until you explicitly submit an application.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/careers/profile"
                className="px-4 py-2 text-sm font-medium rounded-lg text-emerald-800 bg-emerald-50 hover:bg-emerald-100 border border-emerald-200 transition"
              >
                Complete Profile
              </Link>
              <Link
                href="/careers/employer"
                className="px-4 py-2 text-sm font-medium rounded-lg text-gray-700 bg-gray-100 hover:bg-gray-200 transition"
              >
                Post an Opportunity
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
