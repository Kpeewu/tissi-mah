package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

// Statuts booking-service utilisés par le flux de collecte.
const (
	bookingStatusPaymentPending = "paymentPending"
	bookingStatusPaymentFailed  = "paymentFailed"
	bookingPaymentMethodCash    = "cash"
)

// =============================================================================
// CreatePayment — collecte FedaPay avec redirection
// =============================================================================

// CreatePayment crée la transaction FedaPay de la réservation et retourne le lien de la page de paiement.
//
// Le montant est celui de la réservation (le champ Amount du client est ignoré). La fenêtre de paiement
// court depuis la création de la réservation : passé PAYMENT_TIMEOUT_MINUTES, la réservation n'est plus
// payable et le worker d'expiration supprime la transaction FedaPay et libère les places.
func (s *paymentServiceImpl) CreatePayment(ctx context.Context, input *serviceInterfaces.CreatePaymentInput) (*serviceInterfaces.CreatePaymentResult, error) {
	s.logger.Info("creating payment",
		zap.String("bookingID", input.BookingID),
		zap.String("method", input.PaymentMethod),
	)

	booking, refundPhone, expiresAt, err := s.validatePayableBooking(ctx, input)
	if err != nil {
		return nil, err
	}

	// Reprise : un paiement pending existe déjà → même lien
	if existing, err := s.paymentReadRepo.GetPendingByBookingID(ctx, booking.BookingID); err == nil {
		return s.reusePendingPayment(ctx, existing)
	} else if !errors.Is(err, paymentErrors.ErrorPaymentNotFound) {
		return nil, err
	}

	// Un paiement held existe (déjà payé) → pas de nouvelle collecte
	hasActive, err := s.paymentReadRepo.HasActivePayment(ctx, booking.BookingID)
	if err != nil {
		return nil, paymentErrors.ErrorInternalServer
	}
	if hasActive {
		return nil, paymentErrors.ErrorDuplicatePayment
	}

	paymentID := uuid.New().String()
	paymentRef := fmt.Sprintf("PAY-%s-%s", time.Now().UTC().Format("20060102"), generateAlphanumeric(6))

	transaction, err := s.fedapayClient.CreateTransaction(fedapay.CreateTransactionParams{
		Amount:            booking.TotalAmount,
		Description:       fmt.Sprintf("Paiement réservation %s", booking.BookingReference),
		CallbackURL:       s.cfg.FedaPay.CallbackURL,
		MerchantReference: paymentRef,
		Customer:          s.fedapayCustomer(ctx, booking.PassengerID),
		CustomMetadata: map[string]string{
			"payment_id": paymentID,
			"booking_id": booking.BookingID,
		},
	})
	if err != nil {
		s.logger.Error("fedapay create transaction failed", zap.Error(err))
		return nil, paymentErrors.ErrorFedaPayAPIError
	}

	tokenResp, err := s.fedapayClient.GetTransactionToken(transaction.ID)
	if err != nil || tokenResp.URL == "" {
		s.logger.Error("fedapay get transaction token failed", zap.Int("transactionID", transaction.ID), zap.Error(err))
		s.cancelOrphanTransaction(transaction.ID)
		return nil, paymentErrors.ErrorFedaPayAPIError
	}

	payment := &domain.Payment{
		PaymentID:             paymentID,
		BookingID:             booking.BookingID,
		TripID:                booking.TripID,
		Amount:                booking.TotalAmount,
		PaymentMethod:         domain.PaymentMobileMoney,
		PaymentProvider:       "fedapay",
		Status:                domain.PaymentStatusPending,
		ExternalTransactionID: strconv.Itoa(transaction.ID),
		PaymentReference:      paymentRef,
		PassengerPhoneNumber:  refundPhone,
		PaymentURL:            tokenResp.URL,
		ProviderStatus:        fedapay.TransactionStatusPending,
		ExpiresAt:             expiresAt,
	}

	if err := s.paymentWriteRepo.CreatePayment(ctx, payment); err != nil {
		s.cancelOrphanTransaction(transaction.ID)
		if errors.Is(err, paymentErrors.ErrorDuplicatePayment) {
			// Requête concurrente : un autre paiement vient d'être créé pour cette réservation
			if existing, getErr := s.paymentReadRepo.GetPendingByBookingID(ctx, booking.BookingID); getErr == nil {
				return s.reusePendingPayment(ctx, existing)
			}
			return nil, paymentErrors.ErrorDuplicatePayment
		}
		s.logger.Error("save payment failed", zap.Error(err))
		return nil, paymentErrors.ErrorInternalServer
	}

	if s.cache != nil {
		s.cache.InvalidatePaymentByBookingID(ctx, booking.BookingID)
	}

	s.logger.Info("payment created, awaiting redirection",
		zap.String("paymentID", paymentID),
		zap.String("bookingID", booking.BookingID),
		zap.Int("transactionID", transaction.ID),
		zap.Time("expiresAt", expiresAt),
	)

	return &serviceInterfaces.CreatePaymentResult{
		PaymentID:        paymentID,
		PaymentReference: paymentRef,
		Status:           string(domain.PaymentStatusPending),
		PaymentURL:       tokenResp.URL,
		ExpiresAt:        expiresAt.Format(time.RFC3339),
		Amount:           booking.TotalAmount,
	}, nil
}

