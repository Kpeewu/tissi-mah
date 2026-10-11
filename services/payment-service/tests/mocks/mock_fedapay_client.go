package mocks

import (
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	"github.com/stretchr/testify/mock"
)

type MockFedaPayClient struct {
	mock.Mock
}

func (m *MockFedaPayClient) CreateTransaction(params fedapay.CreateTransactionParams) (*fedapay.Transaction, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fedapay.Transaction), args.Error(1)
}

func (m *MockFedaPayClient) GetTransactionToken(transactionID int) (*fedapay.TokenResponse, error) {
	args := m.Called(transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fedapay.TokenResponse), args.Error(1)
}

func (m *MockFedaPayClient) GetTransaction(transactionID int) (*fedapay.Transaction, error) {
	args := m.Called(transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fedapay.Transaction), args.Error(1)
}

func (m *MockFedaPayClient) GetTransactionByMerchantReference(reference string) (*fedapay.Transaction, error) {
	args := m.Called(reference)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fedapay.Transaction), args.Error(1)
}

func (m *MockFedaPayClient) CancelTransaction(transactionID int) error {
	args := m.Called(transactionID)
	return args.Error(0)
}

func (m *MockFedaPayClient) CreatePayout(amount int, mode string, phoneNumber string, firstName string, lastName string) (*fedapay.PayoutItem, error) {
	args := m.Called(amount, mode, phoneNumber, firstName, lastName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fedapay.PayoutItem), args.Error(1)
}

func (m *MockFedaPayClient) StartPayouts(payoutIDs []int) ([]fedapay.PayoutItem, error) {
	args := m.Called(payoutIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]fedapay.PayoutItem), args.Error(1)
}

func (m *MockFedaPayClient) GetPayout(payoutID int) (*fedapay.PayoutItem, error) {
	args := m.Called(payoutID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fedapay.PayoutItem), args.Error(1)
}

func (m *MockFedaPayClient) VerifyWebhookSignature(signature string, rawPayload []byte) bool {
	args := m.Called(signature, rawPayload)
	return args.Bool(0)
}

var _ fedapay.FedaPayClient = (*MockFedaPayClient)(nil)
