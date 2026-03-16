package gateway

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcMetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	authpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/authpb"
	filepb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/filepb"
	kycpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/kycpb"
	ratingpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/ratingpb"
	trippb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/trippb"
	userpb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/userpb"
	vehiclepb "github.com/Kpeewu/tissi-mah/services/api-gateway/proto/gen/vehiclepb"
)

// MuxConfig contient les paramètres pour créer le grpc-gateway ServeMux
type MuxConfig struct {
	AuthServiceAddr    string
	UserServiceAddr    string
	RatingServiceAddr  string
	FileServiceAddr    string
	VehicleServiceAddr string
	TripsServiceAddr   string
	KYCServiceAddr     string
	Logger             *zap.Logger
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

	// Custom error handler : remplace {"code":N,"message":"...","details":[]}
	// par {"ErrorMessage":"..."} avec le bon code HTTP
	errorHandler := runtime.WithErrorHandler(func(
		_ context.Context,
		_ *runtime.ServeMux,
		_ runtime.Marshaler,
		w http.ResponseWriter,
		_ *http.Request,
		err error,
	) {
		s, _ := status.FromError(err)
		httpStatus := runtime.HTTPStatusFromCode(s.Code())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"ErrorMessage": s.Message(),
		})
	})

	mux := runtime.NewServeMux(jsonOpts, metadataAnnotator, errorHandler)

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

	// Enregistrer rating-service
	if err := ratingpb.RegisterRatingServiceHandlerFromEndpoint(ctx, mux, cfg.RatingServiceAddr, dialOpts); err != nil {
		return nil, err
	}
	cfg.Logger.Info("registered rating-service handler", zap.String("endpoint", cfg.RatingServiceAddr))

	// Enregistrer file-service
	if err := filepb.RegisterFileServiceHandlerFromEndpoint(ctx, mux, cfg.FileServiceAddr, dialOpts); err != nil {
		return nil, err
	}
	cfg.Logger.Info("registered file-service handler", zap.String("endpoint", cfg.FileServiceAddr))

	// Enregistrer vehicle-service
	if err := vehiclepb.RegisterVehicleServiceHandlerFromEndpoint(ctx, mux, cfg.VehicleServiceAddr, dialOpts); err != nil {
		return nil, err
	}
	cfg.Logger.Info("registered vehicle-service handler", zap.String("endpoint", cfg.VehicleServiceAddr))

	// Enregistrer trips-service
	if err := trippb.RegisterTripServiceHandlerFromEndpoint(ctx, mux, cfg.TripsServiceAddr, dialOpts); err != nil {
		return nil, err
	}
	cfg.Logger.Info("registered trips-service handler", zap.String("endpoint", cfg.TripsServiceAddr))

	// Enregistrer kyc-service
	if err := kycpb.RegisterKYCServiceHandlerFromEndpoint(ctx, mux, cfg.KYCServiceAddr, dialOpts); err != nil {
		return nil, err
	}
	cfg.Logger.Info("registered kyc-service handler", zap.String("endpoint", cfg.KYCServiceAddr))

	return mux, nil
}
