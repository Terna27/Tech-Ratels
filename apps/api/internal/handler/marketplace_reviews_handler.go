package handler

import (
 "encoding/json"
 "errors"
 "net/http"

 "github.com/go-chi/chi/v5"
 "ratels-api/internal/service"
 "ratels-api/internal/storage"
)

func (h *MarketplaceHandler) ListProductReviews(w http.ResponseWriter,r *http.Request) {
 productID:=chi.URLParam(r,"id")
 response,err:=h.marketplaceService.ListProductReviews(r.Context(),productID)
 if errors.Is(err,service.ErrProductNotFound){ErrorJSON(w,http.StatusNotFound,"NOT_FOUND","Product not found",nil);return}
 if err!=nil{ErrorJSON(w,http.StatusInternalServerError,"REVIEWS_UNAVAILABLE","Unable to load product reviews",nil);return}
 if viewer:=GetCurrentUser(r.Context());viewer!=nil{
  for i:=range response.Reviews{response.Reviews[i].IsMine=response.Reviews[i].BuyerID==viewer.ID}
 }
 JSON(w,http.StatusOK,response)
}

func (h *MarketplaceHandler) SubmitProductReview(w http.ResponseWriter,r *http.Request) {
 user:=GetCurrentUser(r.Context())
 var req service.SubmitReviewInput
 if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,16*1024)).Decode(&req);err!=nil {
  ErrorJSON(w,http.StatusBadRequest,"INVALID_REQUEST","Invalid review payload",nil);return
 }
 id,err:=h.marketplaceService.SubmitProductReview(r.Context(),user,chi.URLParam(r,"id"),req)
 if errors.Is(err,service.ErrReviewIneligible){ErrorJSON(w,http.StatusForbidden,"PURCHASE_REQUIRED",err.Error(),nil);return}
 if errors.Is(err,service.ErrReviewInvalid){ErrorJSON(w,http.StatusBadRequest,"INVALID_REVIEW",err.Error(),nil);return}
 if err!=nil{ErrorJSON(w,http.StatusConflict,"REVIEW_EXISTS","Could not create review; one review is allowed per purchased product",nil);return}
 JSON(w,http.StatusCreated,map[string]string{"id":id})
}

func (h *MarketplaceHandler) EditProductReview(w http.ResponseWriter,r *http.Request) {
 var req service.SubmitReviewInput
 if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,16*1024)).Decode(&req);err!=nil{
  ErrorJSON(w,http.StatusBadRequest,"INVALID_REQUEST","Invalid review payload",nil);return
 }
 err:=h.marketplaceService.EditProductReview(r.Context(),GetCurrentUser(r.Context()),chi.URLParam(r,"id"),req)
 if errors.Is(err,service.ErrReviewNotFound){ErrorJSON(w,http.StatusNotFound,"NOT_FOUND","Review not found or editing window expired",nil);return}
 if errors.Is(err,service.ErrReviewInvalid){ErrorJSON(w,http.StatusBadRequest,"INVALID_REVIEW",err.Error(),nil);return}
 if err!=nil{ErrorJSON(w,http.StatusInternalServerError,"UPDATE_FAILED","Unable to update review",nil);return}
 JSON(w,http.StatusOK,map[string]string{"status":"updated"})
}

func (h *MarketplaceHandler) ReplyToProductReview(w http.ResponseWriter,r *http.Request) {
 var req struct { Reply string `json:"reply"` }
 if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,8*1024)).Decode(&req);err!=nil{ErrorJSON(w,http.StatusBadRequest,"INVALID_REQUEST","Invalid response",nil);return}
 err:=h.marketplaceService.ReplyToReview(r.Context(),GetCurrentUser(r.Context()),chi.URLParam(r,"id"),req.Reply)
 if errors.Is(err,service.ErrUnauthorizedSellerAction){ErrorJSON(w,http.StatusForbidden,"FORBIDDEN","Only the approved vendor may respond",nil);return}
 if errors.Is(err,service.ErrReviewInvalid){ErrorJSON(w,http.StatusBadRequest,"INVALID_RESPONSE",err.Error(),nil);return}
 if err!=nil{ErrorJSON(w,http.StatusInternalServerError,"REPLY_FAILED","Unable to reply to review",nil);return}
 JSON(w,http.StatusOK,map[string]string{"status":"replied"})
}

