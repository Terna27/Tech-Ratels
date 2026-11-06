package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

func communityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrAccountInactive):
		ErrorJSON(w, 403, "ACCOUNT_INACTIVE", "An active member account is required", nil)
	case errors.Is(err, service.ErrCommunityNotFound):
		ErrorJSON(w, 404, "NOT_FOUND", "Member or post unavailable", nil)
	case errors.Is(err, service.ErrCommunityInvalid):
		ErrorJSON(w, 400, "INVALID_REQUEST", "Check your request. Posts must contain 1–3000 characters.", nil)
	case errors.Is(err, service.ErrFriendRequestConflict):
		ErrorJSON(w, 409, "REQUEST_PENDING", err.Error(), nil)
	default:
		ErrorJSON(w, 500, "COMMUNITY_UNAVAILABLE", "Community is temporarily unavailable. Please try again.", nil)
	}
}
func communityPage(r *http.Request) int {
	if r.URL.Query().Get("page") == "" {
		return 1
	}
	n, e := strconv.Atoi(r.URL.Query().Get("page"))
	if e != nil {
		return 0
	}
	return n
}
func communityBody(w http.ResponseWriter, r *http.Request, out any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		communityError(w, service.ErrCommunityInvalid)
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		communityError(w, service.ErrCommunityInvalid)
		return false
	}
	return true
}
func (h *MemberHandler) CommunitySearch(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "all"
	}
	out, err := h.authService.SearchSocialMembers(r.Context(), GetCurrentUser(r.Context()).ID, r.URL.Query().Get("q"), scope, communityPage(r))
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 200, out)
}
func (h *MemberHandler) CommunityProfile(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	out, err := h.authService.SocialMemberProfile(r.Context(), GetCurrentUser(r.Context()).ID, chi.URLParam(r, "reference"))
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 200, out)
}
func (h *MemberHandler) CommunityCover(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	out, err := h.authService.SocialCover(r.Context(), GetCurrentUser(r.Context()).ID, chi.URLParam(r, "reference"), h.mediaStorage)
	if err != nil {
		communityError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.WriteHeader(200)
	_, _ = w.Write(out)
}
func (h *MemberHandler) SocialPreferences(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	var in *service.SocialPreferences
	if r.Method == http.MethodPut {
		in = &service.SocialPreferences{}
		if !communityBody(w, r, in) {
			return
		}
	}
	out, err := h.authService.CommunityPreferences(r.Context(), GetCurrentUser(r.Context()).ID, in)
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 200, out)
}
func (h *MemberHandler) SocialRelationship(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	kind := chi.URLParam(r, "relationship")
	action := ""
	if kind == "follow" {
		action = "follow"
		if r.Method == http.MethodDelete {
			action = "unfollow"
		}
	}
	if kind == "friend" {
		action = "request"
		if r.Method == http.MethodDelete {
			action = "remove"
		}
	}
	if a := chi.URLParam(r, "action"); a != "" {
		action = a
	}
	err := h.authService.ChangeSocialRelationship(r.Context(), GetCurrentUser(r.Context()).ID, chi.URLParam(r, "reference"), action)
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 200, map[string]bool{"ok": true})
}
func (h *MemberHandler) SocialRequests(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	out, err := h.authService.SocialRequests(r.Context(), GetCurrentUser(r.Context()).ID, communityPage(r))
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 200, out)
}
func (h *MemberHandler) SocialFeed(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "all"
	}
	out, err := h.authService.CommunityFeed(r.Context(), GetCurrentUser(r.Context()).ID, scope, r.URL.Query().Get("member"), communityPage(r))
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 200, out)
}
func (h *MemberHandler) SocialPost(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	id := chi.URLParam(r, "postID")
	viewer := GetCurrentUser(r.Context()).ID
	if r.Method == http.MethodDelete {
		if err := h.authService.DeleteSocialPost(r.Context(), viewer, id); err != nil {
			communityError(w, err)
			return
		}
		JSON(w, 200, map[string]bool{"ok": true})
		return
	}
	var in struct {
		Body       string `json:"body"`
		Visibility string `json:"visibility"`
	}
	if !communityBody(w, r, &in) {
		return
	}
	id, err := h.authService.CreateSocialPost(r.Context(), viewer, in.Body, in.Visibility)
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 201, map[string]string{"id": id})
}
func (h *MemberHandler) SocialLike(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	err := h.authService.LikeSocialPost(r.Context(), GetCurrentUser(r.Context()).ID, chi.URLParam(r, "postID"), r.Method == http.MethodPut)
	if err != nil {
		communityError(w, err)
		return
	}
	JSON(w, 200, map[string]bool{"ok": true})
}
