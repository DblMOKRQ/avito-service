package main

import (
	"context"
	"errors"
	"go.uber.org/zap"
	systemLog "log"
	"net/http"

	"avito/internal/config"
	"avito/internal/repository/postgres"
	"avito/internal/service"
	"avito/internal/transport/http/handler"
	"avito/internal/transport/http/router"
	"avito/pkg/logger"
)

func main() {
	ctx := context.Background()
	cfg := config.MustLoad()
	log, err := logger.NewLogger(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := log.Sync()
		if err != nil {
			systemLog.Printf("failed to sync logger: %v", err)
		}
	}()

	storeRepo, err := postgres.NewStore(ctx, cfg.UserRepo, cfg.PasswordRepo, cfg.HostRepo, cfg.PortRepo, cfg.DBName, cfg.SSLMode, log)
	if err != nil {
		log.Error("Failed to initialized to postgres", zap.Error(err))
		return
	}
	userRepo := storeRepo.UserRepository
	prRepo := storeRepo.PullRequestRepository
	statsRepo := storeRepo.StatsRepository

	userSrv := service.NewUserService(&userRepo, &prRepo, log)
	teamSrv := service.NewTeamService(storeRepo, &userRepo, log)
	prSrv := service.NewPullRequestService(&prRepo, userSrv, log)
	statsSrv := service.NewStatsService(&statsRepo, log)

	handl := handler.NewHandler(*teamSrv, *userSrv, *statsSrv, *prSrv)
	rout := router.NewRouter(handl, cfg.LogLevel, log)
	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: rout.GetEngine(),
	}
	log.Info("Starting server", zap.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("Failed to listen and server", zap.Error(err))
		return
	}
}
