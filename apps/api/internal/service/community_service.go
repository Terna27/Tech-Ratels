package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"ratels-api/internal/security"
	"ratels-api/internal/storage"
)

var ErrCommunityNotFound = errors.New("member or post not available")
var ErrCommunityInvalid = errors.New("invalid community request")
var ErrFriendRequestConflict = errors.New("this member has already sent you a request; accept or decline it")

type SocialMember struct {
	Reference  string   `json:"reference"`
	Name       string   `json:"name"`
	Avatar     string   `json:"avatar_url,omitempty"`
	Bio        string   `json:"bio"`
	Location   string   `json:"location"`
	HasCover   bool     `json:"has_cover"`
	AvatarX    int      `json:"avatar_x"`
	AvatarY    int      `json:"avatar_y"`
	AvatarZoom int      `json:"avatar_zoom"`
	CoverX     int      `json:"cover_x"`
	CoverY     int      `json:"cover_y"`
	CoverZoom  int      `json:"cover_zoom"`
	Following  bool     `json:"following"`
	Friendship string   `json:"friendship"`
	Followers  int      `json:"followers"`
	Badges     []string `json:"badges"`
}
type SocialPreferences struct {
	Discoverable bool `json:"discoverable"`
	ShareCover   bool `json:"share_cover"`
}
type SocialPost struct {
	ID              string    `json:"id"`
	AuthorReference string    `json:"author_reference"`
	AuthorName      string    `json:"author_name"`
	Avatar          string    `json:"avatar_url,omitempty"`
	Body            string    `json:"body"`
	Visibility      string    `json:"visibility"`
	CreatedAt       time.Time `json:"created_at"`
	Likes           int       `json:"likes"`
	Liked           bool      `json:"liked"`
	Own             bool      `json:"own"`
}

// All community endpoints require an active signed-in viewer. No user/profile DB
// model is serialized; only the explicit member-facing fields below are returned.
func (s *AuthService) communityViewer(ctx context.Context, viewer string) error {
	var active bool
	if err := s.repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, viewer).Scan(&active); err != nil {
		return err
	}
	if !active {
		return ErrAccountInactive
	}
	return nil
}
func validCommunityPage(page int) bool { return page >= 1 && page <= 500 }

const socialSelect = `SELECT u.membership_reference,u.display_name,COALESCE(p.avatar_url,''),COALESCE(p.bio,''),
 concat_ws(', ',l.name,st.name),COALESCE(sp.share_cover,false) AND COALESCE(m.cover_key,'')<>'',
 COALESCE(m.avatar_x,50),COALESCE(m.avatar_y,50),COALESCE(m.avatar_zoom,100),
 COALESCE(m.cover_x,50),COALESCE(m.cover_y,50),COALESCE(m.cover_zoom,100),
 EXISTS(SELECT 1 FROM member_follows f WHERE f.follower_id=$1 AND f.followed_id=u.id),
 COALESCE((SELECT CASE WHEN fr.state='accepted' THEN 'friends' WHEN fr.requester_id=$1 THEN 'outgoing' ELSE 'incoming' END
 FROM member_friendships fr WHERE (fr.requester_id=$1 AND fr.addressee_id=u.id) OR (fr.addressee_id=$1 AND fr.requester_id=u.id)),'none'),
 (SELECT count(*) FROM member_follows f JOIN users fu ON fu.id=f.follower_id AND fu.status='active' WHERE f.followed_id=u.id)
 FROM users u LEFT JOIN member_profiles p ON p.user_id=u.id LEFT JOIN states st ON st.id=p.home_state_id
 LEFT JOIN lgas l ON l.id=p.home_lga_id LEFT JOIN member_profile_media m ON m.user_id=u.id
 LEFT JOIN member_social_preferences sp ON sp.user_id=u.id `

