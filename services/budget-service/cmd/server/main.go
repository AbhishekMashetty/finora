// Command server is the entrypoint for budget-service: it wires
// config -> mongo connect -> repositories -> services -> handlers -> router
// -> health.Register -> server.Run.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/finora/budget-service/internal/client"
	"github.com/finora/budget-service/internal/config"
	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/budget-service/internal/events"
	"github.com/finora/budget-service/internal/handler"
	"github.com/finora/budget-service/internal/repository"
	"github.com/finora/budget-service/internal/router"
	"github.com/finora/budget-service/internal/service"
	"github.com/finora/shared/eventbus"
	"github.com/finora/shared/logger"
	"github.com/finora/shared/mongox"
	"github.com/finora/shared/openapidoc"
	"github.com/finora/shared/outbox"
	"github.com/finora/shared/server"
)

func main() {
	cfg := config.Load()
	log := logger.New("budget-service", cfg.LogLevel)

	mongoClient, err := mongox.Connect(cfg.MongoURI)
	if err != nil {
		log.Error("failed to connect to mongo", slog.String("error", err.Error()))
		os.Exit(1)
	}
	db := mongoClient.Database(config.DBNameFromURI(cfg.MongoURI))

	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		log.Error("failed to ensure mongo indexes", slog.String("error", err.Error()))
		os.Exit(1)
	}

	outboxStore := outbox.NewStore(db)
	if err := outboxStore.EnsureIndexes(context.Background()); err != nil {
		log.Error("failed to ensure outbox indexes", slog.String("error", err.Error()))
		os.Exit(1)
	}

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

	// Cancelled only at process exit — the outbox relay and the
	// transaction.created consumer both run for the process's whole
	// lifetime, alongside server.Run below.
	bgCtx, cancelBG := context.WithCancel(context.Background())
	defer cancelBG()

	budgetRepo := repository.NewMongoBudgetRepository(db)
	goalRepo := repository.NewMongoGoalRepository(db)
	expenseClient := client.NewExpenseHTTPClient(cfg.ExpenseServiceURL)
	eventPublisher := events.NewOutboxPublisher(outboxStore)

	relay := outbox.NewRelay(outboxStore, bus, log)
	go relay.Run(bgCtx, cfg.OutboxRelayInterval)

	overspendService := service.NewOverspendService(budgetRepo, expenseClient, eventPublisher, log)
	go func() {
		err := bus.Subscribe(bgCtx, domain.EventsStreamName, "budget-service-transaction-created", domain.TransactionCreatedSubject,
			func(ctx context.Context, _ string, data []byte) error {
				var event domain.TransactionCreatedEvent
				if err := json.Unmarshal(data, &event); err != nil {
					// A malformed event is not retryable — acking (returning
					// nil) is correct here so it doesn't churn through
					// MaxDeliver retries for a payload that will never parse.
					log.Error("failed to unmarshal transaction.created event, discarding", slog.String("error", err.Error()))
					return nil
				}
				return overspendService.HandleTransactionCreated(ctx, event.UserID, time.Now().UTC())
			})
		if err != nil {
			log.Error("transaction.created subscription exited with error", slog.String("error", err.Error()))
		}
	}()

	budgetService := service.NewBudgetService(budgetRepo)
	goalService := service.NewGoalService(goalRepo)
	reportService := service.NewReportService(budgetRepo, expenseClient)

	budgetHandler := handler.NewBudgetHandler(budgetService)
	goalHandler := handler.NewGoalHandler(goalService)
	reportHandler := handler.NewReportHandler(reportService)

	mongoChecker := mongox.Checker{Client: mongoClient}
	natsChecker := eventbus.Checker{Bus: bus}
	openapiSpec := openapidoc.Load("openapi.yaml", log)

	r := router.New(log, cfg.CORSAllowedOrigins, budgetHandler, goalHandler, reportHandler, openapiSpec, mongoChecker, natsChecker)

	addr := "0.0.0.0:" + cfg.Port
	if err := server.Run(addr, r, log, cfg.ShutdownTimeout); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
