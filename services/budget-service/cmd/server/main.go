// Command server is the entrypoint for budget-service: it wires
// config -> mongo connect -> repositories -> services -> handlers -> router
// -> health.Register -> server.Run.
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/finora/shared/health"
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

	bus, err := eventbus.Connect(cfg.NATSURL, log)
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
					// A malformed event is not retryable, but it's also not
					// nothing — wrapping eventbus.ErrTerminal tells Subscribe
					// to Term() it immediately (no MaxDeliver churn on a
					// payload that will never parse) AND publish it to a
					// dead-letter subject, so this failure is a durable,
					// observable event instead of one that just vanishes
					// after this log line scrolls off.
					return fmt.Errorf("%w: unmarshal transaction.created event: %v", eventbus.ErrTerminal, err)
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

	// gate is flipped not-ready by server.Run at the start of its shutdown
	// sequence, before the listener actually stops accepting connections —
	// see shared/health.Gate and shared/server.Run's doc comments.
	gate := &health.Gate{}

	r := router.New(log, cfg.CORSAllowedOrigins, budgetHandler, goalHandler, reportHandler, openapiSpec, mongoChecker, natsChecker, gate)

	addr := "0.0.0.0:" + cfg.Port
	if err := server.Run(addr, r, log, cfg.ShutdownTimeout, cfg.DrainDelay, gate); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