// validatePayableBooking vérifie la requête et la réservation avant création de la collecte.
// Retourne la réservation, le numéro de remboursement normalisé et la fin de la fenêtre de paiement.
func (s *paymentServiceImpl) validatePayableBooking(ctx context.Context, input *serviceInterfaces.CreatePaymentInput) (*client.BookingDetails, string, time.Time, error) {
	if input.BookingID == "" || input.PaymentMethod != string(domain.PaymentMobileMoney) {
		return nil, "", time.Time{}, paymentErrors.ErrorInvalidInput
	}
	refundPhone, ok := normalizeTogoPhoneNumber(input.PassengerPhoneNumber)
	if !ok {
		return nil, "", time.Time{}, paymentErrors.ErrorInvalidInput
	}

	booking, err := s.getBooking(ctx, input.BookingID)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if err := s.checkBookingOwnership(ctx, booking); err != nil {
		return nil, "", time.Time{}, err
	}
	if input.TripID != "" && input.TripID != booking.TripID {
		return nil, "", time.Time{}, paymentErrors.ErrorInvalidInput
	}
	if booking.PaymentMethod == bookingPaymentMethodCash || booking.Status != bookingStatusPaymentPending || booking.TotalAmount <= 0 {
		return nil, "", time.Time{}, paymentErrors.ErrorBookingNotPayable
	}
	if input.Amount != 0 && input.Amount != booking.TotalAmount {
		s.logger.Warn("client amount ignored, using booking amount",
			zap.String("bookingID", booking.BookingID),
			zap.Int("clientAmount", input.Amount),
			zap.Int("bookingAmount", booking.TotalAmount),
		)
	}

	expiresAt := s.paymentExpiresAt(booking)
	if !time.Now().UTC().Before(expiresAt) {
		return nil, "", time.Time{}, paymentErrors.ErrorBookingPaymentExpired
	}

	return booking, refundPhone, expiresAt, nil
}

