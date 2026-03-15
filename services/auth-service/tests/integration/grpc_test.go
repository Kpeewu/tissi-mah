package integration

import (
	"context"
	"net"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/auth-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/auth-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/service"
	"github.com/Kpeewu/tissi-mah/services/auth-service/tests/mocks"
	authpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// =============================================================================
// Helpers E2E
// =============================================================================

// newTestGRPCServer crée un vrai serveur gRPC avec de vrais repos et un userClient mocké.
// Retourne le client gRPC connecté et une fonction de nettoyage.
func newTestGRPCServer(t *testing.T, userClient *mocks.MockUserClient) (authpb.AuthServiceClient, func()) {
	t.Helper()

	readRepo := implementations.NewAuthReadRepository(testPool, zap.NewNop())
	writeRepo := implementations.NewAuthWriteRepository(testPool, zap.NewNop())
	authService := service.NewAuthService(readRepo, writeRepo, userClient, zap.NewNop())
	handler := grpcHandler.NewAuthHandler(authService, zap.NewNop())

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer(grpc.UnaryInterceptor(middleware.AuthInterceptor()))
	authpb.RegisterAuthServiceServer(srv, handler)

	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return authpb.NewAuthServiceClient(conn), func() {
		conn.Close()
		srv.GracefulStop()
	}
}

// withFirebaseUID injecte le Firebase UID dans les métadonnées gRPC sortantes.
func withFirebaseUID(ctx context.Context, uid string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-firebase-uid", uid)
}

// assertGRPCCode vérifie que l'erreur gRPC correspond au code attendu.
func assertGRPCCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error")
	assert.Equal(t, code, st.Code())
}

// stubbedUserPreview retourne un UserPreview de test minimal.
func stubbedUserPreview(authID string) *domain.UserPreview {
	return &domain.UserPreview{
		AuthID:    authID,
		UserID:    uuid.New().String(),
		Name:      "Doe",
		FirstName: "John",
	}
}

// =============================================================================
// Health
// =============================================================================

func TestE2E_Health(t *testing.T) {
	client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
	defer cleanup()

	resp, err := client.Health(context.Background(), &authpb.HealthRequest{})

	require.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.NotZero(t, resp.Timestamp)
}

// =============================================================================
// CheckEmail
// =============================================================================

func TestE2E_CheckEmail(t *testing.T) {
	ctx := context.Background()
	client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
	defer cleanup()

	t.Run("should return available when email does not exist", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		resp, err := client.CheckEmail(ctx, &authpb.CheckEmailRequest{Email: "free@example.com"})

		require.NoError(t, err)
		assert.True(t, resp.IsAvailable)
	})

	t.Run("should return not available when email is already taken", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		auth := fixtures.NewTestAuth(fixtures.WithEmail("taken@example.com"), fixtures.WithNoPhoneNumber())
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		resp, err := client.CheckEmail(ctx, &authpb.CheckEmailRequest{Email: "taken@example.com"})

		require.NoError(t, err)
		assert.False(t, resp.IsAvailable)
	})
}

// =============================================================================
// CheckPhoneNumber
// =============================================================================

func TestE2E_CheckPhoneNumber(t *testing.T) {
	ctx := context.Background()
	client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
	defer cleanup()

	t.Run("should return available when phone does not exist", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		resp, err := client.CheckPhoneNumber(ctx, &authpb.CheckPhoneNumberRequest{PhoneNumber: "+22800000000"})

		require.NoError(t, err)
		assert.True(t, resp.IsAvailable)
	})

	t.Run("should return not available when phone is already taken", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		auth := fixtures.NewTestAuth(fixtures.WithPhoneNumber("+22811111111"), fixtures.WithNoEmail())
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		resp, err := client.CheckPhoneNumber(ctx, &authpb.CheckPhoneNumberRequest{PhoneNumber: "+22811111111"})

		require.NoError(t, err)
		assert.False(t, resp.IsAvailable)
	})
}

// =============================================================================
// CreateAccount
// =============================================================================

