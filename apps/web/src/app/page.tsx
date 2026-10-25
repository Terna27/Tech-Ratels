"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  Calendar,
  MapPin,
  ShieldCheck,
  Award,
  ArrowRight,
  Heart,
  Users,
  CheckCircle,
  Clock,
  Sparkles,
  Camera,
  Play,
  Video,
  BarChart3,
  FileText,
} from "lucide-react";
import { apiRequest } from "@/lib/api";
import { EventInstance, PublicLeader, HeroMedia, ActiveHeroMediaResponse, GalleryMedia, PublicOverviewData } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";

export default function HomePage() {
  const [featuredEvent, setFeaturedEvent] = useState<EventInstance | null>(null);
  const [leaders, setLeaders] = useState<PublicLeader[]>([]);
  const [heroMedia, setHeroMedia] = useState<HeroMedia | null>(null);
  const [featuredMedia, setFeaturedMedia] = useState<GalleryMedia[]>([]);
  const [impactMetrics, setImpactMetrics] = useState<PublicOverviewData | null>(null);
  const [videoPlaybackError, setVideoPlaybackError] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (typeof window !== "undefined") {
      const mediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
      setReducedMotion(mediaQuery.matches);
      const listener = (e: MediaQueryListEvent) => setReducedMotion(e.matches);
      mediaQuery.addEventListener?.("change", listener);
      return () => mediaQuery.removeEventListener?.("change", listener);
    }
  }, []);

  useEffect(() => {
    async function loadData() {
      try {
        const events = await apiRequest<EventInstance[]>("/activities/events?limit=1&status=published");
        if (events && events.length > 0) {
          setFeaturedEvent(events[0]);
        }
        const dir = await apiRequest<PublicLeader[]>("/leadership/directory");
        if (dir) {
          setLeaders(dir.slice(0, 4));
        }
        try {
          const heroRes = await apiRequest<ActiveHeroMediaResponse>("/homepage/hero-media");
          if (heroRes?.active && heroRes.media) {
            setHeroMedia(heroRes.media);
          }
        } catch (_) {
          // Graceful fallback to default green gradient
        }
        try {
          const media = await apiRequest<GalleryMedia[]>("/gallery/featured?limit=4");
          if (media && Array.isArray(media)) {
            setFeaturedMedia(media);
          }
        } catch (_) {
          // Graceful fallback if no media exists yet
        }
        try {
          const overview = await apiRequest<PublicOverviewData>("/public/reports/overview");
          if (overview) {
            setImpactMetrics(overview);
          }
        } catch (_) {
          // Graceful fallback if no report published yet
        }
      } catch (err) {
        console.error("Failed to load initial data", err);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, []);

  return (
    <div className="space-y-12 pb-16">
      {/* Hero Section */}
      <section className="relative bg-gradient-to-b from-emerald-950 via-emerald-900 to-gray-900 text-white pt-16 pb-20 px-4 sm:px-6 lg:px-8 border-b border-emerald-800 overflow-hidden">
        {/* Dynamic HTML5 Video Background (Admin-managed via Cloudflare R2) */}
        {heroMedia && !reducedMotion && (
          <div className="absolute inset-0 z-0 overflow-hidden pointer-events-none" aria-hidden="true">
            <video
              autoPlay
              muted
              loop
              playsInline
              preload="metadata"
              poster={heroMedia.poster_url || undefined}
              onError={() => setVideoPlaybackError(true)}
              className={`w-full h-full object-cover object-center transition-opacity duration-500 ${
                videoPlaybackError ? "opacity-0" : "opacity-100"
              }`}
            >
              <source src={heroMedia.video_url} type={heroMedia.content_type || "video/mp4"} />
            </video>
            {/* Dark overlay for contrast and WCAG readability */}
            <div className="absolute inset-0 bg-gradient-to-b from-black/70 via-black/55 to-black/85" />
          </div>
        )}

        {/* Static poster fallback for reduced motion preference */}
        {reducedMotion && heroMedia?.poster_url && (
          <div
            className="absolute inset-0 z-0 bg-cover bg-center pointer-events-none"
            style={{ backgroundImage: `url(${heroMedia.poster_url})` }}
            aria-hidden="true"
          >
            <div className="absolute inset-0 bg-gradient-to-b from-black/70 via-black/55 to-black/85" />
          </div>
        )}

        <div className="relative z-10 max-w-5xl mx-auto text-center space-y-6">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-800/80 border border-emerald-700 text-emerald-200 text-xs font-semibold">
            <Sparkles className="w-3.5 h-3.5 text-emerald-300" />
            <span>Ratels Worldwide Platform • Civic Action, Transparency & Operations</span>
          </div>

          <h1 className="text-3xl sm:text-5xl font-extrabold tracking-tight leading-tight">
            Grassroots Action, Transparent Leadership, <br />
            <span className="text-emerald-400">Cleaner Communities.</span>
          </h1>

          <p className="text-base sm:text-lg text-emerald-100/90 max-w-2xl mx-auto leading-relaxed">
            Organizing cleanups every last Saturday of the month across Nigeria&apos;s 36 States and the FCT.
            Track participation, verify appointments, and build civic responsibility together.
          </p>

          <div className="flex flex-col sm:flex-row justify-center gap-3 pt-4">
            <Link
              href="/register"
              className="inline-flex items-center justify-center px-6 py-3.5 text-base font-semibold rounded-xl text-emerald-950 bg-emerald-400 hover:bg-emerald-300 shadow-md transition"
            >
              Join Ratels Worldwide
              <ArrowRight className="w-4 h-4 ml-2" />
            </Link>
            <Link
              href="/activities"
              className="inline-flex items-center justify-center px-6 py-3.5 text-base font-medium rounded-xl text-white bg-emerald-900/80 border border-emerald-700 hover:bg-emerald-800 transition"
            >
              Explore Next Cleanups
            </Link>
          </div>
        </div>
      </section>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 space-y-12">
        {/* Next National Cleanup Banner */}
        <section className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 shadow-sm">
          <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-6">
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Badge variant="success" size="md">
                  <Calendar className="w-3.5 h-3.5 mr-1" />
                  Monthly Cleanup Campaign
                </Badge>
                <span className="text-xs text-gray-500 font-medium">Lagos Time (UTC+1)</span>
              </div>

              <h2 className="text-xl sm:text-2xl font-bold text-gray-900">
                {featuredEvent ? featuredEvent.title : "Upcoming Nationwide Environmental Sanitation"}
              </h2>

              <p className="text-sm text-gray-600 max-w-2xl leading-relaxed">
                {featuredEvent
                  ? `Gathering at ${featuredEvent.address}, ${featuredEvent.landmark}. Coordinated under ${featuredEvent.state_name} State chapter.`
                  : "Every last Saturday of the month, Ratels volunteers mobilize across Nigerian communities. Venues are published once verified by authorized coordinators."}
              </p>

              {featuredEvent && (
                <div className="flex flex-wrap gap-4 pt-2 text-xs text-gray-700 font-medium">
                  <div className="flex items-center gap-1.5 bg-gray-50 px-2.5 py-1.5 rounded-md border border-gray-200">
                    <Clock className="w-4 h-4 text-emerald-700" />
                    <span>
                      {new Date(featuredEvent.start_time).toLocaleDateString("en-NG", {
                        weekday: "short",
                        year: "numeric",
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 bg-gray-50 px-2.5 py-1.5 rounded-md border border-gray-200">
                    <MapPin className="w-4 h-4 text-emerald-700" />
                    <span>{featuredEvent.state_name} • {featuredEvent.lga_name}</span>
                  </div>
                  <div className="flex items-center gap-1.5 bg-gray-50 px-2.5 py-1.5 rounded-md border border-gray-200">
                    <Users className="w-4 h-4 text-emerald-700" />
                    <span>Capacity: {featuredEvent.registered_count} / {featuredEvent.capacity}</span>
                  </div>
                </div>
              )}
            </div>

            <div className="flex-shrink-0 flex flex-col sm:flex-row gap-3">
              {featuredEvent ? (
                <Link
                  href={`/activities/${featuredEvent.id}`}
                  className="px-5 py-3 text-center text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 rounded-xl transition shadow-sm"
                >
                  View Venue & Register
                </Link>
              ) : (
                <Link
                  href="/activities"
                  className="px-5 py-3 text-center text-sm font-semibold text-white bg-emerald-800 hover:bg-emerald-900 rounded-xl transition shadow-sm"
                >
                  Browse Cleanup Schedule
                </Link>
              )}
            </div>
          </div>
        </section>

        {/* Transparency & Impact Section */}
        <section className="bg-gradient-to-br from-emerald-950 via-emerald-900 to-gray-950 text-white rounded-2xl p-6 sm:p-8 space-y-6 border border-emerald-800 shadow-sm">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <span className="p-1.5 rounded-lg bg-emerald-800/80 text-emerald-300">
                  <BarChart3 className="w-4 h-4" />
                </span>
                <span className="text-xs font-bold text-emerald-300 uppercase tracking-wider">
                  Public Transparency & Impact
                </span>
              </div>
              <h3 className="text-xl sm:text-2xl font-extrabold text-white">
                Measurable Achievements Across Nigeria
              </h3>
              <p className="text-xs text-emerald-200/80">
                Certified metrics derived directly from verified cleanup operations, approved claims, and audited ledgers.
              </p>
            </div>
            <Link
              href="/reports"
              className="inline-flex items-center gap-1.5 px-4 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold rounded-xl transition shadow-sm self-start sm:self-auto"
            >
              <FileText className="w-4 h-4" />
              <span>Explore Public Reports Portal</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </div>

          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Completed Cleanups</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.completed_cleanups_count ?? 0}
              </div>
              <div className="text-[10px] text-emerald-400">Verified community venues</div>
            </div>

            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Verified Attendees</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.verified_attendees_count ?? 0}
              </div>
              <div className="text-[10px] text-emerald-400">
                {impactMetrics?.unique_participants_count ?? 0} unique members
              </div>
            </div>

            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Completed Projects</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.completed_projects_count ?? 0}
              </div>
              <div className="text-[10px] text-emerald-400">
                {impactMetrics?.approved_projects_count ?? 0} approved across LGAs
              </div>
            </div>

            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Approved Revenue</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.approved_income_formatted || "₦0.00"}
              </div>
              <div className="text-[10px] text-emerald-400">Super Admin certified ledger</div>
            </div>
          </div>
        </section>

        {/* 4 Pillars Grid */}
        <section className="space-y-6">
          <div className="text-center max-w-2xl mx-auto space-y-2">
            <h3 className="text-2xl font-bold text-gray-900">Built for Grassroots Accountability</h3>
            <p className="text-sm text-gray-600">
              Clear geographic structure from national oversight down to local government areas and area councils.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm space-y-3">
              <div className="w-10 h-10 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
                <Calendar className="w-5 h-5" />
              </div>
              <h4 className="font-semibold text-gray-900">Monthly Cleanups</h4>
              <p className="text-xs text-gray-600 leading-relaxed">
                Calculated dynamically for every last Saturday in Africa/Lagos. Multiple verified venues in
                state capitals and local areas.
              </p>
            </div>

            <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm space-y-3">
              <div className="w-10 h-10 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
                <ShieldCheck className="w-5 h-5" />
              </div>
              <h4 className="font-semibold text-gray-900">Scoped Leadership</h4>
              <p className="text-xs text-gray-600 leading-relaxed">
                Named coordinators with verified appointments. Strict geographic boundaries ensure coordinators
                act only within their assigned jurisdiction.
              </p>
            </div>

            <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm space-y-3">
              <div className="w-10 h-10 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
                <Award className="w-5 h-5" />
              </div>
              <h4 className="font-semibold text-gray-900">Verified Badges</h4>
              <p className="text-xs text-gray-600 leading-relaxed">
                Earn recognition for completed activities. Coordinators verify participation with anti-self-approval
                safeguards and an appeals process.
              </p>
            </div>

            <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm space-y-3">
              <div className="w-10 h-10 rounded-lg bg-emerald-50 text-emerald-800 flex items-center justify-center font-bold">
                <Heart className="w-5 h-5" />
              </div>
              <h4 className="font-semibold text-gray-900">Martins Vincent Otse Foundation</h4>
              <p className="text-xs text-gray-600 leading-relaxed">
                Official Foundation fundraising preparation. Bank accounts require dual verification before
                publication. Live payments inactive in Phase 1.
              </p>
            </div>
          </div>
        </section>

        {/* Transparency & Impact Section */}
        <section className="bg-gradient-to-br from-emerald-950 via-emerald-900 to-gray-950 text-white rounded-2xl p-6 sm:p-8 space-y-6 border border-emerald-800 shadow-sm">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <span className="p-1.5 rounded-lg bg-emerald-800/80 text-emerald-300">
                  <BarChart3 className="w-4 h-4" />
                </span>
                <span className="text-xs font-bold text-emerald-300 uppercase tracking-wider">
                  Public Transparency & Impact
                </span>
              </div>
              <h3 className="text-xl sm:text-2xl font-extrabold text-white">
                Measurable Achievements Across Nigeria
              </h3>
              <p className="text-xs text-emerald-200/80">
                Certified metrics derived directly from verified cleanup operations, approved claims, and audited ledgers.
              </p>
            </div>
            <Link
              href="/reports"
              className="inline-flex items-center gap-1.5 px-4 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold rounded-xl transition shadow-sm self-start sm:self-auto"
            >
              <FileText className="w-4 h-4" />
              <span>Explore Public Reports Portal</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </div>

          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Completed Cleanups</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.completed_cleanups_count ?? 0}
              </div>
              <div className="text-[10px] text-emerald-400">Verified community venues</div>
            </div>

            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Verified Attendees</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.verified_attendees_count ?? 0}
              </div>
              <div className="text-[10px] text-emerald-400">
                {impactMetrics?.unique_participants_count ?? 0} unique members
              </div>
            </div>

            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Completed Projects</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.completed_projects_count ?? 0}
              </div>
              <div className="text-[10px] text-emerald-400">
                {impactMetrics?.approved_projects_count ?? 0} approved across LGAs
              </div>
            </div>

            <div className="bg-emerald-900/40 border border-emerald-700/50 rounded-xl p-4 space-y-1">
              <div className="text-xs text-emerald-300 font-medium">Approved Revenue</div>
              <div className="text-2xl sm:text-3xl font-extrabold text-white">
                {impactMetrics?.approved_income_formatted || "₦0.00"}
              </div>
              <div className="text-[10px] text-emerald-400">Super Admin certified ledger</div>
            </div>
          </div>
        </section>

        {/* Featured Cleanup Event Media Section */}
        <section className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <span className="p-1.5 rounded-lg bg-emerald-100 text-emerald-800">
                  <Camera className="w-4 h-4" />
                </span>
                <span className="text-xs font-bold text-emerald-800 uppercase tracking-wider">
                  Verified In Action
                </span>
              </div>
              <h3 className="text-xl font-bold text-gray-900 mt-1">Recent Cleanup Media</h3>
              <p className="text-xs text-gray-600">
                Approved photographs and video highlights from volunteer cleanups across Nigeria.
              </p>
            </div>
            <Link
              href="/gallery"
              className="inline-flex items-center text-xs font-semibold text-emerald-800 hover:text-emerald-950"
            >
              Explore Full Media Gallery
              <ArrowRight className="w-3.5 h-3.5 ml-1" />
            </Link>
          </div>

          {featuredMedia.length > 0 ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {featuredMedia.map((m) => (
                <Link
                  key={m.id}
                  href="/gallery"
                  className="group bg-gray-50 rounded-xl border border-gray-200 overflow-hidden hover:shadow-md transition flex flex-col"
                >
                  <div className="relative aspect-video bg-gray-900 overflow-hidden">
                    {m.thumbnail_url || m.public_url ? (
                      <img
                        src={m.thumbnail_url || m.public_url}
                        alt={m.caption}
                        loading="lazy"
                        className="w-full h-full object-cover group-hover:scale-105 transition duration-300"
                      />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center text-gray-400">
                        <Camera className="w-6 h-6" />
                      </div>
                    )}
                    {m.media_type === "video" && (
                      <div className="absolute top-2 left-2 bg-black/70 text-white px-2 py-0.5 rounded text-[10px] font-bold flex items-center gap-1">
                        <Play className="w-2.5 h-2.5 fill-current text-emerald-400" />
                        <span>VIDEO</span>
                      </div>
                    )}
                  </div>
                  <div className="p-3 flex-1 flex flex-col justify-between">
                    <h4 className="font-bold text-xs text-gray-900 line-clamp-2 leading-snug group-hover:text-emerald-800 transition">
                      {m.caption}
                    </h4>
                    <div className="pt-2 text-[10px] text-gray-500 flex items-center justify-between">
                      <span className="truncate">{m.state_name || m.lga_name || "Nigeria"}</span>
                      {m.published_at && (
                        <span>{new Date(m.published_at).toLocaleDateString("en-GB", { month: "short", day: "numeric" })}</span>
                      )}
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          ) : (
            <div className="bg-emerald-50/50 rounded-xl border border-emerald-100 p-6 text-center space-y-3">
              <p className="text-xs text-emerald-950 font-medium">
                Our volunteers are capturing nationwide cleanup operations in real time.
              </p>
              <Link
                href="/gallery"
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-emerald-800 text-white rounded-lg text-xs font-bold hover:bg-emerald-900 transition"
              >
                <span>Browse Approved Gallery</span>
                <ArrowRight className="w-3.5 h-3.5" />
              </Link>
            </div>
          )}
        </section>

        {/* Leadership Directory Highlight */}
        <section className="bg-white rounded-2xl border border-gray-200 p-6 sm:p-8 space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h3 className="text-xl font-bold text-gray-900">Leadership Structure</h3>
              <p className="text-xs text-gray-600 mt-1">
                Displaying authorized appointments. Vacant jurisdictions honestly show &ldquo;Appointment pending&rdquo;.
              </p>
            </div>
            <Link
              href="/leadership"
              className="inline-flex items-center text-xs font-semibold text-emerald-800 hover:text-emerald-950"
            >
              View Full 36 States + FCT Directory
              <ArrowRight className="w-3.5 h-3.5 ml-1" />
            </Link>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {leaders.map((leader, idx) => (
              <div
                key={idx}
                className="p-4 rounded-xl border border-gray-100 bg-gray-50/50 space-y-2 text-xs"
              >
                <div className="font-semibold text-gray-900 text-sm truncate">{leader.role_title}</div>
                <div className="text-gray-600 font-medium">{leader.jurisdiction_name}</div>
                <div className="pt-2 flex items-center justify-between">
                  <span className="text-gray-800 font-semibold">{leader.display_name}</span>
                  <Badge variant={leader.status === "active" ? "success" : "warning"} size="sm">
                    {leader.status}
                  </Badge>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Data Protection and Transparency Alert */}
        <Alert type="info" title="Privacy and Administrative Safeguards">
          The Ratels Worldwide platform complies with Nigeria Data Protection Act (NDPA) 2023 principles.
          Personal CVs, private contact details, and unverified bank accounts are strictly protected from public exposure.
          Privileged coordinator roles cannot be self-selected during registration.
        </Alert>
      </div>
    </div>
  );
}
