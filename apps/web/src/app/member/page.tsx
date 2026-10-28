"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Calendar,
  MapPin,
  Clock,
  Award,
  CheckCircle2,
  Clock3,
  AlertCircle,
  ArrowRight,
  Shield,
  User as UserIcon,
} from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { apiRequest } from "@/lib/api";
import { EventInstance, BadgeAward, ParticipationClaim, EventRegistration } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Alert } from "@/components/ui/Alert";

export default function MemberDashboardPage() {
  const router = useRouter();
  const { user, isLoading } = useAuth();

  const [nextEvent, setNextEvent] = useState<EventInstance | null>(null);
  const [badges, setBadges] = useState<BadgeAward[]>([]);
  const [participations, setParticipations] = useState<ParticipationClaim[]>([]);
  const [isRegisteredNext, setIsRegisteredNext] = useState(false);
  const [loadingData, setLoadingData] = useState(true);

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login?redirect=/member");
      return;
    }

    if (user) {
      async function fetchDashboard() {
        try {
          const [events, myBadges, myClaims, myRegs] = await Promise.all([
            apiRequest<EventInstance[]>("/activities/events?status=published&limit=1"),
            apiRequest<BadgeAward[]>("/participations/badges/my"),
            apiRequest<ParticipationClaim[]>("/participations/my"),
            apiRequest<EventRegistration[]>("/activities/my-registrations"),
          ]);

          if (events && events.length > 0) {
            setNextEvent(events[0]);
            const registered = myRegs.some((r) => r.event_id === events[0].id && ["registered", "attended"].includes(r.status));
            setIsRegisteredNext(registered);
          }
          setBadges(myBadges || []);
          setParticipations(myClaims || []);
        } catch (err) {
          console.error("Failed to load dashboard data", err);
        } finally {
          setLoadingData(false);
        }
      }
      fetchDashboard();
    }
  }, [user, isLoading, router]);

  if (isLoading || loadingData) return <Spinner text="Loading member dashboard..." />;
  if (!user) return null;

  // Phase 68: registration is not verified attendance.
  const pendingClaims = participations.filter((p) => p.status === "pending" || p.status === "appealed");

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-8">
      {/* Welcome Card */}
      <div className="bg-gradient-to-r from-emerald-950 to-emerald-900 text-white rounded-2xl p-6 sm:p-8 shadow-sm flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <span className="text-xs uppercase font-bold tracking-wider text-emerald-300">
              Member Portal
            </span>
            <Badge variant="success" size="sm">
              {user.is_verified ? "Verified Member" : "Pending Verification"}
            </Badge>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold">{user.display_name}</h1>
          <p className="text-xs text-emerald-200">
            Membership Reference: <strong className="font-mono text-white">{user.membership_reference}</strong>
          </p>
        </div>

        <div className="flex gap-2">
          <Link
            href="/member/profile"
            className="px-4 py-2 bg-emerald-800 hover:bg-emerald-700 text-white text-xs font-semibold rounded-lg transition"
          >
            Edit Profile
          </Link>
          <Link
            href="/member/participations"
            className="px-4 py-2 bg-emerald-400 hover:bg-emerald-300 text-emerald-950 text-xs font-bold rounded-lg transition"
          >
            Submit Evidence
          </Link>
        </div>
      </div>

      {/* Main 2-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Left Column: Next Cleanup & Pending Reviews (2 spans) */}
        <div className="lg:col-span-2 space-y-8">
          {/* Next Local Cleanup Highlight */}
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
            <div className="flex justify-between items-center">
              <h2 className="font-bold text-gray-900 text-lg flex items-center gap-2">
                <Calendar className="w-5 h-5 text-emerald-700" />
                Next Monthly Cleanup
              </h2>
              <span className="text-xs text-gray-500">Lagos Time (UTC+1)</span>
            </div>

            {nextEvent ? (
              <div className="p-5 bg-gray-50 rounded-xl border border-gray-200 space-y-4">
                <div className="flex justify-between items-start">
                  <div>
                    <h3 className="font-bold text-gray-900 text-base">{nextEvent.title}</h3>
                    <div className="text-xs text-gray-600 mt-1 flex items-center gap-1.5">
                      <MapPin className="w-4 h-4 text-emerald-700" />
                      <span>{nextEvent.address}, {nextEvent.landmark} ({nextEvent.state_name})</span>
                    </div>
                  </div>
                  <Badge variant={isRegisteredNext ? "success" : "neutral"} size="sm">
                    {isRegisteredNext ? "Registered" : "Not Registered"}
                  </Badge>
                </div>

                <div className="flex flex-wrap gap-4 text-xs text-gray-600 border-t border-gray-200 pt-3">
                  <div className="flex items-center gap-1.5">
                    <Clock className="w-4 h-4 text-emerald-700" />
                    <span>
                      {new Date(nextEvent.start_time).toLocaleDateString("en-NG", {
                        weekday: "short",
                        year: "numeric",
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>
                  <div>Slots: {nextEvent.registered_count} / {nextEvent.capacity}</div>
                </div>

                <div className="flex justify-between items-center pt-2">
                  <Link
                    href={`/activities/${nextEvent.id}`}
                    className="text-xs font-semibold text-emerald-800 hover:text-emerald-950 flex items-center gap-1"
                  >
                    View Full Venue & Safety Notes
                    <ArrowRight className="w-3.5 h-3.5" />
                  </Link>

                  {isRegisteredNext ? (
                    <span className="text-xs text-emerald-700 font-semibold flex items-center gap-1">
                      <CheckCircle2 className="w-4 h-4" /> Registration recorded — attendance requires coordinator verification
                    </span>
                  ) : (
                    <Link
                      href={`/activities/${nextEvent.id}`}
                      className="px-4 py-2 bg-emerald-800 hover:bg-emerald-900 text-white rounded-lg text-xs font-semibold transition"
                    >
                      Register Now
                    </Link>
                  )}
                </div>
              </div>
            ) : (
              <div className="text-xs text-gray-500 py-6 text-center border rounded-xl bg-gray-50">
                No active cleanups scheduled currently. Check back soon.
              </div>
            )}
          </div>

          {/* Pending Reviews Queue */}
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
            <h2 className="font-bold text-gray-900 text-lg flex items-center gap-2">
              <Clock3 className="w-5 h-5 text-emerald-700" />
              Pending Participation Submissions
            </h2>

            {pendingClaims.length === 0 ? (
              <div className="p-4 bg-gray-50 rounded-xl text-center text-xs text-gray-500">
                You have no participation evidence awaiting review.
              </div>
            ) : (
              <div className="space-y-3">
                {pendingClaims.map((claim) => (
                  <div
                    key={claim.id}
                    className="p-4 bg-gray-50 rounded-xl border border-gray-200 text-xs space-y-2"
                  >
                    <div className="flex justify-between items-start">
                      <span className="font-semibold text-gray-900 text-sm">
                        {claim.event_title || "Sanitation Activity"}
                      </span>
                      <Badge variant={claim.status === "appealed" ? "warning" : "info"} size="sm">
                        {claim.status === "appealed" ? "Appeal Pending" : "Coordinator Review Pending"}
                      </Badge>
                    </div>
                    <p className="text-gray-600 line-clamp-2">{claim.description}</p>
                    <div className="text-[11px] text-gray-400">
                      Submitted on: {new Date(claim.submitted_at).toLocaleDateString()}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right Column: Badges & Recognition (1 span) */}
        <div className="space-y-8">
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-4">
            <h2 className="font-bold text-gray-900 text-lg flex items-center gap-2">
              <Award className="w-5 h-5 text-emerald-700" />
              Earned Badges
            </h2>

            {badges.length === 0 ? (
              <div className="p-6 bg-gray-50 rounded-xl border border-dashed border-gray-200 text-center space-y-2">
                <Award className="w-8 h-8 text-gray-300 mx-auto" />
                <h3 className="font-semibold text-xs text-gray-700">No Badges Yet</h3>
                <p className="text-[11px] text-gray-500">
                  Attend your first monthly cleanup and have your participation verified by a coordinator to earn the &ldquo;First Participation&rdquo; badge!
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {badges.map((b) => (
                  <div
                    key={b.id}
                    className="p-4 rounded-xl border border-emerald-100 bg-emerald-50/60 flex items-start gap-3 text-xs"
                  >
                    <div className="w-8 h-8 rounded-lg bg-emerald-700 text-white flex items-center justify-center flex-shrink-0 mt-0.5">
                      <Award className="w-5 h-5 text-emerald-200" />
                    </div>
                    <div>
                      <h4 className="font-bold text-emerald-950 text-sm">{b.badge_name}</h4>
                      <p className="text-emerald-800 text-xs mt-0.5">{b.badge_description}</p>
                      <div className="text-[10px] text-emerald-600 mt-1">
                        Awarded on {new Date(b.awarded_at).toLocaleDateString()}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Quick Links Card */}
          <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm space-y-3 text-xs">
            <h3 className="font-bold text-gray-900 text-sm">Quick Actions</h3>
            <ul className="space-y-2">
              <li>
                <Link
                  href="/activities"
                  className="block p-2.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 font-medium"
                >
                  Browse Monthly Cleanups
                </Link>
              </li>
              <li>
                <Link
                  href="/member/participations"
                  className="block p-2.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 font-medium"
                >
                  Participation History & Appeals
                </Link>
              </li>
              <li>
                <Link
                  href="/marketplace"
                  className="block p-2.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 font-medium"
                >
                  Ratels Marketplace
                </Link>
              </li>
              <li>
                <Link
                  href="/careers"
                  className="block p-2.5 rounded-lg border border-emerald-200 bg-emerald-50/50 hover:bg-emerald-50 text-emerald-900 font-medium"
                >
                  Ratels Careers & Vacancies
                </Link>
              </li>
              <li>
                <Link
                  href="/careers/profile"
                  className="block p-2.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 font-medium"
                >
                  My Candidate Profile
                </Link>
              </li>
              <li>
                <Link
                  href="/careers/applications"
                  className="block p-2.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 font-medium"
                >
                  My Applications & Interviews
                </Link>
              </li>
              <li>
                <Link
                  href="/foundation"
                  className="block p-2.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 font-medium"
                >
                  Martins Vincent Otse Foundation
                </Link>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
