package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ListTransactionRefunds uses Paystack's separate Refund API: transaction/verify
// does not guarantee a refunds array. Caller passes the verified provider ID.
func (p *PaystackGateway) ListTransactionRefunds(ctx context.Context, transactionID string) ([]TransactionRefundSummary, error) {
	if p.secretKey == "" {
		return nil, ErrUnconfiguredProvider
	}
	if _, err := strconv.ParseUint(transactionID, 10, 64); err != nil {
		return nil, ErrMalformedResponse
	}
	result := []TransactionRefundSummary{}
	seen := map[string]bool{}
	for page := 1; page <= 100; page++ {
		q := url.Values{"transaction": {transactionID}, "perPage": {"100"}, "page": {strconv.Itoa(page)}}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/refund?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.secretKey)
		response, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		var body struct {
			Status bool `json:"status"`
			Data   []struct {
				ID          json.Number `json:"id"`
				Transaction json.Number `json:"transaction"`
				Amount      int64       `json:"amount"`
				Currency    string      `json:"currency"`
				Status      string      `json:"status"`
			} `json:"data"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if response.StatusCode != 200 || decodeErr != nil || !body.Status {
			return nil, fmt.Errorf("refund reconciliation unavailable: HTTP %d", response.StatusCode)
		}
		for _, r := range body.Data {
			if r.Transaction.String() != transactionID || r.ID.String() == "" || r.Amount < 0 {
				return nil, ErrMalformedResponse
			}
			if r.Currency != "NGN" {
				return nil, ErrCurrencyMismatch
			}
			if seen[r.ID.String()] {
				continue
			}
			seen[r.ID.String()] = true
			state := "pending"
			switch r.Status {
			case "processed":
				state = "successful"
			case "failed":
				state = "failed"
			}
			result = append(result, TransactionRefundSummary{RefundReference: r.ID.String(), AmountKobo: r.Amount, Status: state})
		}
		if len(body.Data) < 100 {
			return result, nil
		}
	}
	return nil, fmt.Errorf("refund reconciliation exceeds pagination limit; manual review required")
}
