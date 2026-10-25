import React from "react";
import { Shield, MapPin, CheckCircle2, AlertTriangle, Calendar } from "lucide-react";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";

export default function AboutPage() {
  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-10">
      <div className="space-y-4">
        <Badge variant="primary" size="md">About the Platform</Badge>
        <h1 className="text-3xl font-extrabold text-gray-900 tracking-tight">
          Ratels Worldwide Platform
        </h1>
        <p className="text-gray-600 leading-relaxed">
          The operational technology platform designed to support the Ratels Worldwide movement associated
          in the project brief with Chief VeryDarkMan (VDM). Built to provide simple, mobile-friendly tools
          for ordinary members while enforcing transparent, scoped administration for coordinators.
        </p>
      </div>

      <Alert type="warning" title="Planning Notice & Unverified Content">
        This platform represents the implementation of Blueprint Version 2.0. All role titles, provisional
        Foundation names, sample data, and approval thresholds are configurable architectural baselines.
        Official legal entity status, verified leadership rosters, and registered bank accounts will be
        confirmed upon presentation of verified regulatory documents before production release.
      </Alert>

      {/* Geopolitical Model */}
      <section className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-4">
        <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
          <MapPin className="w-5 h-5 text-emerald-700" />
          Geographic Structure
        </h2>
        <p className="text-gray-600 leading-relaxed">
          The Ratels platform rejects free-text location entries in favor of an authoritative relational hierarchy:
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
          <div className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-1.5">
            <div className="font-semibold text-gray-900 text-sm">6 Geopolitical Zones</div>
            <div className="text-gray-600">
              North Central, North East, North West, South East, South South, South West.
            </div>
          </div>
          <div className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-1.5">
            <div className="font-semibold text-gray-900 text-sm">36 States + 1 Federal Territory</div>
            <div className="text-gray-600">
              The Federal Capital Territory (FCT) is strictly modeled as a federal territory with 6 Area Councils,
              not as a 37th state.
            </div>
          </div>
          <div className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-1.5">
            <div className="font-semibold text-gray-900 text-sm">LGAs & Area Councils</div>
            <div className="text-gray-600">
              Validated parent relationships reference National Bureau of Statistics administrative data with
              recorded revision dates.
            </div>
          </div>
          <div className="p-4 bg-gray-50 rounded-xl border border-gray-200 space-y-1.5">
            <div className="font-semibold text-gray-900 text-sm">Distinct Capital Cities & Venues</div>
            <div className="text-gray-600">
              State capitals and cleanup venues are decoupled from local areas, allowing multiple simultaneous
              venues in one metropolitan area.
            </div>
          </div>
        </div>
      </section>

      {/* Monthly Cleanups Operating Model */}
      <section className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-4">
        <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
          <Calendar className="w-5 h-5 text-emerald-700" />
          The Last Saturday Sanitation Model
        </h2>
        <p className="text-gray-600 leading-relaxed">
          National cleanup campaigns occur on the last Saturday of each month, mathematically computed
          in the <code>Africa/Lagos</code> timezone (UTC+1). Local representatives propose venues in draft status;
          only venues inspected and confirmed by the responsible coordinator are published for member registration.
        </p>
      </section>

      {/* Phased Roadmap */}
      <section className="bg-white p-6 sm:p-8 rounded-2xl border border-gray-200 shadow-sm space-y-4">
        <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
          <Shield className="w-5 h-5 text-emerald-700" />
          Delivery Roadmap
        </h2>
        <div className="space-y-3">
          <div className="p-4 border border-emerald-200 bg-emerald-50/50 rounded-xl flex items-start gap-3">
            <CheckCircle2 className="w-5 h-5 text-emerald-700 flex-shrink-0 mt-0.5" />
            <div>
              <div className="font-semibold text-sm text-emerald-950">
                Phase 1: Operational Foundation (Current Release)
              </div>
              <p className="text-xs text-emerald-800 leading-relaxed mt-1">
                PostgreSQL persistence, 6 zones, 36 states, FCT territory, Area Councils, authentication with
                rate limiting and session invalidation, scoped coordinator permissions, monthly cleanups
                engine, participation reviews, First Participation badge, and Foundation preparation.
              </p>
            </div>
          </div>

          <div className="p-4 border border-gray-200 bg-gray-50 rounded-xl space-y-2 text-xs text-gray-600">
            <div className="font-semibold text-sm text-gray-900">Upcoming Phases:</div>
            <ul className="list-disc list-inside space-y-1">
              <li>Phase 2: Controlled Material Inventory, Warehouses, Allocations, and Discrepancy Tracking.</li>
              <li>Phase 3: Secure S3 Object Storage, Metadata Stripping, and Moderated Media Galleries.</li>
              <li>Phase 4: Community Support Requests, Project Milestones, and Balanced Double-Entry Ledgers.</li>
              <li>Phase 5: Member Business Directory, Branch Fulfilment, and Single-Seller Paid Pilot.</li>
              <li>Phase 6: Private Career Vault, Vacancies, and Explainable Transparent Matching.</li>
              <li>Phase 7: Martins Vincent Otse Foundation Bank Matching, Donation Claims, and Audited Receipts.</li>
              <li>Phase 8: Native Android and iOS applications utilizing the identical versioned REST API.</li>
            </ul>
          </div>
        </div>
      </section>
    </div>
  );
}