func scanSocial(row pgx.Row) (SocialMember, error) {
	m := SocialMember{Badges: []string{}}
	err := row.Scan(&m.Reference, &m.Name, &m.Avatar, &m.Bio, &m.Location, &m.HasCover, &m.AvatarX, &m.AvatarY, &m.AvatarZoom, &m.CoverX, &m.CoverY, &m.CoverZoom, &m.Following, &m.Friendship, &m.Followers)
	return m, err
}
func (s *AuthService) SearchSocialMembers(ctx context.Context, viewer, q, scope string, page int) ([]SocialMember, error) {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) > 100 || !validCommunityPage(page) || (scope != "all" && scope != "friends" && scope != "following") {
		return nil, ErrCommunityInvalid
	}
	rows, err := s.repo.Pool().Query(ctx, socialSelect+`WHERE u.status='active' AND COALESCE(sp.discoverable,true) AND
 ($2='' OR strpos(lower(u.display_name),lower($2))>0 OR strpos(lower(u.membership_reference),lower($2))>0)
 AND ($4='all' OR ($4='following' AND EXISTS(SELECT 1 FROM member_follows f WHERE f.follower_id=$1 AND f.followed_id=u.id)) OR ($4='friends' AND EXISTS(SELECT 1 FROM member_friendships fr WHERE fr.state='accepted' AND ((fr.requester_id=$1 AND fr.addressee_id=u.id) OR (fr.addressee_id=$1 AND fr.requester_id=u.id)))))
 ORDER BY lower(u.display_name),u.membership_reference LIMIT 20 OFFSET $3`, viewer, q, (page-1)*20, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SocialMember{}
	for rows.Next() {
		m, e := scanSocial(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *AuthService) SocialMemberProfile(ctx context.Context, viewer, reference string) (*SocialMember, error) {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return nil, err
	}
	if len(reference) > 100 {
		return nil, ErrCommunityInvalid
	}
	m, err := scanSocial(s.repo.Pool().QueryRow(ctx, socialSelect+`WHERE u.membership_reference=$2 AND u.status='active' AND (u.id=$1 OR COALESCE(sp.discoverable,true))`, viewer, reference))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCommunityNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.Pool().Query(ctx, `SELECT b.name FROM badge_awards a JOIN badges b ON b.id=a.badge_id
 JOIN users u ON u.id=a.user_id JOIN member_profiles p ON p.user_id=u.id
 WHERE u.membership_reference=$1 AND p.is_public_badges AND a.is_public AND a.revocation_reason IS NULL ORDER BY b.name LIMIT 100`, reference)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		m.Badges = append(m.Badges, name)
	}
	return &m, rows.Err()
}
func (s *AuthService) SocialCover(ctx context.Context, viewer, reference string, store storage.StorageService) ([]byte, error) {
	m, err := s.SocialMemberProfile(ctx, viewer, reference)
	if err != nil {
		return nil, err
	}
	if !m.HasCover {
		return nil, ErrCommunityNotFound
	}
	var id string
	if err = s.repo.Pool().QueryRow(ctx, `SELECT id FROM users WHERE membership_reference=$1`, reference).Scan(&id); err != nil {
		return nil, err
	}
	return s.MemberCoverImage(ctx, id, store)
}
func (s *AuthService) CommunityPreferences(ctx context.Context, viewer string, in *SocialPreferences) (*SocialPreferences, error) {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return nil, err
	}
	if in != nil {
		_, err := s.repo.Pool().Exec(ctx, `INSERT INTO member_social_preferences(user_id,discoverable,share_cover) VALUES($1,$2,$3)
 ON CONFLICT(user_id) DO UPDATE SET discoverable=$2,share_cover=$3`, viewer, in.Discoverable, in.ShareCover)
		if err != nil {
			return nil, err
		}
	}
	p := &SocialPreferences{Discoverable: true}
	err := s.repo.Pool().QueryRow(ctx, `SELECT discoverable,share_cover FROM member_social_preferences WHERE user_id=$1`, viewer).Scan(&p.Discoverable, &p.ShareCover)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return p, err
}

