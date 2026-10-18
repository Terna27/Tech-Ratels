"use client";

import React, { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  FolderKanban,
  MapPin,
  Calendar,
  CheckCircle2,
  Clock,
  ArrowLeft,
  Coins,
  ShieldCheck,
  Building,
  UserCheck,
  FileText,
  AlertCircle,
  ExternalLink,
  ChevronRight,
  TrendingUp,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { PublicProjectResponse } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";

export default function PublicProjectDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;

  const [data, setData] = useState<PublicProjectResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadProject() {
      if (!id) return;
      setLoading(true);
      setError(null);
      try {
        const res = await apiRequest<PublicProjectResponse>(`/community/projects/${id}`);
        setData(res);
      } catch (err: any) {
        console.error("Failed to fetch community project", err);
        setError("Unable to load project details. The project may be private or does not exist.");
      } finally {
        setLoading(false);
      }
    }
    loadProject();
  }, [id]);

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

  if (loading) {
    return (
      <div className="max-w-5xl mx-auto px-4 py-20 text-center space-y-4">
        <div className="w-10 h-10 border-4 border-emerald-600 border-t-transparent rounded-full animate-spin mx-auto" />
        <p className="text-sm text-gray-500">Loading project details & public ledger...</p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="max-w-3xl mx-auto px-4 py-16 text-center space-y-4">
        <div className="p-8 bg-red-50 border border-red-200 rounded-2xl space-y-3">
          <AlertCircle className="w-8 h-8 text-red-600 mx-auto" />
          <h2 className="text-lg font-bold text-red-900">Project Not Found</h2>
          <p className="text-sm text-red-700">{error || "Requested project record could not be retrieved."}</p>
          <div className="pt-2">
            <Link
              href="/projects"
              className="inline-flex items-center gap-2 px-4 py-2 text-xs font-semibold rounded-lg text-white bg-emerald-900 hover:bg-emerald-800 transition"
            >
              <ArrowLeft className="w-4 h-4" />
              Back to Community Projects
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const { project, financial_summary } = data;
  const milestones = data.milestones || [];
  const updates = data.updates || [];

  return (
    <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Breadcrumb & Top Bar */}
      <div className="flex items-center justify-between">
        <Link
          href="/projects"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-emerald-800 hover:text-emerald-900 transition"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to All Projects
        </Link>
        <span className="text-xs font-mono font-bold text-emerald-800 bg-emerald-50 px-3 py-1 rounded-md border border-emerald-200">
          Ref: {project.reference}
        </span>
      </div>

      {/* Project Header Card */}
      <div className="p-6 sm:p-8 bg-white rounded-2xl border border-gray-200 shadow-sm space-y-6">
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-bold text-emerald-800 uppercase tracking-wider bg-emerald-50 px-2.5 py-1 rounded">
              {project.category.replace(/_/g, " ")}
            </span>
            <Badge variant={getStatusBadgeVariant(project.execution_status)}>
              {project.execution_status.replace(/_/g, " ")}
            </Badge>
            <Badge variant={project.funding_status === "fully_funded" ? "success" : "warning"}>
              {project.funding_status.replace(/_/g, " ")}
            </Badge>
          </div>

          <h1 className="text-2xl sm:text-3xl font-extrabold text-gray-900 tracking-tight">
            {project.title}
          </h1>

          <div className="flex flex-wrap items-center gap-4 text-xs text-gray-500 pt-1">
            <div className="flex items-center gap-1.5">
              <MapPin className="w-4 h-4 text-emerald-600" />
              <span>
                {project.community_ward}, {project.lga_name || "LGA"}, {project.state_name || "State"}
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <Building className="w-4 h-4 text-gray-400" />
              <span>{project.responsible_department}</span>
            </div>
            {project.responsible_officer_name && (
              <div className="flex items-center gap-1.5">
                <UserCheck className="w-4 h-4 text-emerald-600" />
                <span>Coordinator: {project.responsible_officer_name}</span>
              </div>
            )}
          </div>
        </div>

        <p className="text-sm text-gray-700 leading-relaxed whitespace-pre-line border-t border-gray-100 pt-4">
          {project.description}
        </p>

        {/* Location Details */}
        {project.location_details && (
          <div className="p-4 bg-gray-50 rounded-xl text-xs space-y-1 border border-gray-100">
            <span className="font-semibold text-gray-700">Specific Location / Landmark:</span>
            <p className="text-gray-600">{project.location_details}</p>
          </div>
        )}

        {/* Progress Tracker */}
        <div className="space-y-2 pt-2 border-t border-gray-100">
          <div className="flex justify-between items-center text-sm font-semibold">
            <span className="text-gray-700">Verified Milestone Execution</span>
            <span className="text-emerald-800 text-base">{project.progress_percentage}%</span>
          </div>
          <div className="w-full bg-gray-100 rounded-full h-3 overflow-hidden">
            <div
              className="bg-emerald-600 h-3 rounded-full transition-all duration-500"
              style={{ width: `${project.progress_percentage}%` }}
            />
          </div>
          <div className="flex justify-between text-xs text-gray-400">
            <span>
              {project.start_date ? `Started: ${new Date(project.start_date).toLocaleDateString()}` : "Start date pending"}
            </span>
            <span>
              {project.actual_completion_date
                ? `Completed: ${new Date(project.actual_completion_date).toLocaleDateString()}`
                : project.target_completion_date
                ? `Target: ${new Date(project.target_completion_date).toLocaleDateString()}`
                : "Timeline ongoing"}
            </span>
          </div>
        </div>
      </div>

      {/* Public Financial Accountability Section */}
      <div className="p-6 sm:p-8 bg-white rounded-2xl border border-gray-200 shadow-sm space-y-5">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Coins className="w-5 h-5 text-emerald-700" />
              <h2 className="text-lg font-bold text-gray-900">Public Financial Accountability</h2>
            </div>
            <p className="text-xs text-gray-500">
              Double-entry project fund ledger balances (NGN Kobo Minor-Units).
            </p>
          </div>
          <span className="text-xs font-semibold px-2.5 py-1 bg-emerald-50 text-emerald-800 rounded-md border border-emerald-200">
            Ledger-Verified
          </span>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 space-y-1">
            <span className="text-xs text-gray-500 font-medium">Approved Budget</span>
            <p className="text-lg font-bold text-gray-900">{formatNaira(financial_summary.approved_budget_kobo)}</p>
          </div>

          <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 space-y-1">
            <span className="text-xs text-gray-500 font-medium">Allocated Funding</span>
            <p className="text-lg font-bold text-blue-900">{formatNaira(financial_summary.allocated_funding_kobo)}</p>
          </div>

          <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 space-y-1">
            <span className="text-xs text-gray-500 font-medium">Recorded Spent</span>
            <p className="text-lg font-bold text-purple-900">{formatNaira(financial_summary.recorded_expenditure_kobo)}</p>
          </div>

          <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 space-y-1">
            <span className="text-xs text-gray-500 font-medium">Available Balance</span>
            <p className="text-lg font-bold text-emerald-900">{formatNaira(financial_summary.available_balance_kobo)}</p>
          </div>
        </div>

        <div className="p-3 bg-emerald-50/70 border border-emerald-100 rounded-xl text-xs text-emerald-900 flex items-center gap-2">
          <ShieldCheck className="w-4 h-4 text-emerald-700 shrink-0" />
          <span>
            Authoritative figures derived strictly from audited general ledger transactions. Sensitive bank details and donor identities are securely withheld.
          </span>
        </div>
      </div>

      {/* Verified Milestones Timeline */}
      <div className="p-6 sm:p-8 bg-white rounded-2xl border border-gray-200 shadow-sm space-y-6">
        <div className="space-y-1">
          <h2 className="text-lg font-bold text-gray-900">Project Milestones & Verification Trail</h2>
          <p className="text-xs text-gray-500">
            Chronological phases verified on site by independent Ratels inspection officers.
          </p>
        </div>

        {milestones.length === 0 ? (
          <p className="text-xs text-gray-500 italic py-4">No execution milestones recorded yet for this project.</p>
        ) : (
          <div className="space-y-4">
            {milestones.map((m, idx) => (
              <div
                key={m.id}
                className="p-5 rounded-xl border border-gray-200 bg-gray-50/50 space-y-3 relative"
              >
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <span className="w-6 h-6 rounded-full bg-emerald-900 text-white text-xs font-bold flex items-center justify-center shrink-0">
                      {m.sequence_order || idx + 1}
                    </span>
                    <h3 className="text-sm font-bold text-gray-900">{m.title}</h3>
                  </div>

                  <div className="flex items-center gap-2">
                    <Badge variant={m.status === "completed" ? "success" : "neutral"}>
                      {m.status.replace(/_/g, " ")}
                    </Badge>
                    <span
                      className={`text-xs px-2.5 py-0.5 rounded-full font-semibold border ${
                        m.verification_status === "verified"
                          ? "bg-emerald-100 text-emerald-800 border-emerald-300"
                          : m.verification_status === "rejected"
                          ? "bg-red-100 text-red-800 border-red-300"
                          : "bg-amber-100 text-amber-800 border-amber-300"
                      }`}
                    >
                      {m.verification_status === "verified" ? "✓ Verified" : "Pending Inspection"}
                    </span>
                  </div>
                </div>

                <p className="text-xs text-gray-600 leading-relaxed">{m.description}</p>

                <div className="flex flex-wrap gap-4 text-xs text-gray-500 pt-1 border-t border-gray-200/60">
                  <span>Target: {new Date(m.target_date).toLocaleDateString()}</span>
                  {m.completion_date && (
                    <span className="text-emerald-700 font-medium">
                      Completed: {new Date(m.completion_date).toLocaleDateString()}
                    </span>
                  )}
                  {m.assigned_person_name && (
                    <span>Assigned: {m.assigned_person_name}</span>
                  )}
                </div>

                {/* Verification Notes */}
                {m.verification_notes && (
                  <div className="p-3 bg-white rounded-lg border border-emerald-100 text-xs space-y-1">
                    <div className="flex items-center justify-between text-emerald-800 font-semibold">
                      <span>Officer Inspection Notes:</span>
                      {m.verified_by_name && <span>Inspector: {m.verified_by_name}</span>}
                    </div>
                    <p className="text-gray-600">{m.verification_notes}</p>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Progress Updates & Photo Gallery */}
      {updates.length > 0 && (
        <div className="p-6 sm:p-8 bg-white rounded-2xl border border-gray-200 shadow-sm space-y-6">
          <div className="space-y-1">
            <h2 className="text-lg font-bold text-gray-900">Field Updates & Documentation</h2>
            <p className="text-xs text-gray-500">Live community reports and photographic evidence.</p>
          </div>

          <div className="space-y-4">
            {updates.map((u) => (
              <div key={u.id} className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-2">
                <div className="flex items-center justify-between">
                  <h4 className="text-sm font-bold text-gray-900">{u.title}</h4>
                  <span className="text-xs text-gray-400">
                    {new Date(u.created_at).toLocaleDateString()}
                  </span>
                </div>
                <p className="text-xs text-gray-700 leading-relaxed whitespace-pre-line">{u.content}</p>
                <div className="text-xs text-gray-500">Posted by: {u.author_name}</div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
