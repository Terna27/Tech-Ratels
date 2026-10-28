export type StorePlan = "basic" | "business";
export type BillingInterval = "monthly" | "yearly";
export function storePrice(plan: StorePlan, interval: BillingInterval): number {
  return ({ basic: { monthly: 200000, yearly: 2000000 }, business: { monthly: 500000, yearly: 5000000 } })[plan][interval];
}
export function formatKobo(kobo: number): string {
  return new Intl.NumberFormat("en-NG", { style: "currency", currency: "NGN", maximumFractionDigits: 0 }).format(kobo / 100);
}
export function verifiedPaymentMessage(status: string): string {
  if (status === "successful") return "Payment verified. Your paid store term is confirmed.";
  if (status === "failed") return "Payment failed. You can retry from your store dashboard.";
  return "Payment pending. Your subscription has not been activated by this checkout.";
}
