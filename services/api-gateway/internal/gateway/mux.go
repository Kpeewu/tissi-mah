package gateway

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcMetadata "google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"

	authpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/authpb"
	userpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/userpb"
)

// MuxConfig contient les paramètres pour créer le grpc-gateway ServeMux
type MuxConfig struct {
	AuthServiceAddr string
	UserServiceAddr string
	Logger          *zap.Logger
}

// NewGatewayMux crée un runtime.ServeMux configuré avec les handlers
// grpc-gateway pour auth-service et user-service.
// L'annotator transmet le header x-firebase-uid en metadata gRPC.
func NewGatewayMux(ctx context.Context, cfg MuxConfig) (http.Handler, error) {
	// Options de sérialisation JSON (compatibilité avec le comportement Kong)
	jsonOpts := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	// Metadata annotator : transmet le header x-firebase-uid
	// depuis la requête HTTP vers les metadata gRPC
	metadataAnnotator := runtime.WithMetadata(func(_ context.Context, r *http.Request) grpcMetadata.MD {
		md := grpcMetadata.MD{}
		if uid := r.Header.Get("x-firebase-uid"); uid != "" {
			md.Set("x-firebase-uid", uid)
		}
		return md
	})

	mux := runtime.NewServeMux(jsonOpts, metadataAnnotator)

	// Options de connexion gRPC vers les services internes (pas de TLS intra-cluster)
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Enregistrer auth-service
	if err := authpb.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, cfg.AuthServiceAddr, dialOpts); err != nil {
		return nil, err
	}
	cfg.Logger.Info("registered auth-service handler", zap.String("endpoint", cfg.AuthServiceAddr))

	// Enregistrer user-service
	if err := userpb.RegisterUserServiceHandlerFromEndpoint(ctx, mux, cfg.UserServiceAddr, dialOpts); err != nil {
		return nil, err
	}
	cfg.Logger.Info("registered user-service handler", zap.String("endpoint", cfg.UserServiceAddr))

	return mux, nil
}