func (h *MarketplaceHandler) ReportProductReview(w http.ResponseWriter,r *http.Request) {
 var req struct { Reason string `json:"reason"` }
 if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,8*1024)).Decode(&req);err!=nil{ErrorJSON(w,http.StatusBadRequest,"INVALID_REQUEST","Invalid report",nil);return}
 err:=h.marketplaceService.ReportReview(r.Context(),GetCurrentUser(r.Context()),chi.URLParam(r,"id"),req.Reason)
 if errors.Is(err,service.ErrReviewInvalid){ErrorJSON(w,http.StatusBadRequest,"INVALID_REPORT",err.Error(),nil);return}
 if errors.Is(err,service.ErrReviewNotFound){ErrorJSON(w,http.StatusConflict,"ALREADY_REPORTED","Review not found or already reported",nil);return}
 if err!=nil{ErrorJSON(w,http.StatusInternalServerError,"REPORT_FAILED","Unable to report review",nil);return}
 JSON(w,http.StatusCreated,map[string]string{"status":"reported"})
}

func (h *MarketplaceHandler) ModerateProductReview(w http.ResponseWriter,r *http.Request) {
 var req struct { Status string `json:"status"` }
 if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,8*1024)).Decode(&req);err!=nil{ErrorJSON(w,http.StatusBadRequest,"INVALID_REQUEST","Invalid moderation payload",nil);return}
 err:=h.marketplaceService.ModerateReview(r.Context(),GetCurrentUser(r.Context()),chi.URLParam(r,"id"),req.Status)
 if errors.Is(err,service.ErrScopeUnauthorized){ErrorJSON(w,http.StatusForbidden,"FORBIDDEN","Super Admin required",nil);return}
 if errors.Is(err,service.ErrReviewInvalid){ErrorJSON(w,http.StatusBadRequest,"INVALID_STATUS",err.Error(),nil);return}
 if errors.Is(err,service.ErrReviewNotFound){ErrorJSON(w,http.StatusNotFound,"NOT_FOUND","Review not found",nil);return}
 if err!=nil{ErrorJSON(w,http.StatusInternalServerError,"MODERATION_FAILED","Unable to moderate review",nil);return}
 JSON(w,http.StatusOK,map[string]string{"status":req.Status})
}

func (h *MarketplaceHandler) UploadReviewImage(w http.ResponseWriter,r *http.Request) {
 user:=GetCurrentUser(r.Context())
 r.Body=http.MaxBytesReader(w,r.Body,6*1024*1024)
 if err:=r.ParseMultipartForm(5*1024*1024);err!=nil{ErrorJSON(w,http.StatusBadRequest,"INVALID_UPLOAD","Upload limit exceeded",nil);return}
 file,header,err:=r.FormFile("file")
 if err!=nil{ErrorJSON(w,http.StatusBadRequest,"MISSING_IMAGE","An image file is required",nil);return}
 defer file.Close()
 result,err:=h.marketplaceService.UploadReviewImage(r.Context(),user,chi.URLParam(r,"id"),file,header.Size)
 if errors.Is(err,service.ErrReviewIneligible){ErrorJSON(w,http.StatusForbidden,"PURCHASE_REQUIRED",err.Error(),nil);return}
 if errors.Is(err,storage.ErrStorageUnconfigured){ErrorJSON(w,http.StatusServiceUnavailable,"STORAGE_UNAVAILABLE","Review image storage is unavailable",nil);return}
 if err!=nil{ErrorJSON(w,http.StatusBadRequest,"INVALID_IMAGE","The image was rejected",nil);return}
 JSON(w,http.StatusCreated,result)
}

func (h *MarketplaceHandler) ListReviewReports(w http.ResponseWriter,r *http.Request) {
 reports,err:=h.marketplaceService.ListReportedReviews(r.Context(),GetCurrentUser(r.Context()))
 if errors.Is(err,service.ErrScopeUnauthorized){ErrorJSON(w,http.StatusForbidden,"FORBIDDEN","Super Admin required",nil);return}
 if err!=nil{ErrorJSON(w,http.StatusInternalServerError,"REPORTS_UNAVAILABLE","Unable to load reported reviews",nil);return}
 JSON(w,http.StatusOK,reports)
}
