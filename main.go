package main

import (
	"auth-service/middleware"
	"auth-service/wire"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	authpb "auth-service/proto/authpb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfgFile := flag.String("config", "", "path to config file (optional)")
	flag.Parse()

	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	authHandler, err := wire.InitializeContainer(*cfgFile, logger)
	if err != nil {
		logger.Fatal("failed to initialize", zap.Error(err))
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryRecovery(logger),
			middleware.UnaryLogger(logger),
		),
	)

	authpb.RegisterAuthServiceServer(srv, authHandler)
	reflection.Register(srv)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatal("listen failed", zap.Error(err))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("shutting down gRPC server…")
		srv.GracefulStop()
	}()

	logger.Info("auth-service listening", zap.String("addr", lis.Addr().String()))
	if err := srv.Serve(lis); err != nil {
		logger.Fatal("serve failed", zap.Error(err))
	}
}
