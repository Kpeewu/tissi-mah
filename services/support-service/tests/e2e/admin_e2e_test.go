package e2e

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestE2E_CreateSupportAgent_AdminFlow(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	admin := fixtures.NewTestAdmin(fixtures.WithEmail("admin@tissimah.local"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, admin))
	adminCtx := withSupport(ctx, admin.UserID, domain.RoleAdmin)

	// 1. Admin crée un agent
	createRes, err := d.client.CreateSupportAgent(adminCtx, &supportpb.CreateSupportAgentRequest{
		Email:     "new@x.com",
		FirstName: "Jean",
		LastName:  "Dupont",
	})
	require.NoError(t, err)
	require.NotEmpty(t, createRes.GetUserId())

	// Email provisoire envoyé
	waitEmails(t, d.email, 1)
	assert.Equal(t, "new@x.com", d.email.LastTo())

	// 2. List retourne admin + nouveau
	listRes, err := d.client.ListSupportAgents(adminCtx, &supportpb.ListSupportAgentsRequest{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int32(2), listRes.GetTotal())

	// 3. Deactivate le nouvel agent
	_, err = d.client.DeactivateSupportAgent(adminCtx, &supportpb.DeactivateSupportAgentRequest{
		UserId: createRes.GetUserId(),
	})
	require.NoError(t, err)

	// 4. Rôle support appelle admin endpoint → PermissionDenied
	supportCtx := withSupport(ctx, createRes.GetUserId(), domain.RoleSupport)
	_, err = d.client.ListSupportAgents(supportCtx, &supportpb.ListSupportAgentsRequest{Limit: 10})
	assert.Equal(t, codes.PermissionDenied, grpcCode(t, err))
}

func TestE2E_CreateSupportAgent_EmailTaken(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	admin := fixtures.NewTestAdmin()
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, admin))
	existing := fixtures.NewTestSupportUser(fixtures.WithEmail("dup@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, existing))

	adminCtx := withSupport(ctx, admin.UserID, domain.RoleAdmin)
	_, err := d.client.CreateSupportAgent(adminCtx, &supportpb.CreateSupportAgentRequest{
		Email: "dup@x.com", FirstName: "A", LastName: "B",
	})
	assert.Equal(t, codes.AlreadyExists, grpcCode(t, err))
}

func TestE2E_Deactivate_NotFound(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	admin := fixtures.NewTestAdmin()
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, admin))
	adminCtx := withSupport(ctx, admin.UserID, domain.RoleAdmin)

	_, err := d.client.DeactivateSupportAgent(adminCtx, &supportpb.DeactivateSupportAgentRequest{
		UserId: "00000000-0000-0000-0000-000000000abc",
	})
	assert.Equal(t, codes.NotFound, grpcCode(t, err))
}

func TestE2E_Admin_MustChangePassword_Blocked(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	admin := fixtures.NewTestAdmin(fixtures.WithMustChangePassword(true))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, admin))
	adminCtx := withSupport(ctx, admin.UserID, domain.RoleAdmin)

	// Toutes les routes admin bloquées (gateMustChange → ErrMustChangePassword → Internal)
	_, err := d.client.CreateSupportAgent(adminCtx, &supportpb.CreateSupportAgentRequest{
		Email: "n@x.com", FirstName: "A", LastName: "B",
	})
	require.Error(t, err)
}
