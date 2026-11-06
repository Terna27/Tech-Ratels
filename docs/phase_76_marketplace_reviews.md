# Ratels Marketplace — Approved Vendors, Verified Purchase Reviews

Implementation branch: `main`. Database migration: `20261106000027_marketplace_reviews.sql`.

## Vendor approval and storefront policy

- A vendor registers a business in `pending_review`.
- Only Super Admin can approve/reject, suspend, or restore a vendor via `/api/v1/marketplace/admin/businesses/{id}/{approve|suspend|restore}`.
- Vendor approval is verified by the Go service before new products, product updates, and product image uploads. Unapproved businesses are blocked immediately.
- A Super Admin can explicitly revoke a vendor through `POST /api/v1/marketplace/admin/businesses/{id}/revoke` with an administrative reason. Revoked businesses cannot publish or edit listings. The database constraint includes `revoked` via migration `20261106000028_marketplace_vendor_revocation.sql`.
- New published products and draft-to-published updates require a meaningful title, a description of 40–5000 characters, category, SKU, and 1–5 images. Incomplete historical published listings remain editable to avoid damaging legacy records.
- Public product detail endpoints hide nonpublished products and products belonging to businesses that are no longer approved.
- Deleting a product from the seller interface archives it rather than physically deleting order history and review records. Existing order management and subscriptions are preserved.
- Vendor image upload continues to use existing Cloudflare R2 support and server-side image inspection. A vendor can upload up to five product images.

## Customer ratings and reviews

Only authenticated customers with a **paid and delivered order** containing the reviewed product can submit a review. A buyer is allowed one review per product; purchases must be connected to the account. Server-side enforcement is not based on a client-supplied verified flag.

- `GET /api/v1/marketplace/products/{id}/reviews` — Public rating average, review count and up to 100 published reviews. Reviewer internal IDs are not exposed.
- `POST /api/v1/marketplace/products/{id}/reviews` — Authenticated verified buyer; rating 1–5, comment 10–2000 characters, up to 5 product photos. Duplicate review is rejected.
- `PUT /api/v1/marketplace/reviews/{id}` — Buyer may edit their own review within 30 days. Editing a moderated hidden review does not restore it.
- `POST /api/v1/marketplace/products/{id}/reviews/images` — Buyer photo upload using existing server-side JPEG/PNG/WebP validation (max 5 MB each), isolated R2 keys per buyer and product. Buyer must have an eligible order.
- `POST /api/v1/marketplace/reviews/{id}/reply` — Only owner of an approved vendor business may respond publicly.
- `POST /api/v1/marketplace/reviews/{id}/report` — Authenticated users can report inappropriate reviews once each.
- `GET /api/v1/marketplace/admin/reviews/reports` — Super Admin report queue.
- `PATCH /api/v1/marketplace/admin/reviews/{id}/moderate` — Super Admin can hide or restore a review (`{"status":"hidden"}` or `{"status":"published"}`).

Product detail pages display star averages, customer comments, uploaded photos and vendor responses; signed-in buyers may write and edit reviews, and vendors may reply. Super Admin moderation dashboard is at `/admin/marketplace/reviews`.

## Operations and security

- The migration creates only two new tables; does not drop existing member, product, order or payment tables.
- Paystack live checkout and store subscription charging remain disabled. **Do not enable payments** as part of this feature.
- No NGO workflows are added. Existing finance access restrictions are not modified by these marketplace changes.
- Review photos and customer feedback are public content and require normal abuse reporting/moderation.
- Review picture URLs are checked against the buyer/product-specific object storage prefix.
- Existing marketplace prices, inventory ledger, orders, subscriptions, and product quotas are retained.

## Testing and deployment

- A new integration test covers review ineligibility before payment, successful paid/delivered review, duplicate prevention, Super Admin-only moderation, hidden-review editing and restoration.
- GitHub Actions `Production checks` runs backend tests, frontend builds and container builds after commits.
- Verify the newest commit's Actions run before declaring the work ready for launch.
- Verify Neon migration 27 on the live database; do **not** presume successful application from local or CI migration tests alone.
- Test actual member and vendor accounts and Cloudflare R2 uploads in staging, including a buyer order, a simulated delivery, photo review, vendor reply, reporting and moderation.
- Production deployment and real payment integration are not claimed.

## Remaining follow-up work

- Expand specifications and product condition into structured fields (currently required as descriptive text within the product description).
- Add richer review sorting, pagination, and misuse/abuse detection rules. Current public review feed is limited to the newest 100 reviews.
- Verify the new Super Admin revocation action and its migration against staging and live accounts.
- Investigate active-subscription limits and publishing checks under concurrent vendor requests.

## Additional verified design changes — November 6, 2026

- Vendor revocation has a dedicated Super Admin endpoint and requires an administrative reason. Revoked vendors cannot create or update listings; legacy approval, suspension and restoration are blocked from silently reactivating them. Existing orders and reviews are preserved.
- New product publishing and draft-to-published transitions require meaningful titles, descriptions of 40–5000 characters, category, SKU, nonnegative stock and at least one product image. Existing published listings are grandfathered for editing to avoid destructive retroactive changes.
- Newly published product photos must use server-generated vendor-specific R2 object URLs, not external image URLs or images assigned to another seller.
- Public review listing now supports `sort=newest|oldest|highest|lowest|with_photos` and `page=1...` with a fixed 20 items per page and validated sort options; the returned rating average and count reflect all published reviews. The product page provides sorting and pagination controls.
- Review tests include sorting, pagination, and rejection of malformed sorting parameters. Image ownership validation also has unit tests for external URLs, cross-vendor URLs, invalid keys, and missing storage.
- Live checkout and live store subscription payments remain disabled. No NGO functionality is introduced. Existing administrative finance authorization was not changed during this marketplace work.

## Live acceptance still required

Confirm migration 27 (reviews) and migration 28 (vendor revocation) in the actual Neon production database, and verify the latest GitHub Actions run completes successfully. Test buyer photo upload and download, admin review hiding/restoration, vendor revocation, new listing publication, cross-vendor image rejection, and preservation of existing order history with designated accounts. Do not claim production readiness based on CI alone.
