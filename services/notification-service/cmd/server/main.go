// Command server is the entrypoint for notification-service: it wires
// config -> mongo connect -> repositories -> services -> handlers -> router
// -> health.Register -> server.Run.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/finora/notification-service/internal/config"
	"github.com/finora/notification-service/internal/handler"
	"github.com/finora/notification-service/internal/repository"
	"github.com/finora/notification-service/internal/router"
	"github.com/finora/notification-service/internal/service"
	"github.com/finora/shared/logger"
	"github.com/finora/shared/mongox"
	"github.com/finora/shared/openapidoc"
	"github.com/finora/shared/server"
)

func main() {
	cfg := config.Load()
	log := logger.New("notification-service", cfg.LogLevel)

	mongoClient, err := mongox.Connect(cfg.MongoURI)
	if err != nil {
		log.Error("failed to connect to mongo", slog.String("error", err.Error()))
		os.Exit(1)
	}
	db := mongoClient.Database("finora_notifications")

	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		log.Error("failed to ensure mongo indexes", slog.String("error", err.Error()))
		os.Exit(1)
	}

	notificationRepo := repository.NewMongoNotificationRepository(db)

	// LoggingEmailSender is invoked by notificationService.Create (Phase 4)
	// as a best-effort side effect — it doesn't actually send anything, just
	// logs what would have been sent, which is the roadmap's accepted
	// "structured log" alternative to a real SMTP sender.
	emailSender := service.NewLoggingEmailSender(log)

	notificationService := service.NewNotificationService(notificationRepo, emailSender, log)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	mongoChecker := mongox.Checker{Client: mongoClient}
	openapiSpec := openapidoc.Load("openapi.yaml", log)

	r := router.New(log, cfg.CORSAllowedOrigins, notificationHandler, openapiSpec, mongoChecker)

	addr := "0.0.0.0:" + cfg.Port
	if err := server.Run(addr, r, log, cfg.ShutdownTimeout); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
