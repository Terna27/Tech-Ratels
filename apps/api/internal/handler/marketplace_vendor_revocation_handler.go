package handler

import (
 "encoding/json"
 "errors"
 "net/http"

 "github.com/go-chi/chi/v5"
 "ratels-api/internal/service"
)

func (h *MarketplaceHandler) RevokeBusiness(w http.ResponseWriter,r *http.Request) {
 var request struct {Reason string `json:"reason"`}
 if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,8*1024)).Decode(&request);err!=nil {
  ErrorJSON(w,http.StatusBadRequest,"INVALID_REQUEST","A revocation reason is required",nil);return
 }
 result,err:=h.marketplaceService.RevokeBusiness(r.Context(),GetCurrentUser(r.Context()),chi.URLParam(r,"id"),request.Reason)
 if errors.Is(err,service.ErrScopeUnauthorized){ErrorJSON(w,http.StatusForbidden,"FORBIDDEN","Super Admin approval required",nil);return}
 if errors.Is(err,service.ErrBusinessNotFound){ErrorJSON(w,http.StatusNotFound,"NOT_FOUND",err.Error(),nil);return}
 if err!=nil{ErrorJSON(w,http.StatusBadRequest,"REVOCATION_FAILED",err.Error(),nil);return}
 JSON(w,http.StatusOK,result)
}
