// Command server is the Finora API gateway: the single public entry point
// that validates JWTs and reverse-proxies every /api/v1/* request to the
// owning backend service (user-service, expense-service, budget-service,
// notification-service).
package main

import (
	"log/slog"
	"os"

	"github.com/finora/shared/logger"
	"github.com/finora/shared/openapidoc"
	"github.com/finora/shared/server"

	"github.com/finora/gateway/internal/config"
	"github.com/finora/gateway/internal/proxy"
	"github.com/finora/gateway/internal/router"
)

func main() {
	cfg := config.Load()
	log := logger.New("gateway", cfg.LogLevel)

	userProxy, err := proxy.New(cfg.UserServiceURL, log)
	if err != nil {
		log.Error("invalid USER_SERVICE_URL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	expenseProxy, err := proxy.New(cfg.ExpenseServiceURL, log)
	if err != nil {
		log.Error("invalid EXPENSE_SERVICE_URL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	budgetProxy, err := proxy.New(cfg.BudgetServiceURL, log)
	if err != nil {
		log.Error("invalid BUDGET_SERVICE_URL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	notificationProxy, err := proxy.New(cfg.NotificationServiceURL, log)
	if err != nil {
		log.Error("invalid NOTIFICATION_SERVICE_URL", slog.String("error", err.Error()))
		os.Exit(1)
	}

	openapiSpec := openapidoc.Load("openapi.yaml", log)

	engine := router.New(cfg, log, router.Backends{
		User:         userProxy,
		Expense:      expenseProxy,
		Budget:       budgetProxy,
		Notification: notificationProxy,
	}, openapiSpec)

	addr := "0.0.0.0:" + cfg.GatewayPort
	if err := server.Run(addr, engine, log, cfg.ShutdownTimeout); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
