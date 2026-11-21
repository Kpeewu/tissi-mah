package gateway

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	paymentpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/paymentpb"
)

// paymentCallbackTimeout borne l'appel à payment-service (qui interroge FedaPay).
const paymentCallbackTimeout = 15 * time.Second

// paymentCallbackClient est le sous-ensemble du client payment-service utilisé par le callback.
type paymentCallbackClient interface {
	HandlePaymentCallback(ctx context.Context, in *paymentpb.HandlePaymentCallbackRequest, opts ...grpc.CallOption) (*paymentpb.PaymentStatusResponse, error)
}

// paymentCallbackHandler traite le retour de la page de paiement FedaPay
// (GET /api/v1/payment/callback?id=<transaction>&status=<statut>) puis redirige (302)
// vers le deep link mobile : <deepLink>?paymentId=&status=&providerStatus=&bookingStatus=.
// Le paramètre status de FedaPay n'est jamais utilisé : payment-service revérifie la transaction.
// En cas d'erreur, la redirection porte status=error et le mobile appelle verifyPayment.
func paymentCallbackHandler(client paymentCallbackClient, deepLink string, logger *zap.Logger) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		w.Header().Set("Cache-Control", "no-store")

		params := url.Values{}
		transactionID := r.URL.Query().Get("id")
		if id, err := strconv.Atoi(transactionID); err != nil || id <= 0 {
			logger.Warn("fedapay callback: invalid transaction id", zap.String("id", transactionID))
			params.Set("status", "error")
			http.Redirect(w, r, buildDeepLink(deepLink, params), http.StatusFound)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), paymentCallbackTimeout)
		defer cancel()

		resp, err := client.HandlePaymentCallback(ctx, &paymentpb.HandlePaymentCallbackRequest{
			FedapayTransactionId: transactionID,
		})
		if err != nil {
			logger.Warn("fedapay callback: payment-service call failed",
				zap.String("transactionID", transactionID),
				zap.Error(err),
			)
			params.Set("status", "error")
			http.Redirect(w, r, buildDeepLink(deepLink, params), http.StatusFound)
			return
		}

		// Pas de bookingId : la route est publique et les ID FedaPay sont séquentiels,
		// on n'expose que le strict nécessaire au mobile (verifyPayment vérifie la propriété).
		params.Set("paymentId", resp.PaymentId)
		params.Set("status", resp.Status)
		params.Set("providerStatus", resp.ProviderStatus)
		params.Set("bookingStatus", resp.BookingStatus)
		http.Redirect(w, r, buildDeepLink(deepLink, params), http.StatusFound)
	}
}

// buildDeepLink ajoute params à la query du deep link (en conservant une éventuelle query existante).
func buildDeepLink(deepLink string, params url.Values) string {
	u, err := url.Parse(deepLink)
	if err != nil {
		return deepLink + "?" + params.Encode()
	}
	query := u.Query()
	for key, values := range params {
		for _, v := range values {
			query.Add(key, v)
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}
