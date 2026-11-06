package service

import (
 "bytes"
 "context"
 "errors"
 "fmt"
 "io"
 "strings"

 "github.com/jackc/pgx/v5"
 "ratels-api/internal/security"
 "ratels-api/internal/storage"
)

var ErrMemberMediaInvalid = errors.New("invalid member profile image or position")
var ErrMemberMediaUnavailable = errors.New("member image unavailable")

type MemberMedia struct {
 AvatarURL string `json:"avatar_url,omitempty"`
 HasCover bool `json:"has_cover"`
 AvatarX int `json:"avatar_x"`
 AvatarY int `json:"avatar_y"`
 AvatarZoom int `json:"avatar_zoom"`
 CoverX int `json:"cover_x"`
 CoverY int `json:"cover_y"`
 CoverZoom int `json:"cover_zoom"`
}

func memberMediaKind(kind string) bool { return kind == "avatar" || kind == "cover" }

func memberMediaPositionValid(x,y,zoom int) bool {
 return x>=0 && x<=100 && y>=0 && y<=100 && zoom>=100 && zoom<=200
}

func (s *AuthService) MemberMediaDetails(ctx context.Context,id string) (*MemberMedia,error) {
 m:=&MemberMedia{AvatarX:50,AvatarY:50,AvatarZoom:100,CoverX:50,CoverY:50,CoverZoom:100}
 var status string
 err:=s.repo.Pool().QueryRow(ctx,`SELECT status FROM users WHERE id=$1`,id).Scan(&status)
 if err!=nil {return nil,err}
 if status!="active"{return nil,ErrAccountInactive}
 var avatarURL *string
 err=s.repo.Pool().QueryRow(ctx,`SELECT avatar_url FROM member_profiles WHERE user_id=$1`,id).Scan(&avatarURL)
 if err==nil && avatarURL!=nil {m.AvatarURL=*avatarURL} else if err!=nil && !errors.Is(err,pgx.ErrNoRows){return nil,err}
 var coverKey *string
 err=s.repo.Pool().QueryRow(ctx,`SELECT cover_key,avatar_x,avatar_y,avatar_zoom,cover_x,cover_y,cover_zoom FROM member_profile_media WHERE user_id=$1`,id).
  Scan(&coverKey,&m.AvatarX,&m.AvatarY,&m.AvatarZoom,&m.CoverX,&m.CoverY,&m.CoverZoom)
 if err==nil {m.HasCover=coverKey!=nil && *coverKey!=""} else if !errors.Is(err,pgx.ErrNoRows) {return nil,err}
 return m,nil
}

// UpdateMemberMediaPosition is restricted to the authenticated member's own ID.
// Position is a display-only crop and never affects the private QR verification payload.
func (s *AuthService) UpdateMemberMediaPosition(ctx context.Context,id,kind string,x,y,zoom int) error {
 if !memberMediaKind(kind)||!memberMediaPositionValid(x,y,zoom){return ErrMemberMediaInvalid}
 tx,err:=s.repo.Pool().Begin(ctx);if err!=nil{return err};defer tx.Rollback(ctx)
 var status string
 if err=tx.QueryRow(ctx,`SELECT status FROM users WHERE id=$1 FOR UPDATE`,id).Scan(&status);err!=nil{return err}
 if status!="active"{return ErrAccountInactive}
 _,err=tx.Exec(ctx,`INSERT INTO member_profile_media(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`,id)
 if err!=nil{return err}
 if kind=="avatar" {
  _,err=tx.Exec(ctx,`UPDATE member_profile_media SET avatar_x=$2,avatar_y=$3,avatar_zoom=$4,updated_at=now() WHERE user_id=$1`,id,x,y,zoom)
 } else {
  _,err=tx.Exec(ctx,`UPDATE member_profile_media SET cover_x=$2,cover_y=$3,cover_zoom=$4,updated_at=now() WHERE user_id=$1`,id,x,y,zoom)
 }
 if err!=nil{return err}
 if err=tx.Commit(ctx);err!=nil{return err}
 return nil
}

