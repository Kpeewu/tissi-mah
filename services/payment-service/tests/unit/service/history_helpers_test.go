package service_test

import (
	"github.com/stretchr/testify/mock"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
)

// newPaymentHistoryMock accepte toute écriture d'historique : les tests dédiés inspectent les appels
// enregistrés (paymentHistory) plutôt que de déclarer chaque entrée attendue.
func newPaymentHistoryMock() *mocks.MockPaymentHistoryRepository {
	m := new(mocks.MockPaymentHistoryRepository)
	m.On("CreateHistoryEntry", mock.Anything, mock.Anything).Return(nil).Maybe()
	return m
}

// newRefundHistoryMock : cf. newPaymentHistoryMock.
func newRefundHistoryMock() *mocks.MockRefundHistoryRepository {
	m := new(mocks.MockRefundHistoryRepository)
	m.On("CreateHistoryEntry", mock.Anything, mock.Anything).Return(nil).Maybe()
	return m
}

// paymentHistory retourne les entrées d'historique de paiement écrites, dans l'ordre.
func (d *testDeps) paymentHistory() []*domain.PaymentStatusHistory {
	var entries []*domain.PaymentStatusHistory
	for _, call := range d.paymentHistoryRepo.Calls {
		if call.Method == "CreateHistoryEntry" {
			entries = append(entries, call.Arguments.Get(1).(*domain.PaymentStatusHistory))
		}
	}
	return entries
}

// paymentHistoryStatuses retourne les statuts/événements historisés pour un paiement, dans l'ordre.
func (d *testDeps) paymentHistoryStatuses(paymentID string) []string {
	var statuses []string
	for _, e := range d.paymentHistory() {
		if e.PaymentID == paymentID {
			statuses = append(statuses, e.Status)
		}
	}
	return statuses
}

// refundHistory retourne les entrées d'historique de remboursement écrites, dans l'ordre.
func (d *testDeps) refundHistory() []*domain.RefundStatusHistory {
	var entries []*domain.RefundStatusHistory
	for _, call := range d.refundHistoryRepo.Calls {
		if call.Method == "CreateHistoryEntry" {
			entries = append(entries, call.Arguments.Get(1).(*domain.RefundStatusHistory))
		}
	}
	return entries
}

// refundHistoryStatuses retourne les statuts historisés (tous remboursements confondus), dans l'ordre.
func (d *testDeps) refundHistoryStatuses() []string {
	var statuses []string
	for _, e := range d.refundHistory() {
		statuses = append(statuses, e.Status)
	}
	return statuses
}
