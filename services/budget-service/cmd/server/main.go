// Command server is the entrypoint for budget-service: it wires
// config -> mongo connect -> repositories -> services -> handlers -> router
// -> health.Register -> server.Run.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/finora/budget-service/internal/client"
	"github.com/finora/budget-service/internal/config"
	"github.com/finora/budget-service/internal/handler"
	"github.com/finora/budget-service/internal/repository"
	"github.com/finora/budget-service/internal/router"
	"github.com/finora/budget-service/internal/service"
	"github.com/finora/shared/logger"
	"github.com/finora/shared/mongox"
	"github.com/finora/shared/openapidoc"
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

	budgetRepo := repository.NewMongoBudgetRepository(db)
	goalRepo := repository.NewMongoGoalRepository(db)
	expenseClient := client.NewExpenseHTTPClient(cfg.ExpenseServiceURL)
	notificationClient := client.NewNotificationHTTPClient(cfg.NotificationServiceURL)

	budgetService := service.NewBudgetService(budgetRepo)
	goalService := service.NewGoalService(goalRepo)
	reportService := service.NewReportService(budgetRepo, expenseClient, notificationClient, log)

	budgetHandler := handler.NewBudgetHandler(budgetService)
	goalHandler := handler.NewGoalHandler(goalService)
	reportHandler := handler.NewReportHandler(reportService)

	mongoChecker := mongox.Checker{Client: mongoClient}
	openapiSpec := openapidoc.Load("openapi.yaml", log)

	r := router.New(log, cfg.CORSAllowedOrigins, budgetHandler, goalHandler, reportHandler, openapiSpec, mongoChecker)

	addr := "0.0.0.0:" + cfg.Port
	if err := server.Run(addr, r, log, cfg.ShutdownTimeout); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
