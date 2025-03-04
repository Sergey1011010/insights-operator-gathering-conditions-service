package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RedHatInsights/insights-operator-utils/logger"
	"github.com/gorilla/mux"
	"github.com/redhatinsights/insights-operator-conditional-gathering/internal/config"
	"github.com/redhatinsights/insights-operator-conditional-gathering/internal/server"
	"github.com/redhatinsights/insights-operator-conditional-gathering/internal/service"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

const (
	defaultConfigFile = "config/config"
)

func main() {
	var httpServer *server.Server

	// Load config
	err := config.LoadConfiguration(defaultConfigFile)
	if err != nil {
		log.Error().Err(err).Msg("Configuration could not be loaded")
		os.Exit(1)
	}

	serverConfig := config.ServerConfig()
	storageConfig := config.StorageConfig()

	// Logger
	err = logger.InitZerolog(
		config.LoggingConfig(),
		config.CloudWatchConfig(),
		config.SentryLoggingConfig(),
		config.KafkaZerologConfig(),
	)
	if err != nil {
		log.Error().Err(err).Msg("Logger could not be initialized")
		os.Exit(1)
	}

	// Storage
	if _, err = os.Stat(storageConfig.RulesPath); err != nil {
		log.Error().Err(err).Msg("Storage data path not found")
		os.Exit(1)
	}
	store := service.NewStorage(storageConfig)

	// Repository & Service
	repo := service.NewRepository(store)
	svc := service.New(repo)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// HTTP
	g.Go(func() error {
		router := mux.NewRouter().StrictSlash(true)

		// Register the service
		service.NewHandler(svc).Register(router)

		// Create the HTTP Server
		httpServer = server.New(serverConfig, router)

		err = httpServer.Start()
		if err != nil {
			return err
		}

		return nil
	})

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}

	log.Info().Msg("Received shutdown signal")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if httpServer != nil {
		httpServer.Stop(shutdownCtx) // nolint: errcheck
	}

	err = g.Wait()
	if err != nil {
		log.Error().Err(err).Msg("Server returning an error")
		defer os.Exit(2)
	}

	log.Info().Msg("Server closed")
}
