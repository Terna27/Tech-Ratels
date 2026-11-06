# Ratels member community

## Delivered behavior

Signed-in active members can open **Community** from the member sidebar or dashboard, search members by name or membership reference, and view member profiles. Profiles show the name, photo and crop, membership reference, state/LGA, biography, follower count and permitted public badges. An optionally shared cover is delivered through an authenticated endpoint. Email, phone, street address, internal user IDs, roles, appointments, storage keys and card verification tokens are excluded.

Members can follow/unfollow independently of friendship, send/cancel friend requests, accept/decline incoming requests, remove friends, and browse **My friends**, **Following** and pending requests. A crossed request produces a conflict directing the recipient to accept/decline; it never silently creates a friendship. Pairwise row locks and a unique unordered pair index prevent duplicate crossed requests.

The first feed supports text posts up to 3,000 Unicode characters, with **All members** or **Friends only** visibility. Members may delete their own posts and like/unlike readable posts. The following/friends feed filters respect the same visibility rules. Likes are idempotent and cannot be added to inaccessible posts. Post bodies render as text, not HTML. This release does not add comments, messaging or post attachments.

## Privacy and authorization

Community pages and `/api/v1/network/*` require authentication and an active account. This namespace is separate from the existing community-project/request APIs. Existing privileged member-management, coordinator and Super Admin routes are unchanged. Public QR verification is unchanged.

In **Community → Privacy**, members can hide their profile and posts from others and opt into sharing their cover. Discoverability defaults to enabled; cover sharing defaults to disabled. Owners retain access to their own hidden profile/posts. Existing follows/friendships are retained when a member hides their profile. Previously viewed/downloaded copies cannot be recalled. Members should include only information intended for other members in their biography/posts.

Only the authenticated recipient can accept or decline a pending request. Only the authenticated sender can cancel it; either accepted friend can remove the friendship. IDs supplied in request bodies cannot impersonate an author, follower or friend. Cookie-authenticated writes use the existing CSRF middleware. Mutating payloads reject unknown fields; search/feed/request pages are limited to 20 rows, with bounded page and query lengths. Inactive or hidden authors are excluded from other members' reads and likes. No financial or role-changing permissions are granted by a social connection.

The existing R2 integration is reused. The previously documented public-bucket caveat still applies: verify public-domain/edge restrictions for cover objects. API authentication is not an object-level access policy for a public bucket.

## Migration

Apply `20261106000030_member_community.sql` through the existing migration runner before deploying application code. It adds:

- `member_social_preferences`: discoverability and cover-sharing settings.
- `member_follows`: unique directed follows, with self-follow protection.
- `member_friendships`: pending/accepted requests with unique unordered member pairs and no self-friendship.
- `member_posts`: owner-linked text and audience with validation and feed indexes.
- `member_post_likes`: one like per member/post, cascading on post deletion.

Existing members, profiles, cards, images and membership references are preserved. No new environment variables, dependencies or storage providers are required. Take a backup before migration. Rolling migration 30 back deletes the new social data; do not roll it back after launch without an explicit data-preservation plan.

## Validation

The implementation includes HTTP/database integration tests for search allowlists, authentication, profile privacy, follow idempotency, friend-request authorization, friends-only visibility, owner-only post deletion, post validation, hidden/inactive users, crossed requests and concurrent likes. Frontend unit tests cover Unicode limits, allowed audiences, relationship actions and identifier encoding. Browser tests cover mobile discovery/profile navigation, follow/request/cancel/accept, like/unlike, HTML-text safety, friends-only post submission, privacy controls and anonymous access. Existing member-card/profile browser regression tests are run alongside them. Browser API responses are fixtures; the HTTP/database tests exercise real authorization and SQL.

Validation on 6 November 2026 (Go 1.26.10, PostgreSQL 17.11, Node 24.22.1, Chromium headless shell 156):

- `go test -p 1 ./...`: PASS, including full handler/service and migration suites.
- Additional final `go test ./internal/handler -run '^TestCommunity' -count=1`: PASS, including cover-sharing opt-in and public-badge preferences.
- `go vet ./...` and `go build ./...`: PASS.
- `npm test`: 27 passed, 5 browser suites skipped in the unit invocation.
- Community and membership browser suites run separately against the production build: 14 passed, 0 skipped.
- `npm run build`: PASS, 67 static pages plus dynamic routes (existing multiple-lockfile warning).
- `git diff --check`: PASS.

CI now includes the community browser suite.

## Deployment and production checks

1. Confirm migration 30 completes and the API/web deploy the same commit.
2. With two real members, test search, profile visibility, cover-sharing opt-in and public badge preferences.
3. Send, cancel, accept, decline and remove friend requests in both directions; test follows separately.
4. Publish an all-member and a friends-only post. Check access using a friend, a nonfriend and a signed-out browser; repeat after removing the friendship or hiding the author.
5. Like/unlike repeatedly, delete an owned post and confirm its likes are removed. Confirm another member cannot delete it.
6. Run login, marketplace, inventory and coordinator smoke checks. Keep Paystack checkout disabled (`PAYMENT_PROVIDER=unconfigured`); no NGO functionality was introduced.

A push triggers the existing CI/hosting automation but does not by itself prove a successful production deployment. Inspect hosting logs and perform the live checks above.

## Cover display follow-up

Search results, friend requests, **My friends** and **Following** now render the same cover component as member profiles, including saved position/zoom and a neutral fallback for missing or unavailable images. Previously, result cards rendered only avatars and names. Cover sharing remains opt-in: the owner must enable **Community → Privacy → Show my cover photograph to other members** and save. Hidden profiles and unshared covers remain protected by the existing backend checks; this display fix does not change visibility defaults or require a migration.

Regression coverage loads actual image responses on search/profile screens and verifies that unshared covers render on neither screen. Validation: 27 frontend unit tests passed; the Next.js production build passed; 15 community/membership browser checks passed with no skips, including actual shared-cover loading and unshared-cover protection.
