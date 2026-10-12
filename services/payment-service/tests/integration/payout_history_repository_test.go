package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayoutHistoryRepository(t *testing.T) {
	ctx := context.Background()
	historyRepo := newTestPayoutHistoryRepository()
	payoutWriteRepo := newTestPayoutWriteRepository()

	t.Run("entry système - champs support NULL en DB", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(fixtures.WithPayoutID("po-hist-sys-1"))
		require.NoError(t, payoutWriteRepo.CreatePayout(ctx, payout))

		entry := &domain.PayoutStatusHistory{
			HistoryID:   uuid.New().String(),
			PayoutID:    "po-hist-sys-1",
			Status:      "scheduled",
			InitiatedBy: "system",
			OccurredAt:  time.Now().UTC(),
			Notes:       "",
		}

		err := historyRepo.CreateHistoryEntry(ctx, entry)
		require.NoError(t, err)

		var status, initiatedBy string
		var supportUserID, supportFirstName, supportLastName, notes *string
		err = testPool.QueryRow(ctx,
			`SELECT status, initiated_by, support_user_id, support_first_name, support_last_name, notes
			 FROM payout_status_history WHERE payout_id = $1`,
			"po-hist-sys-1",
		).Scan(&status, &initiatedBy, &supportUserID, &supportFirstName, &supportLastName, &notes)
		require.NoError(t, err)
		assert.Equal(t, "scheduled", status)
		assert.Equal(t, "system", initiatedBy)
		assert.Nil(t, supportUserID)
		assert.Nil(t, supportFirstName)
		assert.Nil(t, supportLastName)
		assert.Nil(t, notes)
	})

	t.Run("entry support - champs support remplis", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(fixtures.WithPayoutID("po-hist-sup-1"))
		require.NoError(t, payoutWriteRepo.CreatePayout(ctx, payout))

		entry := &domain.PayoutStatusHistory{
			HistoryID:        uuid.New().String(),
			PayoutID:         "po-hist-sup-1",
			Status:           "launched_by_support",
			InitiatedBy:      "support",
			SupportUserID:    "support-42",
			SupportFirstName: "Awa",
			SupportLastName:  "Diallo",
			OccurredAt:       time.Now().UTC(),
			Notes:            "Déclenché manuellement par Awa Diallo",
		}

		err := historyRepo.CreateHistoryEntry(ctx, entry)
		require.NoError(t, err)

		var initiatedBy string
		var supportUserID, supportFirstName, supportLastName, notes *string
		err = testPool.QueryRow(ctx,
			`SELECT initiated_by, support_user_id, support_first_name, support_last_name, notes
			 FROM payout_status_history WHERE payout_id = $1`,
			"po-hist-sup-1",
		).Scan(&initiatedBy, &supportUserID, &supportFirstName, &supportLastName, &notes)
		require.NoError(t, err)
		assert.Equal(t, "support", initiatedBy)
		require.NotNil(t, supportUserID)
		assert.Equal(t, "support-42", *supportUserID)
		require.NotNil(t, supportFirstName)
		assert.Equal(t, "Awa", *supportFirstName)
		require.NotNil(t, supportLastName)
		assert.Equal(t, "Diallo", *supportLastName)
		require.NotNil(t, notes)
		assert.Contains(t, *notes, "Awa Diallo")
	})

	t.Run("FK violation si payout inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		entry := &domain.PayoutStatusHistory{
			HistoryID:   uuid.New().String(),
			PayoutID:    "po-nonexistent",
			Status:      "scheduled",
			InitiatedBy: "system",
			OccurredAt:  time.Now().UTC(),
		}

		err := historyRepo.CreateHistoryEntry(ctx, entry)
		assert.Error(t, err)
	})

	t.Run("plusieurs entries pour un même payout - ordre chronologique", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(fixtures.WithPayoutID("po-hist-multi-1"))
		require.NoError(t, payoutWriteRepo.CreatePayout(ctx, payout))

		base := time.Now().UTC()
		for i, st := range []string{"scheduled", "processing", "completed"} {
			entry := &domain.PayoutStatusHistory{
				HistoryID:   uuid.New().String(),
				PayoutID:    "po-hist-multi-1",
				Status:      st,
				InitiatedBy: "system",
				OccurredAt:  base.Add(time.Duration(i) * time.Second),
			}
			require.NoError(t, historyRepo.CreateHistoryEntry(ctx, entry))
		}

		rows, err := testPool.Query(ctx,
			`SELECT status FROM payout_status_history WHERE payout_id = $1 ORDER BY occurred_at ASC`,
			"po-hist-multi-1",
		)
		require.NoError(t, err)
		defer rows.Close()

		var statuses []string
		for rows.Next() {
			var s string
			require.NoError(t, rows.Scan(&s))
			statuses = append(statuses, s)
		}
		assert.Equal(t, []string{"scheduled", "processing", "completed"}, statuses)

		entries, err := historyRepo.ListByPayoutID(ctx, "po-hist-multi-1")
		require.NoError(t, err)
		require.Len(t, entries, 3)
		assert.Equal(t, "scheduled", entries[0].Status)
		assert.Equal(t, "completed", entries[2].Status)
		assert.Empty(t, entries[0].SupportUserID)
	})

	t.Run("ListByPayoutID - champs support relus", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(fixtures.WithPayoutID("po-hist-read-1"))
		require.NoError(t, payoutWriteRepo.CreatePayout(ctx, payout))
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, &domain.PayoutStatusHistory{
			HistoryID: uuid.New().String(), PayoutID: "po-hist-read-1", Status: "launched_by_support",
			InitiatedBy: "support", SupportUserID: "support-42", SupportFirstName: "Awa", SupportLastName: "Diallo",
			OccurredAt: time.Now().UTC(), Notes: "manuel",
		}))

		entries, err := historyRepo.ListByPayoutID(ctx, "po-hist-read-1")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "support-42", entries[0].SupportUserID)
		assert.Equal(t, "Awa", entries[0].SupportFirstName)
		assert.Equal(t, "Diallo", entries[0].SupportLastName)
		assert.Equal(t, "manuel", entries[0].Notes)

		empty, err := historyRepo.ListByPayoutID(ctx, "po-unknown")
		require.NoError(t, err)
		assert.Empty(t, empty)
	})
}
