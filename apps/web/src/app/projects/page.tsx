"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import {
  FolderKanban,
  Search,
  Filter,
  MapPin,
  Calendar,
  CheckCircle2,
  Clock,
  ArrowRight,
  TrendingUp,
  Coins,
  ShieldCheck,
  PlusCircle,
  AlertCircle,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import {
  CommunityProject,
  CommunityProjectsOverviewStats,
  State,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";

export default function PublicProjectsPage() {
  const [projects, setProjects] = useState<CommunityProject[]>([]);
  const [stats, setStats] = useState<CommunityProjectsOverviewStats | null>(null);
  const [states, setStates] = useState<State[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [selectedState, setSelectedState] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("");
  const [selectedStatus, setSelectedStatus] = useState("");
  const [searchQuery, setSearchQuery] = useState("");

  const categories = [
    { key: "environmental_sanitation", label: "Environmental Sanitation" },
    { key: "water_and_boreholes", label: "Water & Boreholes" },
    { key: "waste_management", label: "Waste Management" },
    { key: "drainage_and_flooding", label: "Drainage & Flood Prevention" },
    { key: "public_infrastructure", label: "Public Infrastructure" },
    { key: "community_health_hygiene", label: "Community Health & Hygiene" },
    { key: "education_and_awareness", label: "Civic Education & Awareness" },
    { key: "other", label: "Other Community Needs" },
  ];

  useEffect(() => {
    async function loadData() {
      setLoading(true);
      setError(null);
      try {
        const [statsRes, statesRes] = await Promise.all([
          apiRequest<CommunityProjectsOverviewStats>("/community/projects/stats").catch(() => null),
          apiRequest<State[]>("/geography/states").catch(() => []),
        ]);
        if (statsRes) setStats(statsRes);
        if (statesRes) setStates(statesRes);

        const queryParams = new URLSearchParams();
        if (selectedState) queryParams.set("state_id", selectedState);
        if (selectedCategory) queryParams.set("category", selectedCategory);
        if (selectedStatus) queryParams.set("execution_status", selectedStatus);

        const projectsRes = await apiRequest<{ projects: CommunityProject[]; total: number }>(
          `/community/projects?${queryParams.toString()}`
        );
        setProjects(projectsRes?.projects || []);
      } catch (err: any) {
        console.error("Failed to load community projects", err);
        setError("Failed to load projects. Please try refreshing the page.");
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [selectedState, selectedCategory, selectedStatus]);

  const filteredProjects = projects.filter((p) => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return (
      p.title.toLowerCase().includes(q) ||
      p.description.toLowerCase().includes(q) ||
      p.reference.toLowerCase().includes(q) ||
      (p.state_name && p.state_name.toLowerCase().includes(q)) ||
      (p.lga_name && p.lga_name.toLowerCase().includes(q)) ||
      p.community_ward.toLowerCase().includes(q)
    );
  });

  const formatNaira = (kobo: number) => {
    const naira = Math.floor(kobo / 100);
    return new Intl.NumberFormat("en-NG", {
      style: "currency",
      currency: "NGN",
      maximumFractionDigits: 0,
    }).format(naira);
  };

  const getStatusBadgeVariant = (status: string): "success" | "info" | "warning" | "danger" | "neutral" => {
    switch (status) {
      case "completed":
        return "success";
      case "in_progress":
        return "info";
      case "awaiting_verification":
        return "warning";
      case "paused":
        return "neutral";
      case "cancelled":
        return "danger";
      default:
        return "neutral";
    }
  };

  return (
    <div className="space-y-10 pb-16">
      {/* Hero Header */}
      <section className="bg-gradient-to-b from-emerald-950 via-emerald-900 to-gray-900 text-white pt-14 pb-16 px-4 sm:px-6 lg:px-8 border-b border-emerald-800">
        <div className="max-w-6xl mx-auto space-y-6">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-800/80 border border-emerald-700 text-emerald-200 text-xs font-semibold">
            <FolderKanban className="w-4 h-4 text-emerald-300" />
            <span>Civic Transparency & Grassroots Intervention Tracker</span>
          </div>

          <h1 className="text-3xl sm:text-5xl font-extrabold tracking-tight leading-tight">
            Ratels Community Projects <br />
            <span className="text-emerald-400">& Public Accountability.</span>
          </h1>

          <p className="text-base sm:text-lg text-emerald-100/90 max-w-3xl leading-relaxed">
            Direct community-driven development across Nigeria&apos;s 36 States and the FCT. Browse verified grassroots interventions, track milestone executions, and inspect double-entry financial accountability.
          </p>

          <div className="flex flex-wrap gap-4 pt-2">
            <Link
              href="/projects/submit"
              className="inline-flex items-center gap-2 px-6 py-3 text-sm font-semibold rounded-xl text-emerald-950 bg-emerald-400 hover:bg-emerald-300 shadow-md transition"
            >
              <PlusCircle className="w-4 h-4" />
              Submit Community Request
            </Link>
            <Link
              href="/projects/requests"
              className="inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl text-white bg-emerald-900/80 border border-emerald-700 hover:bg-emerald-800 transition"
            >
              Track My Submissions
              <ArrowRight className="w-4 h-4" />
            </Link>
          </div>
        </div>
      </section>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 space-y-8">
        {/* Statistics Banner */}
        {stats && (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <div className="flex items-center justify-between text-gray-500 text-xs font-medium uppercase tracking-wider">
                <span>Active Interventions</span>
                <Clock className="w-4 h-4 text-blue-600" />
              </div>
              <p className="text-2xl font-bold text-gray-900">{stats.active_projects}</p>
              <p className="text-xs text-gray-500">Currently executing across states</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <div className="flex items-center justify-between text-gray-500 text-xs font-medium uppercase tracking-wider">
                <span>Completed & Verified</span>
                <CheckCircle2 className="w-4 h-4 text-emerald-600" />
              </div>
              <p className="text-2xl font-bold text-gray-900">{stats.completed_projects}</p>
              <p className="text-xs text-gray-500">100% verified milestones</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <div className="flex items-center justify-between text-gray-500 text-xs font-medium uppercase tracking-wider">
                <span>Approved Budget</span>
                <Coins className="w-4 h-4 text-amber-600" />
              </div>
              <p className="text-2xl font-bold text-gray-900">
                {formatNaira(stats.total_approved_budget)}
              </p>
              <p className="text-xs text-gray-500">Authorized project allocations</p>
            </div>

            <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-1">
              <div className="flex items-center justify-between text-gray-500 text-xs font-medium uppercase tracking-wider">
                <span>Recorded Expenditure</span>
                <TrendingUp className="w-4 h-4 text-purple-600" />
              </div>
              <p className="text-2xl font-bold text-gray-900">
                {formatNaira(stats.total_recorded_expenditure)}
              </p>
              <p className="text-xs text-gray-500">Ledger-verified disbursements</p>
            </div>
          </div>
        )}

        {/* Filter & Search Bar */}
        <div className="p-5 bg-white rounded-xl border border-gray-200 shadow-sm space-y-4">
          <div className="flex flex-col md:flex-row gap-3">
            <div className="flex-1 relative">
              <Search className="w-4 h-4 absolute left-3.5 top-3.5 text-gray-400" />
              <input
                type="text"
                placeholder="Search projects by title, community, LGA, state, or reference..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full pl-10 pr-4 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500"
              />
            </div>

            <div className="flex flex-wrap gap-2">
              <select
                value={selectedCategory}
                onChange={(e) => setSelectedCategory(e.target.value)}
                className="px-3 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500"
              >
                <option value="">All Categories</option>
                {categories.map((c) => (
                  <option key={c.key} value={c.key}>
                    {c.label}
                  </option>
                ))}
              </select>

              <select
                value={selectedState}
                onChange={(e) => setSelectedState(e.target.value)}
                className="px-3 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500"
              >
                <option value="">All States</option>
                {states.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>

              <select
                value={selectedStatus}
                onChange={(e) => setSelectedStatus(e.target.value)}
                className="px-3 py-2.5 text-sm bg-gray-50 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500"
              >
                <option value="">All Statuses</option>
                <option value="planned">Planned</option>
                <option value="in_progress">In Progress</option>
                <option value="awaiting_verification">Awaiting Verification</option>
                <option value="completed">Completed</option>
              </select>
            </div>
          </div>
        </div>

        {/* Projects Listing */}
        {loading ? (
          <div className="p-12 text-center text-gray-500 space-y-3">
            <div className="w-8 h-8 border-4 border-emerald-600 border-t-transparent rounded-full animate-spin mx-auto" />
            <p className="text-sm">Loading community projects...</p>
          </div>
        ) : error ? (
          <div className="p-8 bg-red-50 border border-red-200 rounded-xl text-center space-y-2">
            <AlertCircle className="w-6 h-6 text-red-600 mx-auto" />
            <p className="text-sm text-red-800">{error}</p>
          </div>
        ) : filteredProjects.length === 0 ? (
          <div className="p-12 bg-white rounded-2xl border border-dashed border-gray-300 text-center space-y-4">
            <FolderKanban className="w-12 h-12 text-gray-400 mx-auto" />
            <div className="space-y-1">
              <h3 className="text-base font-semibold text-gray-900">No community projects found</h3>
              <p className="text-sm text-gray-500 max-w-md mx-auto">
                No projects matched your selected filters. Try resetting the filters or submit a community request for your area.
              </p>
            </div>
            <Link
              href="/projects/submit"
              className="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-semibold rounded-lg text-emerald-950 bg-emerald-400 hover:bg-emerald-300 shadow-sm transition"
            >
              <PlusCircle className="w-4 h-4" />
              Submit a Request
            </Link>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {filteredProjects.map((p) => {
              const categoryLabel = categories.find((c) => c.key === p.category)?.label || p.category;
              return (
                <div
                  key={p.id}
                  className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm hover:shadow-md transition flex flex-col justify-between"
                >
                  <div className="p-6 space-y-4">
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-xs font-mono font-bold text-emerald-800 bg-emerald-50 px-2.5 py-1 rounded-md border border-emerald-200">
                        {p.reference}
                      </span>
                      <Badge variant={getStatusBadgeVariant(p.execution_status)}>
                        {p.execution_status.replace(/_/g, " ")}
                      </Badge>
                    </div>

                    <div className="space-y-1">
                      <span className="text-xs font-semibold text-emerald-700 uppercase tracking-wider block">
                        {categoryLabel}
                      </span>
                      <h3 className="text-lg font-bold text-gray-900 line-clamp-2">
                        {p.title}
                      </h3>
                    </div>

                    <p className="text-xs text-gray-600 line-clamp-3 leading-relaxed">
                      {p.description}
                    </p>

                    <div className="space-y-2 pt-2 border-t border-gray-100 text-xs text-gray-500">
                      <div className="flex items-center gap-1.5">
                        <MapPin className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                        <span className="truncate">
                          {p.community_ward}, {p.lga_name || "LGA"}, {p.state_name || "State"}
                        </span>
                      </div>
                      {p.target_completion_date && (
                        <div className="flex items-center gap-1.5">
                          <Calendar className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                          <span>Target: {new Date(p.target_completion_date).toLocaleDateString()}</span>
                        </div>
                      )}
                    </div>

                    {/* Progress Bar */}
                    <div className="space-y-1.5 pt-1">
                      <div className="flex justify-between text-xs font-medium">
                        <span className="text-gray-600">Verified Progress</span>
                        <span className="text-emerald-800 font-bold">{p.progress_percentage}%</span>
                      </div>
                      <div className="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
                        <div
                          className="bg-emerald-600 h-2 rounded-full transition-all duration-300"
                          style={{ width: `${p.progress_percentage}%` }}
                        />
                      </div>
                    </div>

                    {/* Financial Summary */}
                    <div className="p-3 bg-gray-50 rounded-xl space-y-1.5 text-xs border border-gray-100">
                      <div className="flex justify-between">
                        <span className="text-gray-500">Approved Budget:</span>
                        <span className="font-semibold text-gray-900">{formatNaira(p.approved_budget)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-500">Recorded Spent:</span>
                        <span className="font-medium text-emerald-700">{formatNaira(p.recorded_expenditure)}</span>
                      </div>
                    </div>
                  </div>

                  <div className="p-4 bg-gray-50 border-t border-gray-100">
                    <Link
                      href={`/projects/${p.id}`}
                      className="w-full inline-flex items-center justify-center gap-2 py-2.5 px-4 text-xs font-semibold rounded-lg text-white bg-emerald-900 hover:bg-emerald-800 transition shadow-sm"
                    >
                      Inspect Project Details & Milestones
                      <ArrowRight className="w-3.5 h-3.5" />
                    </Link>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Public Transparency Commitment Box */}
        <div className="p-6 bg-emerald-950 text-emerald-100 rounded-2xl border border-emerald-800 space-y-3">
          <div className="flex items-center gap-2">
            <ShieldCheck className="w-5 h-5 text-emerald-400" />
            <h4 className="font-bold text-white text-base">Ratels Public Transparency Guarantee</h4>
          </div>
          <p className="text-xs text-emerald-200/90 leading-relaxed max-w-4xl">
            All expenditures, milestones, and photo evidence on this portal are recorded against our immutable general ledger. Private member identities, internal review correspondence, and sensitive banking data are rigorously protected and never exposed publicly.
          </p>
        </div>
      </div>
    </div>
  );
}
