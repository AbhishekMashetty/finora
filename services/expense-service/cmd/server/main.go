// Command server is the entrypoint for expense-service: it wires config ->
// mongo connection -> repositories -> services -> handlers -> router ->
// health checks -> the graceful-shutdown HTTP server.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/finora/expense-service/internal/config"
	"github.com/finora/expense-service/internal/handler"
	"github.com/finora/expense-service/internal/repository"
	"github.com/finora/expense-service/internal/router"
	"github.com/finora/expense-service/internal/service"
	"github.com/finora/shared/health"
	"github.com/finora/shared/logger"
	"github.com/finora/shared/mongox"
	"github.com/finora/shared/openapidoc"
	"github.com/finora/shared/server"
)

const serviceName = "expense-service"

func main() {
	cfg := config.Load()
	log := logger.New(serviceName, cfg.LogLevel)

	client, err := mongox.Connect(cfg.MongoURI)
	if err != nil {
		log.Error("failed to connect to mongodb", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Error("failed to disconnect from mongodb", slog.String("error", err.Error()))
		}
	}()

	db := client.Database("finora_expenses")

	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		log.Error("failed to ensure indexes", slog.String("error", err.Error()))
		os.Exit(1)
	}

	accountRepo := repository.NewAccountRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)

	accountSvc := service.NewAccountService(accountRepo)
	transactionSvc := service.NewTransactionService(transactionRepo, accountRepo)
	categorySvc := service.NewCategoryService(categoryRepo)

	accountHandler := handler.NewAccountHandler(accountSvc)
	transactionHandler := handler.NewTransactionHandler(transactionSvc)
	categoryHandler := handler.NewCategoryHandler(categorySvc)

	openapiSpec := openapidoc.Load("openapi.yaml", log)

	r := router.New(router.Deps{
		Logger:             log,
		ServiceName:        serviceName,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		HealthCheckers:     []health.Checker{mongox.Checker{Client: client}},
		AccountHandler:     accountHandler,
		TransactionHandler: transactionHandler,
		CategoryHandler:    categoryHandler,
		OpenAPISpec:        openapiSpec,
	})

	if err := server.Run(cfg.Addr(), r, log, cfg.ShutdownTimeout); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
