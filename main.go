package main

import (
	"auth-service/middleware"
	"auth-service/wire"
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	authpb "auth-service/proto/authpb"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found (continuing with system env)")
	}

	cfgFile := flag.String("config", "", "path to config file (optional)")
	flag.Parse()

	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.CallerKey = "caller"
	logger, err := cfg.Build(zap.AddCaller(), zap.AddCallerSkip(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	authHandler, cleanup, err := wire.InitializeContainer(*cfgFile, logger)
	if err != nil {
		logger.Fatal("failed to initialize", zap.Error(err))
	}
	defer cleanup()

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryRecovery(logger),
			middleware.UnaryLogger(logger),
		),
	)
	authpb.RegisterAuthServiceServer(grpcSrv, authHandler)
	reflection.Register(grpcSrv)

	grpcLis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatal("grpc listen failed", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := authpb.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, ":50051", opts); err != nil {
		logger.Fatal("gateway registration failed", zap.Error(err))
	}

	httpSrv := &http.Server{
		Addr:    ":8080",
		Handler: middleware.HTTPLogger(logger)(mux),
	}

	grpcReady := make(chan struct{})

	go func() {
		logger.Info("gRPC listening", zap.String("addr", ":50051"))
		close(grpcReady) // signal that gRPC is ready
		if err := grpcSrv.Serve(grpcLis); err != nil {
			logger.Fatal("grpc serve failed", zap.Error(err))
		}
	}()

	<-grpcReady

	go func() {
		logger.Info("HTTP listening", zap.String("addr", ":8080"))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http serve failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down servers…")

	grpcSrv.GracefulStop()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		logger.Error("http shutdown error", zap.Error(err))
	}

	logger.Info("servers stopped")
}
