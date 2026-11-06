# Member profile photographs and covers

Baseline: `2fd4a24bcacaf6cb803502acabf2e0416bd26a15` on main, 6 November 2026.

## Audit and changes

The baseline already provided authenticated owner-scoped R2 uploads, server-side JPEG/PNG/WebP inspection, a 5 MiB upload limit, pixel and minimum-dimension limits, JPEG re-encoding to remove metadata, unique server-generated storage keys, transactional image replacement/removal, audit events, private cover delivery, and automatic card photo replacement. Existing Super Admin and coordinator boundaries and the public QR verification allowlist are retained.

This update closes the profile API's arbitrary avatar URL replacement path. Existing clients may send their current unchanged URL, but image changes must use the validated media endpoints. The frontend omits avatar references when saving profile text, preventing stale forms from restoring or clearing newer uploads. Legacy photo values remain stored.

Crop previews now use the selected anchor as the zoom origin. Cover previews maintain the same 3:1 aspect ratio as the uploaded crop. Saved avatar positioning and zoom are included in the authenticated card response and applied in both the displayed/printed card and downloadable HTML card.

The profile header displays the member reference, location and biography, with a circular avatar, neutral cover fallback and camera controls. File controls have accessible names and keyboard focus indicators. Members can cancel image selection, see actual upload progress and processing status, and receive server error messages. Position sliders are disabled during mutations. Multipart temporary files are removed after processing. The shared image inspector now enforces the caller's byte limit.

## Data and privacy

No new migration is required. Deploy the existing `apps/api/migrations/20261106000029_member_profile_media.sql` through the normal migration runner if it has not already been applied. It adds `member_profile_media` with owner-linked avatar/cover keys and bounded X/Y/zoom values; it does not replace legacy profiles or membership references. Back up before migrations; do not roll back that table after members start uploading images.

Avatar URLs are used by authenticated member screens and portable membership cards; treat downloaded cards and their embedded photo URLs as shareable copies. Covers are retrieved through the member's authenticated, no-store endpoint, without publishing their object keys or URLs. Public QR verification continues to expose only the existing name/reference/status allowlist and never either image, positioning, address, email, phone or internal IDs. An existing public R2 bucket is not an object-level privacy boundary: verify edge/public-domain access rules for the cover prefix before describing covers as strictly private storage. Previously downloaded images cannot be recalled.

## Verification

Validated on 6 November 2026 with Go 1.26.10, PostgreSQL 17.11, Node 24.22.1 and Chromium headless shell 156:

- `go test -p 1 ./...`: PASS, including the complete handler and service integration suites.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `npm test`: 24 passed, 4 browser suites skipped because their URLs were not set in the unit-test invocation.
- Membership browser acceptance run separately against the production build: 8 passed, 0 skipped, including avatar zoom synchronization.
- `npm run build`: PASS (66 static pages; existing multiple-lockfile warning).
- `git diff --check`: PASS.

Coverage includes:

- HTTP integration: unauthenticated upload denial, malformed image rejection, route validation, avatar/cover upload and deletion, card crop synchronization, owner isolation, and private response caching.
- Service integration: card photo/position synchronization, rejection of arbitrary URL and stale-form changes, cover dimensions and removal.
- Unit tests: image byte limits, profile payload isolation, upload progress, CSRF/cookie handling, and server validation errors.
- Browser acceptance against a production build with mocked API responses: responsive cards, mobile cover preview/cancel/upload/removal, unsaved profile text, avatar positioning on cards, print output, public verification privacy/statuses, and administrative controls. Browser fixtures do not substitute for live R2 verification.

## Production verification

1. Confirm the deployed backend and frontend use this commit and the normal migration runner completed migration 29.
2. With a real member account, upload, replace, reposition and remove both photo types; refresh the page and verify persistence. Confirm the displayed, printed and downloaded card match the current avatar and crop.
3. Verify JPEG, PNG and WebP inputs, corrupt/truncated payloads, oversized uploads, tiny dimensions and large-pixel images. Confirm sanitized objects have no EXIF/GPS metadata.
4. With a second member and a coordinator account, confirm media endpoints cannot access or modify the first member's cover or images. Check anonymous access and QR payload privacy.
5. Verify real R2 object replacement/deletion, public-domain cover access policy, proxy upload size/timeouts and mobile network progress/error handling.
6. Confirm marketplace, inventory, login and coordinator smoke checks; keep `PAYMENT_PROVIDER=unconfigured` so Paystack checkout remains disabled. No NGO functionality was introduced.

A GitHub push may trigger existing hosting automation; it does not prove a successful production deployment. Check hosting logs and the live smoke tests before marking deployment verified.
