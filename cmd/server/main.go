package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"pubsub/config"
	"pubsub/logger"
	"pubsub/protos/gen"
	"pubsub/service"
	"pubsub/subpub"

	"google.golang.org/grpc"
)

func monitorGoroutines() {
	for {
		time.Sleep(5 * time.Second)
		fmt.Printf("Number of goroutines: %d\n", runtime.NumGoroutine())
	}
}

func main() {
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	// config
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// log
	log := logger.New(cfg.LogLevel)
	log.Info("Starting PubSub service")

	// internal server
	sp := subpub.NewSubPub()

	grpcServer := grpc.NewServer()

	// services
	pubSubService := service.NewPubSubService(sp, log)

	gen.RegisterPubSubServer(grpcServer, pubSubService)
	log.Info("Registered PubSub service")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		log.Fatal("Failed to listen: %v", err)
	}
	log.Info("Server listening on port %d", cfg.GRPCPort)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("Failed to serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go monitorGoroutines()
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	grpcServer.GracefulStop()
	log.Info("gRPC server stopped")

	if err := sp.Close(ctx); err != nil {
		log.Error("Error closing SubPub: %v", err)
	} else {
		log.Info("SubPub closed")
	}

	cancel()

	log.Info("Server shutdown complete")
}
