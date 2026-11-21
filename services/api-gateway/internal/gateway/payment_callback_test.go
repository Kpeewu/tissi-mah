package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	paymentpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/paymentpb"
)

type fakePaymentCallbackClient struct {
	resp      *paymentpb.PaymentStatusResponse
	err       error
	gotTxID   string
	callCount int
}

func (f *fakePaymentCallbackClient) HandlePaymentCallback(_ context.Context, in *paymentpb.HandlePaymentCallbackRequest, _ ...grpc.CallOption) (*paymentpb.PaymentStatusResponse, error) {
	f.callCount++
	f.gotTxID = in.FedapayTransactionId
	return f.resp, f.err
}

func doCallback(t *testing.T, client paymentCallbackClient, deepLink, rawQuery string) (*url.URL, *httptest.ResponseRecorder) {
	t.Helper()
	handler := paymentCallbackHandler(client, deepLink, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payment/callback?"+rawQuery, nil)
	rec := httptest.NewRecorder()
	handler(rec, req, nil)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("invalid Location header: %v", err)
	}
	return location, rec
}

func TestPaymentCallbackHandler_RedirectsWithVerifiedStatus(t *testing.T) {
	client := &fakePaymentCallbackClient{resp: &paymentpb.PaymentStatusResponse{
		PaymentId:      "pay-1",
		BookingId:      "bk-1",
		Status:         "held",
		ProviderStatus: "approved",
		BookingStatus:  "pendingApproval",
	}}

	// Le status=canceled de FedaPay est ignoré : seul le statut revérifié compte
	location, rec := doCallback(t, client, "tissimah://payment/result", "id=258&status=canceled")

	if client.gotTxID != "258" {
		t.Errorf("transaction id = %q, want 258", client.gotTxID)
	}
	if location.Scheme != "tissimah" || location.Host != "payment" || location.Path != "/result" {
		t.Errorf("unexpected deep link: %s", location)
	}
	q := location.Query()
	for key, want := range map[string]string{
		"paymentId": "pay-1", "status": "held",
		"providerStatus": "approved", "bookingStatus": "pendingApproval",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if q.Has("bookingId") {
		t.Errorf("bookingId must not be exposed on the public callback")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
}

func TestPaymentCallbackHandler_InvalidTransactionID(t *testing.T) {
	for _, query := range []string{"", "id=abc", "id=-3", "id=0"} {
		client := &fakePaymentCallbackClient{}
		location, _ := doCallback(t, client, "tissimah://payment/result", query)

		if client.callCount != 0 {
			t.Errorf("query %q: payment-service should not be called", query)
		}
		if got := location.Query().Get("status"); got != "error" {
			t.Errorf("query %q: status = %q, want error", query, got)
		}
	}
}

func TestPaymentCallbackHandler_PaymentServiceError(t *testing.T) {
	client := &fakePaymentCallbackClient{err: errors.New("unavailable")}
	location, _ := doCallback(t, client, "tissimah://payment/result?source=fedapay", "id=42&status=approved")

	q := location.Query()
	if q.Get("status") != "error" {
		t.Errorf("status = %q, want error", q.Get("status"))
	}
	if q.Get("source") != "fedapay" {
		t.Errorf("existing deep link query lost: %s", location)
	}
}
