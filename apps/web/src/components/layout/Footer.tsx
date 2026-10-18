import React from "react";
import Link from "next/link";
import { Shield, ExternalLink } from "lucide-react";

export function Footer() {
  return (
    <footer className="bg-gray-900 text-gray-300 border-t border-gray-800 text-sm mt-auto">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
          {/* Movement identity */}
          <div className="md:col-span-2">
            <div className="flex items-center gap-2 mb-3">
              <div className="w-8 h-8 rounded bg-emerald-700 flex items-center justify-center text-white">
                <Shield className="w-4 h-4" />
              </div>
              <span className="font-bold text-white text-base">Nigerian Ratels Movement</span>
            </div>
            <p className="text-gray-400 text-xs leading-relaxed max-w-md mb-4">
              Building a coordinated grassroots movement for environmental sanitation, community
              service, humanitarian support, and social transparency across the 36 States and the FCT.
            </p>
            <div className="p-3 bg-gray-800/80 rounded-lg text-xs text-gray-400 border border-gray-700 leading-normal">
              <strong className="text-gray-200 block mb-1">Planning Baseline & Transparency:</strong>
              This platform implementation is based on Version 2.0 of the project blueprint. Role titles,
              sample metrics, and provisional Foundation content are configurable planning defaults
              pending official verified registration and leadership authorization.
            </div>
          </div>

          {/* Quick Navigation */}
          <div>
            <h4 className="text-white font-semibold text-xs uppercase tracking-wider mb-3">
              Platform Areas
            </h4>
            <ul className="space-y-2 text-xs">
              <li>
                <Link href="/activities" className="hover:text-emerald-400 transition">
                  Monthly Cleanups
                </Link>
              </li>
              <li>
                <Link href="/leadership" className="hover:text-emerald-400 transition">
                  Leadership Directory
                </Link>
              </li>
              <li>
                <Link href="/foundation" className="hover:text-emerald-400 transition">
                  Martins Vincent Otse Foundation
                </Link>
              </li>
              <li>
                <Link href="/about" className="hover:text-emerald-400 transition">
                  About the Movement
                </Link>
              </li>
              <li>
                <Link href="/register" className="hover:text-emerald-400 transition">
                  Join as Member
                </Link>
              </li>
            </ul>
          </div>

          {/* Planned Modules (Honest Roadmap) */}
          <div>
            <h4 className="text-white font-semibold text-xs uppercase tracking-wider mb-3">
              Roadmap (Planned)
            </h4>
            <ul className="space-y-2 text-xs text-gray-400">
              <li className="flex items-center gap-1.5">
                <span>Material Warehouses & Dispatch</span>
                <span className="text-[10px] px-1.5 py-0.2 bg-gray-800 text-gray-400 rounded">Phase 2</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>Media Vault & Moderation</span>
                <span className="text-[10px] px-1.5 py-0.2 bg-gray-800 text-gray-400 rounded">Phase 3</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>Community Requests & Projects</span>
                <span className="text-[10px] px-1.5 py-0.2 bg-gray-800 text-gray-400 rounded">Phase 4</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>Business Marketplace & Delivery</span>
                <span className="text-[10px] px-1.5 py-0.2 bg-gray-800 text-gray-400 rounded">Phase 5</span>
              </li>
              <li className="flex items-center gap-1.5">
                <span>Audited Fund Reconciliation</span>
                <span className="text-[10px] px-1.5 py-0.2 bg-gray-800 text-gray-400 rounded">Phase 7</span>
              </li>
            </ul>
          </div>
        </div>

        <div className="mt-8 pt-6 border-t border-gray-800 flex flex-col sm:flex-row justify-between items-center text-xs text-gray-500 gap-3">
          <div>
            © 2026 Nigerian Ratels Management System. Operational Phase 1.
          </div>
          <div className="flex gap-4">
            <span>Africa/Lagos Timezone</span>
            <span>•</span>
            <span>Nigeria Data Protection Act 2023 Principles</span>
          </div>
        </div>
      </div>
    </footer>
  );
}