// Pairwise row locks serialize crossed friend requests and idempotent follow writes.
func (s *AuthService) ChangeSocialRelationship(ctx context.Context, viewer, reference, action string) error {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return err
	}
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var target string
	err = tx.QueryRow(ctx, `SELECT u.id FROM users u LEFT JOIN member_social_preferences sp ON sp.user_id=u.id
 WHERE u.membership_reference=$1 AND u.status='active' AND COALESCE(sp.discoverable,true)`, reference).Scan(&target)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCommunityNotFound
	}
	if err != nil {
		return err
	}
	if target == viewer {
		return ErrCommunityInvalid
	}
	rows, err := tx.Query(ctx, `SELECT id FROM users WHERE id IN ($1,$2) AND status='active' ORDER BY id FOR UPDATE`, viewer, target)
	if err != nil {
		return err
	}
	n := 0
	for rows.Next() {
		n++
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	if n != 2 {
		return ErrCommunityNotFound
	}
	switch action {
	case "follow":
		_, err = tx.Exec(ctx, `INSERT INTO member_follows(follower_id,followed_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, viewer, target)
	case "unfollow":
		_, err = tx.Exec(ctx, `DELETE FROM member_follows WHERE follower_id=$1 AND followed_id=$2`, viewer, target)
	case "request":
		var requester, state string
		e := tx.QueryRow(ctx, `SELECT requester_id,state FROM member_friendships WHERE (requester_id=$1 AND addressee_id=$2) OR (requester_id=$2 AND addressee_id=$1)`, viewer, target).Scan(&requester, &state)
		if errors.Is(e, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO member_friendships(requester_id,addressee_id) VALUES($1,$2)`, viewer, target)
		} else if e != nil {
			return e
		} else if state == "pending" && requester != viewer {
			return ErrFriendRequestConflict
		}
	case "accept":
		tag, e := tx.Exec(ctx, `UPDATE member_friendships SET state='accepted' WHERE requester_id=$2 AND addressee_id=$1 AND state='pending'`, viewer, target)
		err = e
		if e == nil && tag.RowsAffected() == 0 {
			return ErrCommunityNotFound
		}
	case "decline":
		tag, e := tx.Exec(ctx, `DELETE FROM member_friendships WHERE requester_id=$2 AND addressee_id=$1 AND state='pending'`, viewer, target)
		err = e
		if e == nil && tag.RowsAffected() == 0 {
			return ErrCommunityNotFound
		}
	case "remove":
		_, err = tx.Exec(ctx, `DELETE FROM member_friendships WHERE (state='accepted' AND ((requester_id=$1 AND addressee_id=$2) OR (requester_id=$2 AND addressee_id=$1))) OR (state='pending' AND requester_id=$1 AND addressee_id=$2)`, viewer, target)
	default:
		return ErrCommunityInvalid
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *AuthService) SocialRequests(ctx context.Context, viewer string, page int) ([]SocialMember, error) {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return nil, err
	}
	if !validCommunityPage(page) {
		return nil, ErrCommunityInvalid
	}
	rows, err := s.repo.Pool().Query(ctx, socialSelect+`WHERE u.status='active' AND COALESCE(sp.discoverable,true) AND EXISTS(
 SELECT 1 FROM member_friendships fr WHERE fr.state='pending' AND ((fr.requester_id=$1 AND fr.addressee_id=u.id) OR (fr.addressee_id=$1 AND fr.requester_id=u.id)))
 ORDER BY u.membership_reference LIMIT 20 OFFSET $2`, viewer, (page-1)*20)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SocialMember{}
	for rows.Next() {
		m, e := scanSocial(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const visiblePost = ` u.status='active' AND (u.id=$1 OR COALESCE(sp.discoverable,true)) AND
 (p.author_id=$1 OR p.visibility='members' OR EXISTS(SELECT 1 FROM member_friendships fr WHERE fr.state='accepted' AND
 ((fr.requester_id=$1 AND fr.addressee_id=p.author_id) OR (fr.addressee_id=$1 AND fr.requester_id=p.author_id)))) `

func (s *AuthService) CommunityFeed(ctx context.Context, viewer, scope, reference string, page int) ([]SocialPost, error) {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return nil, err
	}
	if !validCommunityPage(page) || len(reference) > 100 || (scope != "all" && scope != "following" && scope != "friends") {
		return nil, ErrCommunityInvalid
	}
	rows, err := s.repo.Pool().Query(ctx, `SELECT p.id,u.membership_reference,u.display_name,COALESCE(mp.avatar_url,''),p.body,p.visibility,p.created_at,
 (SELECT count(*) FROM member_post_likes l WHERE l.post_id=p.id),EXISTS(SELECT 1 FROM member_post_likes l WHERE l.post_id=p.id AND l.user_id=$1),p.author_id=$1
 FROM member_posts p JOIN users u ON u.id=p.author_id LEFT JOIN member_profiles mp ON mp.user_id=u.id
 LEFT JOIN member_social_preferences sp ON sp.user_id=u.id WHERE `+visiblePost+` AND ($2='' OR u.membership_reference=$2)
 AND ($3='all' OR ($3='following' AND EXISTS(SELECT 1 FROM member_follows f WHERE f.follower_id=$1 AND f.followed_id=p.author_id))
 OR ($3='friends' AND EXISTS(SELECT 1 FROM member_friendships fr WHERE fr.state='accepted' AND ((fr.requester_id=$1 AND fr.addressee_id=p.author_id) OR (fr.addressee_id=$1 AND fr.requester_id=p.author_id)))))
 ORDER BY p.created_at DESC,p.id DESC LIMIT 20 OFFSET $4`, viewer, reference, scope, (page-1)*20)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SocialPost{}
	for rows.Next() {
		var p SocialPost
		if err = rows.Scan(&p.ID, &p.AuthorReference, &p.AuthorName, &p.Avatar, &p.Body, &p.Visibility, &p.CreatedAt, &p.Likes, &p.Liked, &p.Own); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *AuthService) CreateSocialPost(ctx context.Context, viewer, body, visibility string) (string, error) {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return "", err
	}
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 3000 || !validProfileText(body, 12000) || (visibility != "members" && visibility != "friends") {
		return "", ErrCommunityInvalid
	}
	id := security.GenerateUUID()
	_, err := s.repo.Pool().Exec(ctx, `INSERT INTO member_posts(id,author_id,body,visibility) VALUES($1,$2,$3,$4)`, id, viewer, body, visibility)
	return id, err
}
func (s *AuthService) DeleteSocialPost(ctx context.Context, viewer, id string) error {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return err
	}
	tag, err := s.repo.Pool().Exec(ctx, `DELETE FROM member_posts WHERE id=$1 AND author_id=$2`, id, viewer)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrCommunityNotFound
	}
	return err
}
func (s *AuthService) LikeSocialPost(ctx context.Context, viewer, id string, like bool) error {
	if err := s.communityViewer(ctx, viewer); err != nil {
		return err
	}
	// Visibility is checked in the mutation statement itself, including friends-only
	// access and hidden/inactive authors; POST IDs never confer access.
	var count int
	err := s.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM member_posts p JOIN users u ON u.id=p.author_id LEFT JOIN member_social_preferences sp ON sp.user_id=u.id WHERE p.id=$2 AND `+visiblePost, viewer, id).Scan(&count)
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrCommunityNotFound
	}
	if like {
		_, err = s.repo.Pool().Exec(ctx, `INSERT INTO member_post_likes(post_id,user_id) SELECT p.id,$1::varchar(36) FROM member_posts p JOIN users u ON u.id=p.author_id LEFT JOIN member_social_preferences sp ON sp.user_id=u.id WHERE p.id=$2 AND `+visiblePost+` ON CONFLICT DO NOTHING`, viewer, id)
	} else {
		_, err = s.repo.Pool().Exec(ctx, `DELETE FROM member_post_likes WHERE post_id=$1 AND user_id=$2`, id, viewer)
	}
	return err
}
