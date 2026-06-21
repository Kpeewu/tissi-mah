package e2e

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	postgresHelper "github.com/Kpeewu/tissi-mah/pkg-test/postgres"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/config"
	grpcsrv "github.com/Kpeewu/tissi-mah/services/support-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/otp"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/service"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/token"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/support-service/tests/mocks"
	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var (
	testPool  *pgxpool.Pool
	testMini  *miniredis.Miniredis
	testRedis *redis.Client
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	tp, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations")
	if err != nil {
		log.Printf("SetupTestPostgres: %v", err)
	}
	if tp == nil {
		log.Fatal("no test postgres")
	}
	testPool = tp.Pool

	var exists bool
	_ = testPool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='support_users')").Scan(&exists)
	if !exists {
		if err := applyUpMigrations(ctx, testPool, "../../migrations"); err != nil {
			log.Fatalf("migrations: %v", err)
		}
	}

	testMini, err = miniredis.Run()
	if err != nil {
		log.Fatalf("miniredis: %v", err)
	}
	testRedis = redis.NewClient(&redis.Options{Addr: testMini.Addr()})

	code := m.Run()

	_ = testRedis.Close()
	testMini.Close()
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = tp.CleanUp(cleanupCtx)
	os.Exit(code)
}

func applyUpMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return err
		}
	}
	return nil
}

func cleanAll(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "DELETE FROM support_users")
	require.NoError(t, err)
	testMini.FlushAll()
}

// e2eDeps regroupe tout ce qu'un test a besoin : client gRPC, email spy, store OTP pour lire l'OTP envoyé.
type e2eDeps struct {
	client  supportpb.SupportServiceClient
	email   *mocks.SpyEmailSender
	cleanup func()
}

// newE2E démarre un vrai serveur gRPC sur un port éphémère avec tout le stack.
func newE2E(t *testing.T) *e2eDeps {
	t.Helper()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:          "test-secret-that-is-at-least-32-bytes-long-xxx",
			AccessTTLHours:  1,
			RefreshTTLHours: 24,
		},
		OTP: config.OTPConfig{
			TTLSeconds:        300,
			MaxAttempts:       3,
			ResendCooldownSec: 300,
		},
		RateLimit: config.RateLimitConfig{
			FailThreshold:     5,
			FailWindowSeconds: 86400,
		},
		PasswordReset: config.PasswordResetConfig{TTLSeconds: 3600},
		FrontendURL:   "https://support.test",
	}

	readRepo := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	writeRepo := implementations.NewSupportUserWriteRepository(testPool, zap.NewNop())

	otpStore := otp.NewStore(testRedis,
		time.Duration(cfg.OTP.TTLSeconds)*time.Second,
		time.Duration(cfg.OTP.ResendCooldownSec)*time.Second,
		time.Duration(cfg.RateLimit.FailWindowSeconds)*time.Second,
	)
	jwtSig := token.NewJWTSigner(cfg.JWT.Secret, cfg.JWT.AccessTTLHours)
	refresh := token.NewRefreshStore(testRedis, cfg.JWT.RefreshTTLHours)
	reset := token.NewResetStore(testRedis, time.Duration(cfg.PasswordReset.TTLSeconds)*time.Second)
	email := mocks.NewSpyEmailSender()

	svc := service.NewSupportService(cfg, readRepo, writeRepo, otpStore, jwtSig, refresh, reset, email, zap.NewNop())
	handler := grpcsrv.NewSupportHandler(svc, zap.NewNop())

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer(grpc.UnaryInterceptor(middleware.SupportInterceptor()))
	supportpb.RegisterSupportServiceServer(srv, handler)
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return &e2eDeps{
		client: supportpb.NewSupportServiceClient(conn),
		email:  email,
		cleanup: func() {
			_ = conn.Close()
			srv.GracefulStop()
		},
	}
}

// withSupport injecte x-support-uid (et role optionnel) dans la metadata sortante.
func withSupport(ctx context.Context, uid, role string) context.Context {
	md := metadata.Pairs("x-support-uid", uid)
	if role != "" {
		md.Append("x-support-role", role)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// waitEmails attend async.
func waitEmails(t *testing.T, spy *mocks.SpyEmailSender, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if spy.Count() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout: expected %d emails, got %d", n, spy.Count())
}

// extractOTP extrait le code à 6 chiffres du dernier email envoyé.
func extractOTP(t *testing.T, spy *mocks.SpyEmailSender) string {
	t.Helper()
	calls := spy.Calls()
	require.NotEmpty(t, calls)
	body := calls[len(calls)-1].BodyText
	// Le corps est "Votre code de connexion est : XXXXXX\n..." — on isole 6 digits.
	for i := 0; i+6 <= len(body); i++ {
		sub := body[i : i+6]
		allDigits := true
		for _, c := range sub {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return sub
		}
	}
	t.Fatalf("no 6-digit OTP in body: %q", body)
	return ""
}