// reusePendingPayment retourne le lien d'un paiement pending existant (regénéré s'il manque).
func (s *paymentServiceImpl) reusePendingPayment(ctx context.Context, payment *domain.Payment) (*serviceInterfaces.CreatePaymentResult, error) {
	if !time.Now().UTC().Before(payment.ExpiresAt) {
		return nil, paymentErrors.ErrorBookingPaymentExpired
	}

	paymentURL := payment.PaymentURL
	if paymentURL == "" {
		externalID, err := strconv.Atoi(payment.ExternalTransactionID)
		if err != nil {
			return nil, paymentErrors.ErrorInternalServer
		}
		tokenResp, err := s.fedapayClient.GetTransactionToken(externalID)
		if err != nil || tokenResp.URL == "" {
			s.logger.Error("fedapay regenerate token failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
			return nil, paymentErrors.ErrorFedaPayAPIError
		}
		paymentURL = tokenResp.URL
		if err := s.paymentWriteRepo.UpdatePaymentURL(ctx, payment.PaymentID, paymentURL); err != nil {
			s.logger.Warn("persist regenerated payment url failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
		}
	}

	s.logger.Info("reusing pending payment", zap.String("paymentID", payment.PaymentID))

	return &serviceInterfaces.CreatePaymentResult{
		PaymentID:        payment.PaymentID,
		PaymentReference: payment.PaymentReference,
		Status:           string(payment.Status),
		PaymentURL:       paymentURL,
		ExpiresAt:        payment.ExpiresAt.Format(time.RFC3339),
		Reused:           true,
		Amount:           payment.Amount,
	}, nil
}

// =============================================================================
// VerifyPayment / CancelPayment / HandlePaymentCallback / CancelPaymentForBooking
// =============================================================================

// VerifyPayment resynchronise le paiement avec FedaPay. Le mobile l'appelle au retour de la page
// de paiement (deep link ou fermeture du navigateur) : c'est la source de vérité côté client.
func (s *paymentServiceImpl) VerifyPayment(ctx context.Context, paymentID string) (*serviceInterfaces.PaymentSyncResult, error) {
	if paymentID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}

	payment, err := s.paymentReadRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	booking, err := s.getBooking(ctx, payment.BookingID)
	if err != nil {
		return nil, err
	}
	if err := s.checkBookingOwnership(ctx, booking); err != nil {
		return nil, err
	}

	if err := s.syncWithProvider(ctx, payment); err != nil {
		return nil, err
	}

	return s.buildSyncResult(ctx, payment.PaymentID)
}

// CancelPayment abandonne un paiement pending à la demande du passager : la transaction FedaPay
// est supprimée et la réservation passe en paymentFailed (places libérées).
// Si FedaPay a déjà approuvé le paiement, l'approbation est appliquée et rien n'est annulé.
func (s *paymentServiceImpl) CancelPayment(ctx context.Context, paymentID string) (*serviceInterfaces.PaymentSyncResult, error) {
	if paymentID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}

	payment, err := s.paymentReadRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	booking, err := s.getBooking(ctx, payment.BookingID)
	if err != nil {
		return nil, err
	}
	if err := s.checkBookingOwnership(ctx, booking); err != nil {
		return nil, err
	}

	switch payment.Status {
	case domain.PaymentStatusPending:
		if err := s.cancelPendingPayment(ctx, payment, "Paiement annulé par le passager", true); err != nil {
			return nil, err
		}
	case domain.PaymentStatusHeld, domain.PaymentStatusFailed:
		// Déjà payé ou déjà annulé : on retourne simplement l'état courant
	default:
		return nil, paymentErrors.ErrorPaymentNotCancellable
	}

	return s.buildSyncResult(ctx, payment.PaymentID)
}

// HandlePaymentCallback traite le retour de la page FedaPay. Le statut présent dans l'URL
// n'est jamais utilisé : il est revérifié auprès de FedaPay.
func (s *paymentServiceImpl) HandlePaymentCallback(ctx context.Context, fedapayTransactionID string) (*serviceInterfaces.PaymentSyncResult, error) {
	if id, err := strconv.Atoi(fedapayTransactionID); err != nil || id <= 0 {
		return nil, paymentErrors.ErrorInvalidInput
	}

	payment, err := s.paymentReadRepo.GetByExternalTransactionID(ctx, fedapayTransactionID)
	if err != nil {
		return nil, err
	}

	if err := s.syncWithProvider(ctx, payment); err != nil {
		return nil, err
	}

	return s.buildSyncResult(ctx, payment.PaymentID)
}

// CancelPaymentForBooking annule le paiement pending d'une réservation que le passager annule
// (appel interne de booking-service, avant de libérer les places). booking-service gère lui-même
// la réservation : FailPayment n'est pas rappelé. Si FedaPay a déjà approuvé le paiement,
// l'approbation est appliquée et le résultat (held) permet à booking-service de demander un remboursement.
func (s *paymentServiceImpl) CancelPaymentForBooking(ctx context.Context, bookingID string, reason string) (*serviceInterfaces.PaymentSyncResult, error) {
	if bookingID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}

	payment, err := s.paymentReadRepo.GetPendingByBookingID(ctx, bookingID)
	if err != nil {
		if errors.Is(err, paymentErrors.ErrorPaymentNotFound) {
			// Aucune collecte en cours : rien à annuler
			return &serviceInterfaces.PaymentSyncResult{BookingID: bookingID, IsFinal: true}, nil
		}
		return nil, err
	}

	if reason == "" {
		reason = "Réservation annulée"
	}
	if err := s.cancelPendingPayment(ctx, payment, reason, false); err != nil {
		return nil, err
	}

	return s.buildSyncResult(ctx, payment.PaymentID)
}