// UploadMemberMedia uses trusted server-side image decoding and clean re-encoding
// to strip EXIF, GPS, and other metadata before storing on the existing R2 service.
func (s *AuthService) UploadMemberMedia(ctx context.Context,id,kind string,r io.Reader,store storage.StorageService) (*MemberMedia,error) {
 if !memberMediaKind(kind){return nil,ErrMemberMediaInvalid}
 if store==nil||!store.IsConfigured(){return nil,storage.ErrStorageUnconfigured}
 data,meta,err:=storage.InspectAndValidateImage(r,5*1024*1024)
 if err!=nil{return nil,err}
 // Reject enormous compressed images before full pixel decoding to cap memory use.
 if meta.Width*meta.Height>16000000 || meta.Width>8000 || meta.Height>8000 {return nil,ErrMemberMediaInvalid}
 if kind=="avatar" && (meta.Width<128||meta.Height<128) {return nil,ErrMemberMediaInvalid}
 if kind=="cover" && (meta.Width<640||meta.Height<160) {return nil,ErrMemberMediaInvalid}
 clean,_,_,_,err:=storage.SanitizeAndReencodeImage(data)
 if err!=nil{return nil,ErrMemberMediaInvalid}
 if len(clean)>5*1024*1024{return nil,storage.ErrImageTooLarge}
 var status string
 if err=s.repo.Pool().QueryRow(ctx,`SELECT status FROM users WHERE id=$1`,id).Scan(&status);err!=nil{return nil,err}
 if status!="active"{return nil,ErrAccountInactive}
 key:=fmt.Sprintf("members/profile/%s/%s/%s.jpg",id,kind,strings.ReplaceAll(security.GenerateUUID(),"-",""))
 imageURL,err:=store.UploadObject(ctx,key,"image/jpeg",bytes.NewReader(clean),int64(len(clean)))
 if err!=nil{return nil,err}
 tx,err:=s.repo.Pool().Begin(ctx)
 if err!=nil{_ =store.DeleteObject(ctx,key);return nil,err}
 defer tx.Rollback(ctx)
 if err=tx.QueryRow(ctx,`SELECT status FROM users WHERE id=$1 FOR UPDATE`,id).Scan(&status);err!=nil{_ =store.DeleteObject(ctx,key);return nil,err}
 if status!="active"{_ =store.DeleteObject(ctx,key);return nil,ErrAccountInactive}
 _,err=tx.Exec(ctx,`INSERT INTO member_profiles(id,user_id) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`,security.GenerateUUID(),id)
 if err!=nil{_ =store.DeleteObject(ctx,key);return nil,err}
 _,err=tx.Exec(ctx,`INSERT INTO member_profile_media(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`,id)
 if err!=nil{_ =store.DeleteObject(ctx,key);return nil,err}
 var prior *string
 field:="avatar_key"
 if kind=="cover"{field="cover_key"}
 err=tx.QueryRow(ctx,`SELECT `+field+` FROM member_profile_media WHERE user_id=$1 FOR UPDATE`,id).Scan(&prior)
 if err!=nil{_ =store.DeleteObject(ctx,key);return nil,err}
 if kind=="avatar" {
  _,err=tx.Exec(ctx,`UPDATE member_profile_media SET avatar_key=$2,avatar_x=50,avatar_y=50,avatar_zoom=100,updated_at=now() WHERE user_id=$1`,id,key)
  if err==nil {_,err=tx.Exec(ctx,`UPDATE member_profiles SET avatar_url=$2,updated_at=now(),version=version+1 WHERE user_id=$1`,id,imageURL)}
 } else {
  _,err=tx.Exec(ctx,`UPDATE member_profile_media SET cover_key=$2,cover_url=NULL,cover_x=50,cover_y=50,cover_zoom=100,updated_at=now() WHERE user_id=$1`,id,key)
 }
 if err==nil {_,err=tx.Exec(ctx,`INSERT INTO audit_events(id,actor_id,actor_role,action,resource_type,resource_id,details) SELECT $1,id,primary_role,$2,'member_profile',id,'{"source":"profile_media"}' FROM users WHERE id=$3`,security.GenerateUUID(),"upload_member_"+kind,id)}
 if err==nil {err=tx.Commit(ctx)}
 if err!=nil{_ =store.DeleteObject(ctx,key);return nil,err}
 if prior!=nil&&*prior!=""&&*prior!=key{_ =store.DeleteObject(ctx,*prior)}
 return s.MemberMediaDetails(ctx,id)
}

