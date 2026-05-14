package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockPayoutRepositoryRead struct {
	mock.Mock
}

func (m *MockPayoutRepositoryRead) GetByID(ctx context.Context, payoutID string) (*domain.Payout, error) {
	args := m.Called(ctx, payoutID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payout), args.Error(1)
}

func (m *MockPayoutRepositoryRead) GetByTripID(ctx context.Context, tripID string) (*domain.Payout, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payout), args.Error(1)
}

func (m *MockPayoutRepositoryRead) GetDriverPayouts(ctx context.Context, driverID string, pageIndex int) ([]*domain.Payout, error) {
	args := m.Called(ctx, driverID, pageIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Payout), args.Error(1)
}

func (m *MockPayoutRepositoryRead) GetBatchByID(ctx context.Context, batchID string) (*domain.PayoutBatch, error) {
	args := m.Called(ctx, batchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PayoutBatch), args.Error(1)
}

func (m *MockPayoutRepositoryRead) GetTripsReadyForPayout(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockPayoutRepositoryRead) GetReleasedPaymentsForTrip(ctx context.Context, tripID string) ([]*domain.Payment, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Payment), args.Error(1)
}

func (m *MockPayoutRepositoryRead) IsTripReadyForPayout(ctx context.Context, tripID string) (bool, error) {
	args := m.Called(ctx, tripID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPayoutRepositoryRead) GetByProviderReference(ctx context.Context, providerReference string) (*domain.Payout, error) {
	args := m.Called(ctx, providerReference)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payout), args.Error(1)
}

type MockPayoutRepositoryWrite struct {
	mock.Mock
}

func (m *MockPayoutRepositoryWrite) CreatePayout(ctx context.Context, payout *domain.Payout) error {
	args := m.Called(ctx, payout)
	return args.Error(0)
}

func (m *MockPayoutRepositoryWrite) UpdatePayoutStatus(ctx context.Context, payoutID string, status domain.PayoutStatus, providerReference string) error {
	args := m.Called(ctx, payoutID, status, providerReference)
	return args.Error(0)
}

func (m *MockPayoutRepositoryWrite) MarkPayoutFailed(ctx context.Context, payoutID string, reason string) error {
	args := m.Called(ctx, payoutID, reason)
	return args.Error(0)
}

func (m *MockPayoutRepositoryWrite) IncrementRetryCount(ctx context.Context, payoutID string) error {
	args := m.Called(ctx, payoutID)
	return args.Error(0)
}

func (m *MockPayoutRepositoryWrite) CreateBatch(ctx context.Context, batch *domain.PayoutBatch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockPayoutRepositoryWrite) UpdateBatch(ctx context.Context, batch *domain.PayoutBatch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func (m *MockPayoutRepositoryWrite) MarkPaymentsAsPaidOut(ctx context.Context, tripID string) error {
	args := m.Called(ctx, tripID)
	return args.Error(0)
}

func (m *MockPayoutRepositoryRead) HasActivePayout(ctx context.Context, driverID string) (bool, error) {
	args := m.Called(ctx, driverID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPayoutRepositoryWrite) AnonymizeDriverRefs(ctx context.Context, driverID string) error {
	args := m.Called(ctx, driverID)
	return args.Error(0)
}
