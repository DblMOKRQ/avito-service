package service

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"

	"avito/internal/domain"

	"github.com/google/uuid"
)

type PullRequestProviderForUser interface {
	GetByReviewerID(ctx context.Context, reviewerID uuid.UUID) ([]*domain.PullRequest, error)
}

type UserRepository interface {
	SaveUser(ctx context.Context, user domain.User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetActiveTeamMembers(ctx context.Context, teamName string, excludeIDs []uuid.UUID) ([]domain.User, error)
	SetIsActive(ctx context.Context, id uuid.UUID, isActive bool) error
}

type UserService struct {
	userRepo UserRepository
	prRepo   PullRequestProviderForUser
	log      *zap.Logger
}

func NewUserService(userRepo UserRepository, prRepo PullRequestProviderForUser, log *zap.Logger) *UserService {
	return &UserService{
		userRepo: userRepo,
		prRepo:   prRepo,
		log:      log.Named("UserService"),
	}
}

func (us *UserService) SaveUser(ctx context.Context, user domain.User) error {
	if user.ID == uuid.Nil {
		us.log.Error("attempted to save user with nil ID")
		return domain.ErrUserIDNil
	}
	if err := us.userRepo.SaveUser(ctx, user); err != nil {
		us.log.Error("failed to save user", zap.Error(err))
		return fmt.Errorf("failed to save user: %w", err)
	}
	return nil
}

func (us *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if id == uuid.Nil {
		us.log.Warn("user not found, id is null", zap.String("id", id.String()))
		return nil, domain.ErrOneOfParametersNil
	}
	user, err := us.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			us.log.Warn("user not found", zap.String("id", id.String()))
			return nil, domain.ErrNotFound
		}
		us.log.Error("failed to get user", zap.String("id", id.String()))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (us *UserService) GetActiveTeamMembers(ctx context.Context, teamName string, excludeIDs []uuid.UUID) ([]domain.User, error) {
	if teamName == "" {
		us.log.Warn("users not found, teamName is null", zap.String("teamName", teamName))
		return nil, domain.ErrOneOfParametersNil
	}
	if excludeIDs == nil {
		us.log.Warn("users not found, excludeIDs is nil")
		return nil, domain.ErrOneOfParametersNil
	}
	users, err := us.userRepo.GetActiveTeamMembers(ctx, teamName, excludeIDs)
	if err != nil {
		us.log.Error("Failed to get active team members", zap.String("teamName", teamName), zap.Any("excludeIDs", excludeIDs), zap.Error(err))
		return nil, fmt.Errorf("failed to get active team members: %w", err)
	}
	return users, nil
}

func (us *UserService) SetIsActive(ctx context.Context, id uuid.UUID, isActive bool) (*domain.User, error) {
	if id == uuid.Nil {
		us.log.Warn("Failed to setting is_active, id is null", zap.String("id", id.String()))
		return nil, domain.ErrOneOfParametersNil
	}
	err := us.userRepo.SetIsActive(ctx, id, isActive)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			us.log.Warn("User not found for SetIsActive", zap.String("id", id.String()))
			return nil, domain.ErrNotFound
		}
		us.log.Error("failed to set is_active", zap.String("id", id.String()))
		return nil, fmt.Errorf("failed to set is_active: %w", err)
	}

	user, err := us.userRepo.GetUserByID(ctx, id)
	if err != nil {
		us.log.Error("failed to get user", zap.String("id", id.String()))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetReviewsForUser Возвращает список всех PR, где указанный пользователь назначен ревьюером
func (us *UserService) GetReviewsForUser(ctx context.Context, userID uuid.UUID) ([]*domain.PullRequest, error) {
	log := us.log.With(zap.String("user_id", userID.String()))

	_, err := us.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("attempted to get reviews for a non-existent user")
			return nil, domain.ErrNotFound
		}
		log.Error("failed to check user existence before getting reviews", zap.Error(err))
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}

	log.Debug("fetching pull requests for review")
	pullRequests, err := us.prRepo.GetByReviewerID(ctx, userID)
	if err != nil {
		log.Error("failed to get pull requests from repo", zap.Error(err))
		return nil, fmt.Errorf("failed to get pull requests for user: %w", err)
	}

	log.Info("successfully fetched pull requests for review", zap.Int("count", len(pullRequests)))
	return pullRequests, nil
}