func (s *AuthService) RemoveMemberMedia(ctx context.Context,id,kind string,store storage.StorageService) error {
 if !memberMediaKind(kind){return ErrMemberMediaInvalid}
 tx,err:=s.repo.Pool().Begin(ctx);if err!=nil{return err};defer tx.Rollback(ctx)
 var status string
 if err=tx.QueryRow(ctx,`SELECT status FROM users WHERE id=$1 FOR UPDATE`,id).Scan(&status);err!=nil{return err}
 if status!="active"{return ErrAccountInactive}
 // Legacy users may have an avatar URL without any media tracking row.
 _,err=tx.Exec(ctx,`INSERT INTO member_profile_media(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`,id)
 if err!=nil{return err}
 var oldKey *string
 if kind=="avatar" {
  err=tx.QueryRow(ctx,`SELECT avatar_key FROM member_profile_media WHERE user_id=$1 FOR UPDATE`,id).Scan(&oldKey)
 } else {
  err=tx.QueryRow(ctx,`SELECT cover_key FROM member_profile_media WHERE user_id=$1 FOR UPDATE`,id).Scan(&oldKey)
 }
 if err!=nil {return ErrMemberMediaUnavailable}
 if kind=="avatar" {
  _,err=tx.Exec(ctx,`UPDATE member_profile_media SET avatar_key=NULL,avatar_x=50,avatar_y=50,avatar_zoom=100,updated_at=now() WHERE user_id=$1`,id)
  if err==nil {_,err=tx.Exec(ctx,`UPDATE member_profiles SET avatar_url=NULL,updated_at=now(),version=version+1 WHERE user_id=$1`,id)}
 } else {
  _,err=tx.Exec(ctx,`UPDATE member_profile_media SET cover_key=NULL,cover_url=NULL,cover_x=50,cover_y=50,cover_zoom=100,updated_at=now() WHERE user_id=$1`,id)
 }
 if err!=nil{return err}
 _,err=tx.Exec(ctx,`INSERT INTO audit_events(id,actor_id,actor_role,action,resource_type,resource_id,details) SELECT $1,id,primary_role,$2,'member_profile',id,'{"source":"profile_media"}' FROM users WHERE id=$3`,security.GenerateUUID(),"remove_member_"+kind,id)
 if err!=nil{return err}
 if err=tx.Commit(ctx);err!=nil{return err}
 if oldKey!=nil&&*oldKey!=""&&store!=nil&&store.IsConfigured(){_ =store.DeleteObject(ctx,*oldKey)}
 return nil
}

// MemberCoverImage returns raw sanitized bytes for authenticated delivery.
// The API never publishes a cover URL or object key.
func (s *AuthService) MemberCoverImage(ctx context.Context,id string,store storage.StorageService) ([]byte,error) {
 if store==nil||!store.IsConfigured(){return nil,storage.ErrStorageUnconfigured}
 var status,key string
 err:=s.repo.Pool().QueryRow(ctx,`SELECT u.status,m.cover_key FROM users u JOIN member_profile_media m ON m.user_id=u.id WHERE u.id=$1`,id).Scan(&status,&key)
 if err!=nil{return nil,ErrMemberMediaUnavailable}
 if status!="active"{return nil,ErrAccountInactive}
 if key==""{return nil,ErrMemberMediaUnavailable}
 return store.ReadObject(ctx,key)
}
