// Command server is user-service's entrypoint: it wires config -> Mongo
// connection -> repositories -> services -> handlers -> router ->
// health.Register -> server.Run.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/finora/shared/health"
	"github.com/finora/shared/logger"
	"github.com/finora/shared/mongox"
	"github.com/finora/shared/openapidoc"
	"github.com/finora/shared/server"

	appconfig "github.com/finora/user-service/internal/config"
	"github.com/finora/user-service/internal/handler"
	"github.com/finora/user-service/internal/repository"
	"github.com/finora/user-service/internal/router"
	"github.com/finora/user-service/internal/service"
)

func main() {
	cfg := appconfig.Load()
	log := logger.New("user-service", cfg.LogLevel)

	client, err := mongox.Connect(cfg.MongoURI)
	if err != nil {
		log.Error("failed to connect to mongo", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Error("failed to disconnect from mongo", slog.String("error", err.Error()))
		}
	}()

	db := client.Database(appconfig.DBNameFromURI(cfg.MongoURI))

	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		log.Error("failed to ensure indexes", slog.String("error", err.Error()))
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)

	authSvc := service.NewAuthService(
		userRepo, settingsRepo, tokenRepo,
		cfg.JWTAccessSecret, cfg.JWTRefreshSecret,
		cfg.JWTAccessTTL, cfg.JWTRefreshTTL,
	)
	userSvc := service.NewUserService(userRepo, settingsRepo)

	authHandler := handler.NewAuthHandler(authSvc)
	userHandler := handler.NewUserHandler(userSvc)

	mongoChecker := mongox.Checker{Client: client}
	openapiSpec := openapidoc.Load("openapi.yaml", log)

	r := router.New(log, cfg.CORSAllowedOrigins, authHandler, userHandler, openapiSpec, health.Checker(mongoChecker))

	addr := "0.0.0.0:" + cfg.Port
	if err := server.Run(addr, r, log, cfg.ShutdownTimeout); err != nil {
		log.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
