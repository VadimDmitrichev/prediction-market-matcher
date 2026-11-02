package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"prediction-market-matcher/config"
	local_grpc "prediction-market-matcher/internal/grpc"
	pb "prediction-market-matcher/proto/gen/marketsimilarity"
	logger "prediction-market-matcher/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "../../config.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	log, err := logger.InitLogger(cfg)
	if err != nil {
		slog.Error("failed to initialize logger", "error", err)
		os.Exit(1)
	}

	port := cfg.GRPCPort
	if port == "" {
		port = ":50051"
	}

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Error("failed to start TCP listener", "port", port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(10*1024*1024), // 10 MB
		grpc.MaxSendMsgSize(10*1024*1024),
	)

	marketHandler := local_grpc.NewMarketHandler(cfg, log)
	pb.RegisterMarketComparisonServiceServer(grpcServer, marketHandler)
	reflection.Register(grpcServer)

	// Channel to listen for OS interrupt signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Info("gRPC server started", "port", port, "workers", cfg.WorkerCount)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	sig := <-stop
	log.Info("shutdown signal received, stopping gRPC server gracefully...", "signal", sig.String())

	grpcServer.GracefulStop()
	log.Info("gRPC server stopped")
}
