package e2e

import (
	"context"
	"strings"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

// extractResetToken isole le token du lien `…/reset-password?token=XXXX` dans le dernier email.
func extractResetToken(t *testing.T, body string) string {
	t.Helper()
	const marker = "token="
	idx := strings.Index(body, marker)
	require.GreaterOrEqual(t, idx, 0, "lien de reset absent du corps: %q", body)
	tok := body[idx+len(marker):]
	// Coupe à la première frontière (guillemet, espace, retour ligne, fin de balise).
	tok = strings.FieldsFunc(tok, func(r rune) bool {
		return r == '"' || r == ' ' || r == '\n' || r == '<' || r == '>'
	})[0]
	require.NotEmpty(t, tok)
	return tok
}

// Flux complet support : forgot → notif admin → list → trigger → reset.
func TestE2E_PasswordReset_SupportFlow(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	admin := fixtures.NewTestAdmin(fixtures.WithEmail("admin@tissimah.local"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, admin))
	adminCtx := withSupport(ctx, admin.UserID, domain.RoleAdmin)

	agent := fixtures.NewTestSupportUser(fixtures.WithEmail("agent@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, agent))

	// 1. Demande publique de l'agent → email de notification aux admins.
	_, err := d.client.ForgotPassword(ctx, &supportpb.ForgotPasswordRequest{Email: "agent@x.com"})
	require.NoError(t, err)
	waitEmails(t, d.email, 1)
	assert.Equal(t, "admin@tissimah.local", d.email.Calls()[0].To)

	// 2. L'admin voit la demande au dashboard.
	listRes, err := d.client.ListPasswordResetRequests(adminCtx, &supportpb.ListPasswordResetRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, listRes.GetRequests(), 1)
	assert.Equal(t, agent.UserID, listRes.GetRequests()[0].GetUserId())

	// 3. L'admin lance la réinitialisation → email avec lien à l'agent.
	_, err = d.client.TriggerPasswordReset(adminCtx, &supportpb.TriggerPasswordResetRequest{UserId: agent.UserID})
	require.NoError(t, err)
	waitEmails(t, d.email, 2)
	resetEmail := d.email.Calls()[1]
	assert.Equal(t, "agent@x.com", resetEmail.To)
	token := extractResetToken(t, resetEmail.BodyText)

	// 3b. Un second admin (liste périmée) tente de relancer → demande déjà traitée.
	_, err = d.client.TriggerPasswordReset(adminCtx, &supportpb.TriggerPasswordResetRequest{UserId: agent.UserID})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, grpcCode(t, err))
	assert.Equal(t, 2, d.email.Count()) // pas de nouvel email

	// 4. L'agent applique son nouveau mot de passe via le token.
	_, err = d.client.ResetPassword(ctx, &supportpb.ResetPasswordRequest{
		Token: token, NewPassword: "BrandNewPass1!",
	})
	require.NoError(t, err)

	// 5. La demande a disparu de la liste.
	listRes, err = d.client.ListPasswordResetRequests(adminCtx, &supportpb.ListPasswordResetRequestsRequest{})
	require.NoError(t, err)
	assert.Empty(t, listRes.GetRequests())

	// 6. Token à usage unique : un second usage échoue.
	_, err = d.client.ResetPassword(ctx, &supportpb.ResetPasswordRequest{
		Token: token, NewPassword: "AnotherPass1!",
	})
	require.Error(t, err)
}

// Self-service admin : forgot → lien direct sur sa propre adresse → reset.
func TestE2E_PasswordReset_AdminSelfService(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	admin := fixtures.NewTestAdmin(fixtures.WithEmail("admin@tissimah.local"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, admin))

	_, err := d.client.ForgotPassword(ctx, &supportpb.ForgotPasswordRequest{Email: "admin@tissimah.local"})
	require.NoError(t, err)
	waitEmails(t, d.email, 1)
	call := d.email.Calls()[0]
	assert.Equal(t, "admin@tissimah.local", call.To)
	token := extractResetToken(t, call.BodyText)

	_, err = d.client.ResetPassword(ctx, &supportpb.ResetPasswordRequest{
		Token: token, NewPassword: "AdminBrandNew1!",
	})
	require.NoError(t, err)
}

// ForgotPassword sur un email inconnu : anti-énumération (succès, aucun email).
func TestE2E_ForgotPassword_UnknownEmail(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	_, err := d.client.ForgotPassword(ctx, &supportpb.ForgotPasswordRequest{Email: "ghost@x.com"})
	require.NoError(t, err)
	assert.Equal(t, 0, d.email.Count())
}
