// Command server is the entrypoint for notification-service: it wires
// config -> mongo connect -> repositories -> services -> handlers -> router
// -> health.Register -> server.Run.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/finora/notification-service/internal/config"
	"github.com/finora/notification-service/internal/domain"
	"github.com/finora/notification-service/internal/handler"
	"github.com/finora/notification-service/internal/repository"
	"github.com/finora/notification-service/internal/router"
	"github.com/finora/notification-service/internal/service"
	"github.com/finora/shared/eventbus"
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

	bus, err := eventbus.Connect(cfg.NATSURL)
	if err != nil {
		log.Error("failed to connect to nats", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer bus.Close()
	if err := bus.EnsureStream(context.Background(), domain.EventsStreamName, []string{"finora.>"}); err != nil {
		log.Error("failed to ensure nats stream", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Cancelled only at process exit — the budget.overspent consumer runs
	// for the process's whole lifetime, alongside server.Run below.
	bgCtx, cancelBG := context.WithCancel(context.Background())
	defer cancelBG()

	overspendConsumer := service.NewOverspendConsumer(notificationService)
	go func() {
		err := bus.Subscribe(bgCtx, domain.EventsStreamName, "notification-service-budget-overspent", domain.BudgetOverspentSubject,
			func(ctx context.Context, _ string, data []byte) error {
				var event domain.BudgetOverspentEvent
				if err := json.Unmarshal(data, &event); err != nil {
					// A malformed event is not retryable — acking (returning
					// nil) is correct here so it doesn't churn through
					// MaxDeliver retries for a payload that will never parse.
					log.Error("failed to unmarshal budget.overspent event, discarding", slog.String("error", err.Error()))
					return nil
				}
				return overspendConsumer.HandleBudgetOverspent(ctx, event)
			})
		if err != nil {
			log.Error("budget.overspent subscription exited with error", slog.String("error", err.Error()))
		}
	}()

	mongoChecker := mongox.Checker{Client: mongoClient}
	natsChecker := eventbus.Checker{Bus: bus}
	openapiSpec := openapidoc.Load("openapi.yaml", log)

	r := router.New(log, cfg.CORSAllowedOrigins, notificationHandler, openapiSpec, mongoChecker, natsChecker)

	addr := "0.0.0.0:" + cfg.Port
	if err := server.Run(addr, r, log, cfg.ShutdownTimeout); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
