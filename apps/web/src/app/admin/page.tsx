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
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { AdminOverview } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function AdminOverviewPage() {
  const router = useRouter();
  const { user, isCoordinator, isLoading } = useAuth();

  const [overview, setOverview] = useState<AdminOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isLoading && (!user || !isCoordinator)) {
      router.push("/login?redirect=/admin");
      return;
    }

    if (user && isCoordinator) {
      async function fetchOverview() {
        try {
          const data = await apiRequest<AdminOverview>("/admin/overview");
          setOverview(data);
        } catch (err: any) {
          setError(err.message || "Failed to load admin overview metrics");
        } finally {
          setLoading(false);
        }
      }
      fetchOverview();
    }
  }, [user, isCoordinator, isLoading, router]);

  if (isLoading || loading) return <Spinner text="Loading administration workspace..." />;
  if (!user || !overview) return null;

  const unverifiedQueue = overview.unverified_participations_queue ?? (overview as any).pending_reviews ?? 0;
  const draftQueue = overview.draft_venues_review_queue ?? 0;
  const appealsQueue = overview.pending_appeals_queue ?? 0;
  const totalMembers = overview.total_members ?? 0;
  const pendingVerifications = overview.pending_member_verifications ?? 0;
  const activeCoordinators = overview.active_coordinators ?? (overview as any).active_appointments ?? 0;
  const publishedVenues = overview.published_activities ?? (overview as any).published_cleanups ?? 0;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 space-y-8">
      {/* Role & Geographic Jurisdiction Context Banner */}
      <div className="bg-emerald-950 text-white rounded-2xl p-6 sm:p-8 shadow-sm flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border border-emerald-800">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-xs uppercase font-bold tracking-wider text-emerald-400">
              Administrative Context
            </span>
            <Badge variant="primary" size="sm">
              {(overview.current_role || user.primary_role || "ADMIN")
                .replace(/_/g, " ")
                .toUpperCase()}
            </Badge>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold flex items-center gap-2">
            <MapPin className="w-6 h-6 text-emerald-400" />
            {overview.current_jurisdiction_name ||
              (user.primary_role === "super_admin"
                ? "National Command (All Nigeria)"
                : "National Jurisdiction")}
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

      {/* "Needs Your Attention" Section */}
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
          <Clock className="w-5 h-5 text-emerald-700" />
          Needs Your Attention
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-3">
            <div className="flex justify-between items-start">
              <span className="text-xs text-gray-500 font-semibold uppercase">
                Participation Queue
              </span>
              <Badge variant={unverifiedQueue > 0 ? "warning" : "neutral"}>
                {unverifiedQueue} pending
              </Badge>
            </div>
            <div className="text-2xl font-black text-gray-900">
              {unverifiedQueue}
            </div>
            <p className="text-xs text-gray-600">
              Volunteer activity attendance submissions awaiting coordinator review.
            </p>
            <Link
              href="/admin/reviews"
              className="inline-flex items-center text-xs font-bold text-emerald-800 hover:text-emerald-950 pt-2"
            >
              Open Review Queue
              <ArrowRight className="w-3.5 h-3.5 ml-1" />
            </Link>
          </div>

          <div className="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-3">
            <div className="flex justify-between items-start">
              <span className="text-xs text-gray-500 font-semibold uppercase">
                Draft Cleanup Venues
              </span>
              <Badge variant={draftQueue > 0 ? "warning" : "neutral"}>
                {draftQueue} drafts
              </Badge>
            </div>
            <div className="text-2xl font-black text-gray-900">
              {draftQueue}
            </div>
            <p className="text-xs text-gray-600">
              Proposed local sanitation venues awaiting review and publication.
            </p>
            <Link
              href="/admin/activities"
              className="inline-flex items-center text-xs font-bold text-emerald-800 hover:text-emerald-950 pt-2"
            >
              Inspect & Publish Venues
              <ArrowRight className="w-3.5 h-3.5 ml-1" />
            </Link>
          </div>

          <div className="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-3">
            <div className="flex justify-between items-start">
              <span className="text-xs text-gray-500 font-semibold uppercase">
                Appeals Queue
              </span>
              <Badge variant={appealsQueue > 0 ? "danger" : "neutral"}>
                {appealsQueue} appeals
              </Badge>
            </div>
            <div className="text-2xl font-black text-gray-900">
              {appealsQueue}
            </div>
            <p className="text-xs text-gray-600">
              Rejected attendance claims appealed by members.
            </p>
            <Link
              href="/admin/reviews"
              className="inline-flex items-center text-xs font-bold text-emerald-800 hover:text-emerald-950 pt-2"
            >
              Review Appeals
              <ArrowRight className="w-3.5 h-3.5 ml-1" />
            </Link>
          </div>
        </div>
      </div>

      {/* Real Metric Totals */}
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
          <LayoutDashboard className="w-5 h-5 text-emerald-700" />
          Jurisdiction Metrics (Derived from Live Database)
        </h2>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Registered Members</span>
            <div className="text-xl font-bold text-gray-900">{totalMembers}</div>
            <span className="text-[10px] text-gray-400">Total movement members</span>
          </div>

          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Pending Verification</span>
            <div className="text-xl font-bold text-gray-900">{pendingVerifications}</div>
            <span className="text-[10px] text-gray-400">Awaiting contact verify</span>
          </div>

          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Active Coordinators</span>
            <div className="text-xl font-bold text-gray-900">{activeCoordinators}</div>
            <span className="text-[10px] text-gray-400">Authorized appointments</span>
          </div>

          <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm text-xs space-y-1">
            <span className="text-gray-500">Published Venues</span>
            <div className="text-xl font-bold text-gray-900">{publishedVenues}</div>
            <span className="text-[10px] text-gray-400">Confirmed activities</span>
          </div>
        </div>
      </div>

      {/* Admin Modules Navigation */}
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-gray-900">Administration Sections</h2>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
          <Link
            href="/admin/activities"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <Calendar className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Cleanups & Venue Management
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Create venue drafts, verify local facilities, reschedule instances, and publish activities for member sign-up.
            </p>
          </Link>

          <Link
            href="/admin/reviews"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <FileCheck2 className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Participation Review Queue
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Verify attendance claims, record reasons for rejections, resolve appeals, and manage First Participation badges.
            </p>
          </Link>

          <Link
            href="/admin/leadership"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <ShieldCheck className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Leadership & Appointments
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              View appointments by state and zone, track term dates, record handovers, and change active/acting/revoked statuses.
            </p>
          </Link>

          <Link
            href="/admin/finance"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <DollarSign className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Financial Documentation & Ledger
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Official double-entry fund movements, balanced general ledger, cash commitments, printable vouchers, and Super Admin clearances.
            </p>
          </Link>

          <Link
            href="/admin/inventory"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <Boxes className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Inventory & Material Distribution
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Manage operational movement supplies, track nationwide warehouses, allocate stock, monitor shipments in transit, and resolve discrepancies.
            </p>
          </Link>

          <Link
            href="/admin/marketplace"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <Store className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Ratels Marketplace
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Nationwide vendor governance, multi-seller product catalogue, order tracking, escrow reconciliations, and dispute arbitration.
            </p>
          </Link>

          <Link
            href="/admin/careers"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <Briefcase className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Ratels Careers & Job Opportunities
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Employer verification, job vacancy moderation, transparent recruitment oversight, and fair rules-based candidate matching.
            </p>
          </Link>

          <Link
            href="/admin/audit"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <History className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              System Audit History
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Inspect immutable audit records of administrative approvals, venue updates, and access controls.
            </p>
          </Link>

          <Link
            href="/admin/foundation"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <Building className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Foundation & Bank Accounts
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Configure Martins Vincent Otse Foundation settings and submit bank account versions for dual verification.
            </p>
          </Link>

          <Link
            href="/admin/hero"
            className="p-5 bg-white rounded-xl border border-gray-200 hover:border-emerald-500 shadow-sm space-y-2 group transition"
          >
            <div className="w-9 h-9 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
              <Film className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-gray-900 text-sm group-hover:text-emerald-800 transition">
              Homepage Hero Media
            </h3>
            <p className="text-xs text-gray-600 leading-relaxed">
              Upload, preview, publish, toggle, or remove the dynamic HTML5 video background for the public homepage hero section.
            </p>
          </Link>
        </div>
      </div>
    </div>
  );
}