func TestE2E_CreateAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("should create account successfully", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		mockUserClient := &mocks.MockUserClient{}
		client, cleanup := newTestGRPCServer(t, mockUserClient)
		defer cleanup()

		firebaseUID := uuid.New().String()

		mockUserClient.On("CreateUser",
			mock.Anything,
			mock.AnythingOfType("string"), // authID généré en interne
			firebaseUID,
			"Doe", "John", "",
		).Return(stubbedUserPreview(uuid.New().String()), nil).Once()

		resp, err := client.CreateAccount(
			withFirebaseUID(ctx, firebaseUID),
			&authpb.CreateAccountRequest{
				Name: "Doe", FirstName: "John", Email: "new@example.com",
			},
		)

		require.NoError(t, err)
		assert.NotNil(t, resp.User)
		assert.NotEmpty(t, resp.User.AuthID)
		assert.Equal(t, "Doe", resp.User.Name)
		assert.Equal(t, "new@example.com", resp.User.Email)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("should fail without firebase UID in metadata", func(t *testing.T) {
		client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
		defer cleanup()

		_, err := client.CreateAccount(ctx, &authpb.CreateAccountRequest{
			Name: "Doe", FirstName: "John",
		})

		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("should fail on duplicate email", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		mockUserClient := &mocks.MockUserClient{}
		client, cleanup := newTestGRPCServer(t, mockUserClient)
		defer cleanup()

		dupEmail := "dup@example.com"
		existing := fixtures.NewTestAuth(fixtures.WithEmail(dupEmail), fixtures.WithNoPhoneNumber())
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, existing))

		_, err := client.CreateAccount(
			withFirebaseUID(ctx, uuid.New().String()),
			&authpb.CreateAccountRequest{Name: "Doe", FirstName: "John", Email: dupEmail},
		)

		assertGRPCCode(t, err, codes.AlreadyExists)
	})

	t.Run("should fail on duplicate phone number", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		mockUserClient := &mocks.MockUserClient{}
		client, cleanup := newTestGRPCServer(t, mockUserClient)
		defer cleanup()

		dupPhone := "+22822222222"
		existing := fixtures.NewTestAuth(fixtures.WithPhoneNumber(dupPhone), fixtures.WithNoEmail())
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, existing))

		_, err := client.CreateAccount(
			withFirebaseUID(ctx, uuid.New().String()),
			&authpb.CreateAccountRequest{Name: "Doe", FirstName: "John", PhoneNumber: dupPhone},
		)

		assertGRPCCode(t, err, codes.AlreadyExists)
	})
}

// =============================================================================
// Login
// =============================================================================

func TestE2E_Login(t *testing.T) {
	ctx := context.Background()

	t.Run("should return exists=false when user has no account", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
		defer cleanup()

		resp, err := client.Login(
			withFirebaseUID(ctx, uuid.New().String()),
			&authpb.LoginRequest{},
		)

		require.NoError(t, err)
		assert.False(t, resp.Exists)
		assert.Nil(t, resp.User)
	})

	t.Run("should return exists=true with user preview when account exists", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		mockUserClient := &mocks.MockUserClient{}
		client, cleanup := newTestGRPCServer(t, mockUserClient)
		defer cleanup()

		firebaseUID := uuid.New().String()
		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID(firebaseUID))
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		mockUserClient.On("GetUserByAuthID", mock.Anything, auth.AuthID).
			Return(stubbedUserPreview(auth.AuthID), nil).Once()

		resp, err := client.Login(
			withFirebaseUID(ctx, firebaseUID),
			&authpb.LoginRequest{},
		)

		require.NoError(t, err)
		assert.True(t, resp.Exists)
		require.NotNil(t, resp.User)
		assert.Equal(t, auth.AuthID, resp.User.AuthID)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("should fail without firebase UID in metadata", func(t *testing.T) {
		client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
		defer cleanup()

		_, err := client.Login(ctx, &authpb.LoginRequest{})

		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("should return Internal error for suspended account", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
		defer cleanup()

		firebaseUID := uuid.New().String()
		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID(firebaseUID))
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		// Suspendre le compte directement en base
		_, err := testPool.Exec(ctx,
			"UPDATE auth SET is_suspended = true, suspension_end_date = NOW() + INTERVAL '1 day' WHERE auth_id = $1",
			auth.AuthID,
		)
		require.NoError(t, err)

		_, err = client.Login(
			withFirebaseUID(ctx, firebaseUID),
			&authpb.LoginRequest{},
		)

		// CanLogin() == false → service retourne ErrorInternalServer → codes.Internal
		assertGRPCCode(t, err, codes.Internal)
	})
}

// =============================================================================
// DeleteAccount
// =============================================================================

func TestE2E_DeleteAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("should soft-delete account successfully", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
		defer cleanup()

		firebaseUID := uuid.New().String()
		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID(firebaseUID))
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		resp, err := client.DeleteAccount(
			withFirebaseUID(ctx, firebaseUID),
			&authpb.DeleteAccountRequest{},
		)

		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("should fail when account does not exist", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
		defer cleanup()

		_, err := client.DeleteAccount(
			withFirebaseUID(ctx, uuid.New().String()),
			&authpb.DeleteAccountRequest{},
		)

		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("should fail without firebase UID in metadata", func(t *testing.T) {
		client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
		defer cleanup()

		_, err := client.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})

		assertGRPCCode(t, err, codes.Unauthenticated)
	})
}

// =============================================================================
// GetAuthInfo
// =============================================================================

func TestE2E_GetAuthInfo(t *testing.T) {
	ctx := context.Background()
	client, cleanup := newTestGRPCServer(t, &mocks.MockUserClient{})
	defer cleanup()

	t.Run("should return auth info for existing auth_id", func(t *testing.T) {
		cleanupAuthTable(t, ctx)
		auth := fixtures.NewTestAuth()
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		resp, err := client.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{AuthID: auth.AuthID})

		require.NoError(t, err)
		assert.Equal(t, auth.AuthID, resp.AuthID)
		assert.True(t, resp.IsActive)
		assert.False(t, resp.IsSuspended)
		assert.Equal(t, *auth.Email, resp.Email)
		assert.Equal(t, *auth.PhoneNumber, resp.PhoneNumber)
	})

	t.Run("should return NotFound when auth_id does not exist", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		_, err := client.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{AuthID: uuid.New().String()})

		assertGRPCCode(t, err, codes.NotFound)
	})
}
