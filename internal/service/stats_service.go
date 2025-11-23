package service

import (
	"context"
	"fmt"
	"go.uber.org/zap"

	"avito/internal/domain"
)

type StatsRepository interface {
	GetUserReviewStats(ctx context.Context) ([]*domain.UserReviewStat, error)
}

type StatsService struct {
	statsRepo StatsRepository
	log       *zap.Logger
}

func NewStatsService(statsRepo StatsRepository, log *zap.Logger) *StatsService {
	return &StatsService{
		statsRepo: statsRepo,
		log:       log.Named("StatsService"),
	}
}

// GetUserReviewStats возвращает статистику по ревью для всех пользователей.
func (s *StatsService) GetUserReviewStats(ctx context.Context) ([]*domain.UserReviewStat, error) {

	s.log.Info("Fetching user review stats")

	stats, err := s.statsRepo.GetUserReviewStats(ctx)
	if err != nil {
		s.log.Error("Failed to get user review stats from repository", zap.Error(err))
		return nil, fmt.Errorf("failed to get stats from repo: %w", err)
	}

	return stats, nil
}
