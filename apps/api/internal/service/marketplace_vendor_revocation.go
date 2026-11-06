package service

import (
 "context"
 "errors"
 "strings"
 "time"

 "ratels-api/internal/db"
)

func (s *MarketplaceService) RevokeBusiness(ctx context.Context, admin *db.User, id, reason string) (*db.MarketplaceBusiness,error) {
 if admin==nil || !s.isSuperAdminUser(admin,s.getActiveAppointments(ctx,admin.ID)) { return nil,ErrScopeUnauthorized }
 reason=strings.TrimSpace(reason)
 if len(reason)<10 || len(reason)>1000 {return nil,errors.New("revocation requires an administrative reason of 10–1000 characters")}
 existing,err:=s.repo.Queries().GetMarketplaceBusinessByID(ctx,id)
 if err!=nil{return nil,ErrBusinessNotFound}
 if existing.Status=="revoked"{return nil,errors.New("business is already revoked")}
 result,err:=s.repo.Queries().UpdateMarketplaceBusinessStatus(ctx,db.UpdateMarketplaceBusinessStatusParams{
  ID:id,
  Status:"revoked",
  IsVerified:false,
  VerifiedByUserID:TextFromString(admin.ID),
  VerifiedAt:TimestamptzFromTime(time.Now()),
  RejectionReason:TextFromString(reason),
 })
 if err!=nil{return nil,err}
 _=s.audit.Log(ctx,&admin.ID,&admin.PrimaryRole,"revoke_marketplace_business","marketplace_business",id,nil,nil,&reason,nil)
 return &result,nil
}
