package fedapay

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "sk_test", "whsec", 2, zap.NewNop())
}

func TestCreateTransaction_RedirectPayload(t *testing.T) {
	var got map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/transactions" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk_test" {
			t.Errorf("missing bearer token")
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"v1/transaction":{"id":101,"reference":"trx_1","status":"pending","merchant_reference":"PAY-1"}}`))
	})

	tx, err := client.CreateTransaction(CreateTransactionParams{
		Amount:            5500,
		Description:       "Paiement réservation BK-1",
		CallbackURL:       "https://api.test/api/v1/payment/callback",
		MerchantReference: "PAY-1",
		Customer:          &CustomerPayload{FirstName: "Ama", LastName: "Koffi"},
		CustomMetadata:    map[string]string{"booking_id": "bk-1"},
	})
	if err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}
	if tx.ID != 101 || tx.MerchantReference != "PAY-1" {
		t.Errorf("unexpected transaction: %+v", tx)
	}

	if got["callback_url"] != "https://api.test/api/v1/payment/callback" {
		t.Errorf("callback_url = %v", got["callback_url"])
	}
	if got["merchant_reference"] != "PAY-1" {
		t.Errorf("merchant_reference = %v", got["merchant_reference"])
	}
	if got["amount"] != float64(5500) {
		t.Errorf("amount = %v", got["amount"])
	}
	customer, _ := got["customer"].(map[string]any)
	if _, hasPhone := customer["phone_number"]; hasPhone {
		t.Errorf("customer must not carry a phone number in redirect mode: %v", customer)
	}
	if _, hasMode := got["mode"]; hasMode {
		t.Errorf("mode must not be sent in redirect mode")
	}
}

func TestGetTransactionToken(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/transactions/101/token" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"token":"tok_123","url":"https://process.fedapay.com/tok_123"}`))
	})

	resp, err := client.GetTransactionToken(101)
	if err != nil {
		t.Fatalf("GetTransactionToken: %v", err)
	}
	if resp.URL != "https://process.fedapay.com/tok_123" || resp.Token != "tok_123" {
		t.Errorf("unexpected token response: %+v", resp)
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Transaction not found"}`))
	})

	_, err := client.GetTransaction(999)
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("err = %v, want ErrTransactionNotFound", err)
	}
	if err := client.CancelTransaction(999); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("CancelTransaction err = %v, want ErrTransactionNotFound", err)
	}
}

func TestGetTransactionByMerchantReference(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/transactions/merchant/PAY-20261011-ABC" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"v1/transaction":{"id":7,"status":"approved"}}`))
	})

	tx, err := client.GetTransactionByMerchantReference("PAY-20261011-ABC")
	if err != nil {
		t.Fatalf("GetTransactionByMerchantReference: %v", err)
	}
	if tx.ID != 7 || tx.Status != TransactionStatusApproved {
		t.Errorf("unexpected transaction: %+v", tx)
	}
}
