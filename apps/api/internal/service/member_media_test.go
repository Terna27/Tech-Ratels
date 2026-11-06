package service_test

import (
 "bytes"
 "context"
 "fmt"
 "image"
 "image/color"
 "image/jpeg"
 "strings"
 "testing"
 "time"

 "ratels-api/internal/service"
 "ratels-api/internal/storage"
)

func testMemberPhoto(t *testing.T, width,height int) []byte {
 t.Helper()
 img:=image.NewRGBA(image.Rect(0,0,width,height))
 for y:=0;y<height;y++{for x:=0;x<width;x++{img.Set(x,y,color.RGBA{R:25,G:110,B:80,A:255})}}
 var out bytes.Buffer
 if err:=jpeg.Encode(&out,img,&jpeg.Options{Quality:86});err!=nil{t.Fatal(err)}
 return out.Bytes()
}

func TestMemberMediaSelfServiceAndCardPhoto(t *testing.T) {
 env:=setupTestEnv(t)
 defer env.Pool.Close()
 ctx:=context.Background()
 member:=createRegisteredUser(t,env,fmt.Sprintf("photo_member_%d",time.Now().UnixNano()),"member")
 store:=storage.NewDevSandboxStorageService("https://media.example.org")
 before,err:=env.AuthService.MemberMediaDetails(ctx,member.ID)
 if err!=nil||before.HasCover{t.Fatalf("unexpected initial member media %+v %v",before,err)}
 if _,err:=env.AuthService.UploadMemberMedia(ctx,member.ID,"avatar",strings.NewReader("not a photo"),store);err==nil {
  t.Fatal("non-image upload was accepted")
 }
 avatar,err:=env.AuthService.UploadMemberMedia(ctx,member.ID,"avatar",bytes.NewReader(testMemberPhoto(t,256,256)),store)
 if err!=nil||!strings.Contains(avatar.AvatarURL,"members/profile/"+member.ID+"/avatar/"){t.Fatalf("avatar upload failed: %+v, %v",avatar,err)}
 card,err:=env.AuthService.MemberCard(ctx,member.ID)
 if err!=nil||card.Photo!=avatar.AvatarURL{t.Fatalf("membership card avatar did not sync: %+v err %v",card,err)}
 cover,err:=env.AuthService.UploadMemberMedia(ctx,member.ID,"cover",bytes.NewReader(testMemberPhoto(t,900,240)),store)
 if err!=nil||!cover.HasCover{t.Fatalf("cover upload failed: %+v err %v",cover,err)}
 data,err:=env.AuthService.MemberCoverImage(ctx,member.ID,store)
 if err!=nil||len(data)==0{t.Fatalf("protected cover read failed: %v",err)}
 if err:=env.AuthService.UpdateMemberMediaPosition(ctx,member.ID,"cover",25,72,125);err!=nil{t.Fatalf("cover reposition failed: %v",err)}
 got,err:=env.AuthService.MemberMediaDetails(ctx,member.ID)
 if err!=nil||got.CoverX!=25||got.CoverY!=72||got.CoverZoom!=125{t.Fatalf("position did not persist %+v %v",got,err)}
 if err:=env.AuthService.UpdateMemberMediaPosition(ctx,member.ID,"cover",-1,50,100);err!=service.ErrMemberMediaInvalid{
  t.Fatalf("invalid position accepted: %v",err)
 }
 if err:=env.AuthService.RemoveMemberMedia(ctx,member.ID,"avatar",store);err!=nil{t.Fatalf("avatar removal failed: %v",err)}
 after,err:=env.AuthService.MemberMediaDetails(ctx,member.ID)
 if err!=nil||after.AvatarURL!=""{t.Fatalf("avatar persisted after removal: %+v %v",after,err)}
 card,err=env.AuthService.MemberCard(ctx,member.ID)
 if err!=nil||card.Photo!=""{t.Fatalf("membership card kept deleted avatar: %+v %v",card,err)}
 if err:=env.AuthService.RemoveMemberMedia(ctx,member.ID,"cover",store);err!=nil{t.Fatalf("cover removal failed: %v",err)}
 if _,err:=env.AuthService.MemberCoverImage(ctx,member.ID,store);err!=service.ErrMemberMediaUnavailable{
  t.Fatalf("removed cover still available: %v",err)
 }
 if _,err:=env.AuthService.UploadMemberMedia(ctx,member.ID,"cover",bytes.NewReader(testMemberPhoto(t,200,200)),store);err!=service.ErrMemberMediaInvalid{
  t.Fatalf("too-small cover accepted: %v",err)
 }
}