// =============================================================================
// Synchronisation FedaPay et transitions communes
// =============================================================================

// syncWithProvider lit le statut FedaPay d'un paiement non final et applique la transition.
// Les paiements failed sont aussi vérifiés pour rattraper une approbation tardive.
func (s *paymentServiceImpl) syncWithProvider(ctx context.Context, payment *domain.Payment) error {
	if payment.Status != domain.PaymentStatusPending && payment.Status != domain.PaymentStatusFailed {
		return nil
	}

	providerStatus, err := s.fetchProviderStatus(payment)
	if err != nil {
		return err
	}

	_, err = s.applyProviderStatus(ctx, payment, providerStatus)
	return err
}

// fetchProviderStatus retourne le statut FedaPay de la transaction d'un paiement
// (TransactionStatusDeleted si la transaction n'existe plus).
func (s *paymentServiceImpl) fetchProviderStatus(payment *domain.Payment) (string, error) {
	externalID, err := strconv.Atoi(payment.ExternalTransactionID)
	if err != nil {
		s.logger.Error("invalid external transaction ID",
			zap.String("paymentID", payment.PaymentID),
			zap.String("externalID", payment.ExternalTransactionID),
		)
		return "", paymentErrors.ErrorInternalServer
	}

	transaction, err := s.fedapayClient.GetTransaction(externalID)
	if err != nil {
		if errors.Is(err, fedapay.ErrTransactionNotFound) {
			return fedapay.TransactionStatusDeleted, nil
		}
		s.logger.Error("get transaction from FedaPay failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
		return "", paymentErrors.ErrorFedaPayAPIError
	}

	return transaction.Status, nil
}

// applyProviderStatus applique un statut FedaPay (webhook, callback, vérification mobile ou worker)
// au paiement et à la réservation. Idempotent : les transitions sont protégées par verrouillage optimiste.
//
//	approved / transferred → pending|failed → held, réservation confirmée (ou remboursement si plus de places
//	                         ou si la réservation n'est plus confirmable : annulée, rejetée, etc.)
//	declined / canceled    → reste pending (nouvelle tentative possible sur la page FedaPay)
//	expired / deleted      → pending → failed, réservation paymentFailed (places libérées)
func (s *paymentServiceImpl) applyProviderStatus(ctx context.Context, payment *domain.Payment, providerStatus string) (domain.PaymentStatus, error) {
	switch providerStatus {
	case fedapay.TransactionStatusApproved, fedapay.TransactionStatusTransferred:
		return s.applyApproval(ctx, payment)

	case fedapay.TransactionStatusDeclined, fedapay.TransactionStatusCanceled:
		if payment.Status == domain.PaymentStatusPending {
			if err := s.paymentWriteRepo.UpdateProviderStatus(ctx, payment.PaymentID, providerStatus); err != nil {
				return payment.Status, err
			}
			s.invalidatePaymentCache(ctx, payment.BookingID)
			s.logger.Info("payment attempt not successful, retry possible",
				zap.String("paymentID", payment.PaymentID),
				zap.String("providerStatus", providerStatus),
			)
		}
		return payment.Status, nil

	case fedapay.TransactionStatusExpired, fedapay.TransactionStatusDeleted:
		if payment.Status != domain.PaymentStatusPending {
			return payment.Status, nil
		}
		reason := fmt.Sprintf("FedaPay: transaction %s", providerStatus)
		if err := s.failPendingPayment(ctx, payment, reason, providerStatus, true); err != nil {
			return payment.Status, err
		}
		return domain.PaymentStatusFailed, nil

	default:
		// pending (ou statut inconnu) : rien à faire
		return payment.Status, nil
	}
}

// applyApproval passe le paiement en held et rattache l'argent à la réservation.
// Le passager est remboursé intégralement si la réservation a été libérée entre-temps et que le trajet
// est complet, ou si elle n'est plus confirmable (annulée, rejetée, etc.).
func (s *paymentServiceImpl) applyApproval(ctx context.Context, payment *domain.Payment) (domain.PaymentStatus, error) {
	switch payment.Status {
	case domain.PaymentStatusPending:
		if err := s.paymentWriteRepo.UpdatePaymentStatus(ctx, payment.PaymentID, domain.PaymentStatusHeld, payment.ExternalTransactionID); err != nil {
			if errors.Is(err, paymentErrors.ErrorPaymentAlreadyProcessed) {
				s.logger.Info("payment already processed concurrently, skipping", zap.String("paymentID", payment.PaymentID))
				return payment.Status, nil
			}
			return payment.Status, err
		}
		if err := s.paymentWriteRepo.UpdateProviderStatus(ctx, payment.PaymentID, fedapay.TransactionStatusApproved); err != nil {
			s.logger.Warn("update provider status failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
		}

	case domain.PaymentStatusFailed:
		// Approbation tardive (après expiration ou annulation) : l'argent est encaissé, il doit être
		// rattaché à la réservation ou remboursé.
		s.logger.Warn("late FedaPay approval on failed payment",
			zap.String("paymentID", payment.PaymentID),
			zap.String("bookingID", payment.BookingID),
		)
		if err := s.paymentWriteRepo.MarkFailedPaymentHeld(ctx, payment.PaymentID); err != nil {
			if errors.Is(err, paymentErrors.ErrorPaymentAlreadyProcessed) {
				return payment.Status, nil
			}
			s.logger.Error("late approval: mark failed payment held failed — remboursement manuel requis",
				zap.String("paymentID", payment.PaymentID), zap.Error(err))
			return payment.Status, err
		}

	default:
		// held / released / paidOut / refunded : déjà traité
		return payment.Status, nil
	}

	s.invalidatePaymentCache(ctx, payment.BookingID)

	bookingStatus, err := s.bookingClient.ConfirmPayment(ctx, payment.BookingID, payment.ExternalTransactionID)
	switch {
	case err == nil:
		s.logger.Info("payment held, booking confirmed",
			zap.String("paymentID", payment.PaymentID),
			zap.String("bookingStatus", bookingStatus),
		)
	case errors.Is(err, paymentErrors.ErrorPaymentAlreadyProcessed):
		s.logger.Info("booking already confirmed", zap.String("bookingID", payment.BookingID))
	case errors.Is(err, paymentErrors.ErrorNoSeatsAvailable):
		// Les places ont été réattribuées : remboursement intégral
		s.logger.Warn("late approval but no seats left, refunding passenger",
			zap.String("paymentID", payment.PaymentID),
			zap.String("bookingID", payment.BookingID),
		)
		return s.refundUnconfirmablePayment(ctx, payment.PaymentID, domain.RefundReasonSeatsUnavailable), nil
	case errors.Is(err, paymentErrors.ErrorBookingNotConfirmable):
		// Réservation annulée, rejetée, etc. : l'argent ne peut plus y être rattaché
		return s.handleUnconfirmableBooking(ctx, payment), nil
	default:
		// booking-service indisponible : le paiement reste held, le sweeper booking-service rattrapera
		s.logger.Error("confirm payment to booking-service failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
	}

	s.publishPaymentEvent(ctx, payment, notification.PaymentCompleted, map[string]string{
		"amount": fmt.Sprintf("%d", payment.Amount),
	})

	return domain.PaymentStatusHeld, nil
}

// handleUnconfirmableBooking traite une approbation FedaPay pour une réservation que booking-service
// refuse de confirmer (statut incompatible). Si la réservation est définitivement close (annulée,
// rejetée, etc.), le passager est remboursé intégralement. PAYMENT_COMPLETED n'est jamais publié.
func (s *paymentServiceImpl) handleUnconfirmableBooking(
	ctx context.Context, payment *domain.Payment,
) domain.PaymentStatus {
	booking, err := s.bookingClient.GetBookingDetails(ctx, payment.BookingID)
	if err != nil {
		s.logger.Error("unconfirmable booking: get booking details failed — remboursement manuel requis",
			zap.String("paymentID", payment.PaymentID),
			zap.String("bookingID", payment.BookingID),
			zap.Error(err),
		)
		return domain.PaymentStatusHeld
	}
	if booking.Status == bookingStatusPaymentPending || booking.Status == bookingStatusPaymentFailed {
		// Statut modifié pendant la confirmation (course) : la réservation reste payable, pas de remboursement
		s.logger.Error("unconfirmable booking is still payable — rattachement manuel requis",
			zap.String("paymentID", payment.PaymentID),
			zap.String("bookingID", payment.BookingID),
			zap.String("bookingStatus", booking.Status),
		)
		return domain.PaymentStatusHeld
	}

	s.logger.Warn("approval for a booking that can no longer be confirmed, refunding passenger",
		zap.String("paymentID", payment.PaymentID),
		zap.String("bookingID", payment.BookingID),
		zap.String("bookingStatus", booking.Status),
	)
	return s.refundUnconfirmablePayment(ctx, payment.PaymentID, domain.RefundReasonBookingUnavailable)
}

// refundUnconfirmablePayment rembourse intégralement un paiement held dont la réservation ne peut plus
// être confirmée et retourne le statut résultant (held si le remboursement échoue : traitement manuel).
func (s *paymentServiceImpl) refundUnconfirmablePayment(
	ctx context.Context, paymentID string, reason domain.RefundReason,
) domain.PaymentStatus {
	if err := s.createFullRefund(ctx, paymentID, reason); err != nil {
		s.logger.Error("refund of unconfirmable payment failed — remboursement manuel requis",
			zap.String("paymentID", paymentID),
			zap.String("reason", string(reason)),
			zap.Error(err),
		)
		return domain.PaymentStatusHeld
	}
	return domain.PaymentStatusRefunded
}

// createFullRefund crée le remboursement intégral (sous-total + frais de service) d'un paiement held.
func (s *paymentServiceImpl) createFullRefund(ctx context.Context, paymentID string, reason domain.RefundReason) error {
	payment, err := s.paymentReadRepo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}
	booking, err := s.bookingClient.GetBookingDetails(ctx, payment.BookingID)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.createRefundForPayment(ctx, payment, &serviceInterfaces.RequestRefundInput{
		BookingID:         payment.BookingID,
		RefundReason:      string(reason),
		OriginalAmount:    booking.Subtotal,
		ServiceFee:        booking.ServiceFee,
		DepartureDatetime: now,
		CancelledAt:       now,
	})
	return err
}

// cancelPendingPayment supprime la transaction FedaPay d'un paiement pending puis le marque failed.
// Si FedaPay a déjà approuvé le paiement, l'approbation est appliquée à la place.
// notifyBooking=false quand booking-service est à l'origine de l'annulation.
func (s *paymentServiceImpl) cancelPendingPayment(ctx context.Context, payment *domain.Payment, reason string, notifyBooking bool) error {
	providerStatus, err := s.fetchProviderStatus(payment)
	if err != nil {
		return err
	}
	if providerStatus == fedapay.TransactionStatusApproved || providerStatus == fedapay.TransactionStatusTransferred {
		_, err := s.applyApproval(ctx, payment)
		return err
	}

	if providerStatus != fedapay.TransactionStatusDeleted {
		externalID, _ := strconv.Atoi(payment.ExternalTransactionID) //nolint:errcheck // validé par fetchProviderStatus
		if err := s.fedapayClient.CancelTransaction(externalID); err != nil && !errors.Is(err, fedapay.ErrTransactionNotFound) {
			// La suppression a échoué : revérifier, l'utilisateur a peut-être payé entre-temps
			s.logger.Warn("delete FedaPay transaction failed, re-checking status",
				zap.String("paymentID", payment.PaymentID), zap.Error(err))
			recheck, recheckErr := s.fetchProviderStatus(payment)
			if recheckErr != nil {
				return recheckErr
			}
			if recheck == fedapay.TransactionStatusApproved || recheck == fedapay.TransactionStatusTransferred {
				_, err := s.applyApproval(ctx, payment)
				return err
			}
			if recheck != fedapay.TransactionStatusDeleted && recheck != fedapay.TransactionStatusExpired {
				return paymentErrors.ErrorFedaPayAPIError
			}
		}
	}

	return s.failPendingPayment(ctx, payment, reason, fedapay.TransactionStatusDeleted, notifyBooking)
}

// failPendingPayment marque un paiement pending comme échoué, libère la réservation
// (si notifyBooking) et notifie le passager.
func (s *paymentServiceImpl) failPendingPayment(ctx context.Context, payment *domain.Payment, reason string, providerStatus string, notifyBooking bool) error {
	if err := s.paymentWriteRepo.MarkPaymentFailed(ctx, payment.PaymentID, reason); err != nil {
		if errors.Is(err, paymentErrors.ErrorPaymentAlreadyProcessed) {
			s.logger.Info("payment already processed concurrently, skipping", zap.String("paymentID", payment.PaymentID))
			return nil
		}
		return err
	}
	if err := s.paymentWriteRepo.UpdateProviderStatus(ctx, payment.PaymentID, providerStatus); err != nil {
		s.logger.Warn("update provider status failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
	}

	s.invalidatePaymentCache(ctx, payment.BookingID)

	if notifyBooking {
		// Restaure les places (Redis + legs trips) et passe la réservation en paymentFailed
		if err := s.bookingClient.FailPayment(ctx, payment.BookingID, reason); err != nil {
			s.logger.Error("fail payment to booking-service failed",
				zap.String("paymentID", payment.PaymentID),
				zap.String("bookingID", payment.BookingID),
				zap.Error(err),
			)
		}
	}

	s.publishPaymentEvent(ctx, payment, notification.PaymentFailed, nil)

	s.logger.Info("payment failed", zap.String("paymentID", payment.PaymentID), zap.String("reason", reason))
	return nil
}

// =============================================================================
// Helpers
// =============================================================================

// buildSyncResult relit le paiement et la réservation pour construire la réponse au mobile.
func (s *paymentServiceImpl) buildSyncResult(ctx context.Context, paymentID string) (*serviceInterfaces.PaymentSyncResult, error) {
	payment, err := s.paymentReadRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	var bookingStatus string
	if booking, err := s.bookingClient.GetBookingDetails(ctx, payment.BookingID); err == nil {
		bookingStatus = booking.Status
	} else {
		s.logger.Warn("get booking details for sync result failed", zap.String("bookingID", payment.BookingID), zap.Error(err))
	}

	return &serviceInterfaces.PaymentSyncResult{
		PaymentID:      payment.PaymentID,
		BookingID:      payment.BookingID,
		Status:         string(payment.Status),
		ProviderStatus: payment.ProviderStatus,
		IsFinal:        payment.Status != domain.PaymentStatusPending,
		PaymentURL:     pendingPaymentURL(payment),
		ExpiresAt:      formatOptionalTime(&payment.ExpiresAt),
		BookingStatus:  bookingStatus,
		SeatsReleased:  payment.Status == domain.PaymentStatusFailed || bookingStatus == bookingStatusPaymentFailed,
		Amount:         payment.Amount,
	}, nil
}

// getBooking récupère la réservation en traduisant l'absence en ErrorBookingNotFound.
func (s *paymentServiceImpl) getBooking(ctx context.Context, bookingID string) (*client.BookingDetails, error) {
	booking, err := s.bookingClient.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil, paymentErrors.ErrorBookingNotFound
		}
		s.logger.Error("get booking details failed", zap.String("bookingID", bookingID), zap.Error(err))
		return nil, paymentErrors.ErrorInternalServer
	}
	return booking, nil
}

// checkBookingOwnership vérifie que l'utilisateur authentifié (Firebase UID) est le passager de la réservation.
// booking.PassengerID peut contenir le UserID interne ou le Firebase UID.
func (s *paymentServiceImpl) checkBookingOwnership(ctx context.Context, booking *client.BookingDetails) error {
	firebaseUID, _ := ctx.Value(middleware.FirebaseIDKey).(string)
	if firebaseUID == "" {
		return paymentErrors.ErrorUnauthorized
	}
	if booking.PassengerID == firebaseUID {
		return nil
	}

	userID, err := s.userClient.GetUserIDByFirebaseID(ctx, firebaseUID)
	if err != nil {
		if errors.Is(err, paymentErrors.ErrorUserNotFound) {
			return paymentErrors.ErrorPaymentNotOwned
		}
		s.logger.Error("resolve user from firebase uid failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	if userID == "" || userID != booking.PassengerID {
		s.logger.Warn("payment access denied: user is not the booking passenger",
			zap.String("bookingID", booking.BookingID),
		)
		return paymentErrors.ErrorPaymentNotOwned
	}
	return nil
}

// paymentExpiresAt calcule la fin de la fenêtre de paiement : les places étant réservées dès la
// création de la réservation, le délai court depuis booking.CreatedAt.
func (s *paymentServiceImpl) paymentExpiresAt(booking *client.BookingDetails) time.Time {
	timeout := time.Duration(s.cfg.Expiration.PaymentTimeoutMinutes) * time.Minute
	start := booking.CreatedAt
	if start.IsZero() {
		start = time.Now().UTC()
	}
	return start.UTC().Add(timeout)
}

// fedapayCustomer construit le client FedaPay (nom/prénom uniquement : le numéro de paiement
// est choisi par l'utilisateur sur la page FedaPay). Nil si user-service est indisponible.
func (s *paymentServiceImpl) fedapayCustomer(ctx context.Context, passengerID string) *fedapay.CustomerPayload {
	userInfo, err := s.userClient.GetUserByUserID(ctx, passengerID)
	if err != nil || userInfo == nil {
		s.logger.Warn("get user info failed, creating transaction without customer", zap.Error(err))
		return nil
	}
	return &fedapay.CustomerPayload{
		FirstName: userInfo.FirstName,
		LastName:  userInfo.Name,
	}
}

// cancelOrphanTransaction supprime (best effort) une transaction FedaPay qui n'a pas pu être persistée.
func (s *paymentServiceImpl) cancelOrphanTransaction(transactionID int) {
	if err := s.fedapayClient.CancelTransaction(transactionID); err != nil {
		s.logger.Error("cancel orphan FedaPay transaction failed", zap.Int("transactionID", transactionID), zap.Error(err))
	}
}

// invalidatePaymentCache invalide le cache du paiement d'une réservation.
func (s *paymentServiceImpl) invalidatePaymentCache(ctx context.Context, bookingID string) {
	if s.cache != nil {
		s.cache.InvalidatePaymentByBookingID(ctx, bookingID)
	}
}

// publishPaymentEvent notifie le passager (non bloquant).
func (s *paymentServiceImpl) publishPaymentEvent(ctx context.Context, payment *domain.Payment, eventType string, payload map[string]string) {
	if s.notifRedis == nil {
		return
	}
	booking, err := s.bookingClient.GetBookingDetails(ctx, payment.BookingID)
	if err != nil {
		return
	}
	if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
		EventType:     eventType,
		UserID:        booking.PassengerID,
		ReferenceID:   payment.PaymentID,
		ReferenceType: notification.RefPayment,
		Payload:       payload,
	}); pubErr != nil {
		s.logger.Error("failed to publish payment notification", zap.String("eventType", eventType), zap.Error(pubErr))
	}
}

// pendingPaymentURL n'expose le lien de paiement que tant que le paiement est pending.
func pendingPaymentURL(payment *domain.Payment) string {
	if payment.Status != domain.PaymentStatusPending {
		return ""
	}
	return payment.PaymentURL
}

// normalizeTogoPhoneNumber normalise un numéro togolais sur 8 chiffres (préfixes +228 / 00228 / 228 retirés).
func normalizeTogoPhoneNumber(raw string) (string, bool) {
	var digits strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case unicode.IsDigit(r):
			digits.WriteRune(r)
		case r == '+' || r == ' ' || r == '-' || r == '.':
			// séparateurs tolérés
		default:
			return "", false
		}
	}
	number := digits.String()
	number = strings.TrimPrefix(number, "00228")
	if len(number) == 11 {
		number = strings.TrimPrefix(number, "228")
	}
	if len(number) != 8 {
		return "", false
	}
	return number, true
}
