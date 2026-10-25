import type { Metadata } from "next";
import "./globals.css";
import { AuthProvider } from "@/lib/auth-context";
import { CartProvider } from "@/lib/cart-context";
import { AppShell } from "@/components/layout/AppShell";

export const metadata: Metadata = {
  title: "Ratels Worldwide — Civic Action, Transparency & Operations Platform",
  description: "Operational management platform for the Ratels Worldwide movement across Nigeria and global networks",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="h-full">
      <body className="min-h-screen bg-gray-50 text-gray-900 antialiased font-sans">
        <AuthProvider>
          <CartProvider>
            <AppShell>{children}</AppShell>
          </CartProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
