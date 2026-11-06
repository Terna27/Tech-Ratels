package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/url"
	"ratels-api/internal/security"
	"ratels-api/internal/storage"
	"strings"
	"sync"
	"testing"

	"ratels-api/internal/service"
)

func communityRef(t *testing.T, h *TestHarness, id string) string {
	t.Helper()
	var ref string
	if err := h.Pool.QueryRow(context.Background(), `SELECT membership_reference FROM users WHERE id=$1`, id).Scan(&ref); err != nil {
		t.Fatal(err)
	}
	return ref
}
func TestCommunitySearchFriendshipsPostsAndPrivacy(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	a, ta := operationalUser(t, h, "member")
	b, tb := operationalUser(t, h, "member")
	_, tc := operationalUser(t, h, "member")
	ra, rb := communityRef(t, h, a), communityRef(t, h, b)
	base := "/network/members/" + url.PathEscape(rb)
	assertStatus(t, operationRequest(h, "", "GET", "/network/members", nil), 401)
	assertStatus(t, operationRequest(h, ta, "GET", "/network/members?page=0", nil), 400)
	w := operationRequest(h, ta, "GET", "/network/members?q="+url.QueryEscape(rb), nil)
	assertStatus(t, w, 200)
	var members []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &members); err != nil || len(members) != 1 {
		t.Fatal(w.Body.String(), err)
	}
	for _, key := range []string{"id", "user_id", "email", "phone", "address", "password_hash", "primary_role", "appointments", "cover_key", "cover_url"} {
		if _, ok := members[0][key]; ok {
			t.Fatalf("directory exposes %s", key)
		}
	}
	assertStatus(t, operationRequest(h, ta, "GET", base, nil), 200)
	assertStatus(t, operationRequest(h, ta, "GET", base+"/cover", nil), 404)
	assertStatus(t, operationRequest(h, ta, "POST", "/network/members/"+ra+"/follow", nil), 400)
	for i := 0; i < 2; i++ {
		assertStatus(t, operationRequest(h, ta, "POST", base+"/follow", nil), 200)
	}
	var n int
	if err := h.Pool.QueryRow(ctx, `SELECT count(*) FROM member_follows WHERE follower_id=$1 AND followed_id=$2`, a, b).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	for i := 0; i < 2; i++ {
		assertStatus(t, operationRequest(h, ta, "POST", base+"/friend", nil), 200)
	}
	assertStatus(t, operationRequest(h, tb, "POST", "/network/members/"+ra+"/friend", nil), 409)
	assertStatus(t, operationRequest(h, ta, "POST", base+"/friend/accept", nil), 404)
	assertStatus(t, operationRequest(h, tc, "POST", "/network/members/"+ra+"/friend/accept", nil), 404)
	assertStatus(t, operationRequest(h, tb, "GET", "/network/requests", nil), 200)
	assertStatus(t, operationRequest(h, tb, "POST", "/network/members/"+ra+"/friend/accept", nil), 200)
	w = operationRequest(h, ta, "GET", base, nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"friendship":"friends"`) {
		t.Fatal(w.Body.String())
	}
	// A friend may read and like friends-only posts; another member may not.
	w = operationRequest(h, tb, "POST", "/network/posts", map[string]string{"body": "Friends update <script>alert(1)</script>", "visibility": "friends"})
	assertStatus(t, w, 201)
	var created map[string]string
	json.Unmarshal(w.Body.Bytes(), &created)
	post := created["id"]
	w = operationRequest(h, tc, "GET", "/network/feed?member="+rb, nil)
	assertStatus(t, w, 200)
	if w.Body.String() != "[]\n" {
		t.Fatal("friends-only post leaked", w.Body.String())
	}
	assertStatus(t, operationRequest(h, tc, "PUT", "/network/posts/"+post+"/like", nil), 404)
	assertStatus(t, operationRequest(h, ta, "DELETE", "/network/posts/"+post, nil), 404)
	for i := 0; i < 2; i++ {
		assertStatus(t, operationRequest(h, ta, "PUT", "/network/posts/"+post+"/like", nil), 200)
	}
	w = operationRequest(h, ta, "GET", "/network/feed?member="+rb, nil)
	assertStatus(t, w, 200)
	var posts []map[string]any
	json.Unmarshal(w.Body.Bytes(), &posts)
	if len(posts) != 1 || posts[0]["likes"] != float64(1) || posts[0]["liked"] != true {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, operationRequest(h, ta, "DELETE", "/network/posts/"+post+"/like", nil), 200)
	assertStatus(t, operationRequest(h, ta, "DELETE", base+"/friend", nil), 200)
	assertStatus(t, operationRequest(h, ta, "PUT", "/network/posts/"+post+"/like", nil), 404)
	assertStatus(t, operationRequest(h, ta, "POST", "/network/posts", map[string]string{"body": " ", "visibility": "members"}), 400)
	assertStatus(t, operationRequest(h, ta, "POST", "/network/posts", map[string]string{"body": strings.Repeat("a", 3001), "visibility": "members"}), 400)
	assertStatus(t, operationRequest(h, ta, "POST", "/network/posts", map[string]string{"body": "safe", "visibility": "public", "author_id": b}), 400)
	// Cancelling belongs to the sender; declining belongs to the recipient.
	assertStatus(t, operationRequest(h, ta, "POST", base+"/friend", nil), 200)
	assertStatus(t, operationRequest(h, ta, "POST", base+"/friend/decline", nil), 404)
	assertStatus(t, operationRequest(h, tb, "POST", "/network/members/"+ra+"/friend/decline", nil), 200)
	assertStatus(t, operationRequest(h, ta, "POST", base+"/friend", nil), 200)
	assertStatus(t, operationRequest(h, ta, "DELETE", base+"/friend", nil), 200)
	assertStatus(t, operationRequest(h, tb, "PUT", "/network/preferences", map[string]bool{"discoverable": false, "share_cover": true}), 200)
	assertStatus(t, operationRequest(h, ta, "GET", base, nil), 404)
	w = operationRequest(h, ta, "GET", "/network/members?q="+rb, nil)
	assertStatus(t, w, 200)
	if w.Body.String() != "[]\n" {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, operationRequest(h, tb, "GET", base, nil), 200) // owner retains access
	assertStatus(t, operationRequest(h, tb, "DELETE", "/network/posts/"+post, nil), 200)
	_, err := h.Pool.Exec(ctx, `UPDATE users SET status='suspended' WHERE id=$1`, b)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, operationRequest(h, ta, "GET", base, nil), 404)
}
func TestCommunityCrossedRequestsAndLikeIdempotency(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	a, _ := operationalUser(t, h, "member")
	b, _ := operationalUser(t, h, "member")
	ra, rb := communityRef(t, h, a), communityRef(t, h, b)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, pair := range [][2]string{{a, rb}, {b, ra}} {
		wg.Add(1)
		go func(v, r string) {
			defer wg.Done()
			errs <- h.AuthService.ChangeSocialRelationship(ctx, v, r, "request")
		}(pair[0], pair[1])
	}
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if err == service.ErrFriendRequestConflict {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal(successes, conflicts)
	}
	post, err := h.AuthService.CreateSocialPost(ctx, a, "Concurrent likes", "members")
	if err != nil {
		t.Fatal(err)
	}
	errs = make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- h.AuthService.LikeSocialPost(ctx, b, post, true) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := h.Pool.QueryRow(ctx, `SELECT count(*) FROM member_post_likes WHERE post_id=$1`, post).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}

func TestCommunityCoverSharingAndBadgePreferences(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	a, _ := operationalUser(t, h, "member")
	b, _ := operationalUser(t, h, "member")
	ref := communityRef(t, h, b)
	store := storage.NewDevSandboxStorageService("https://media.example.test")
	var imageBytes bytes.Buffer
	if err := jpeg.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 900, 300)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.AuthService.UploadMemberMedia(ctx, b, "cover", &imageBytes, store); err != nil {
		t.Fatal(err)
	}
	p, err := h.AuthService.SocialMemberProfile(ctx, a, ref)
	if err != nil || p.HasCover {
		t.Fatal("cover shared without opt-in", p, err)
	}
	if _, err := h.AuthService.SocialCover(ctx, a, ref, store); err != service.ErrCommunityNotFound {
		t.Fatal("private cover readable", err)
	}
	if _, err := h.AuthService.CommunityPreferences(ctx, b, &service.SocialPreferences{Discoverable: true, ShareCover: true}); err != nil {
		t.Fatal(err)
	}
	data, err := h.AuthService.SocialCover(ctx, a, ref, store)
	if err != nil || len(data) == 0 {
		t.Fatal("opted-in cover missing", err)
	}
	badge := security.GenerateUUID()
	if _, err = h.Pool.Exec(ctx, `INSERT INTO badges(id,code,name,description) VALUES($1,$1,'Public helper','Test badge')`, badge); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Pool.Exec(ctx, `INSERT INTO badge_awards(id,user_id,badge_id) VALUES($1,$2,$3)`, security.GenerateUUID(), b, badge); err != nil {
		t.Fatal(err)
	}
	p, err = h.AuthService.SocialMemberProfile(ctx, a, ref)
	if err != nil || len(p.Badges) != 1 {
		t.Fatal(p, err)
	}
	if _, err = h.Pool.Exec(ctx, `UPDATE member_profiles SET is_public_badges=false WHERE user_id=$1`, b); err != nil {
		t.Fatal(err)
	}
	p, err = h.AuthService.SocialMemberProfile(ctx, a, ref)
	if err != nil || len(p.Badges) != 0 {
		t.Fatal("private badges leaked", p, err)
	}
	if _, err = h.AuthService.CommunityPreferences(ctx, b, &service.SocialPreferences{Discoverable: false, ShareCover: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = h.AuthService.SocialCover(ctx, a, ref, store); err != service.ErrCommunityNotFound {
		t.Fatal("hidden cover readable", err)
	}
}
