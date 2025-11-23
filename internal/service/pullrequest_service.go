package service

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"math/rand"
	"time"

	"avito/internal/domain"

	"github.com/google/uuid"
)

const (
	// countReviewers определяет, сколько ревьюеров назначается при создании PR.
	countReviewers = 2
	// countReassignReviewer определяет, сколько ревьюеров выбирается для замены.
	countReassignReviewer = 1
)

type PullRequestRepo interface {
	Create(ctx context.Context, pr *domain.PullRequest) error
	GetPRByID(ctx context.Context, id string) (*domain.PullRequest, error)
	ReassignReviewer(ctx context.Context, reasReviewer domain.Reassignment) error
	Exists(ctx context.Context, id string) (bool, error)
	SetMerge(ctx context.Context, id string) error
}

type UserProviderForPR interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetActiveTeamMembers(ctx context.Context, teamName string, excludeIDs []uuid.UUID) ([]domain.User, error)
}
type PullRequestService struct {
	prRepo  PullRequestRepo
	userSvc UserProviderForPR
	log     *zap.Logger
}

func NewPullRequestService(prRepo PullRequestRepo, userSvc UserProviderForPR, log *zap.Logger) *PullRequestService {
	return &PullRequestService{
		prRepo:  prRepo,
		userSvc: userSvc,
		log:     log.Named("PullRequestService"),
	}
}

// CreatePR обрабатывает создание нового Pull Request и назначение ревьюеров
func (pr *PullRequestService) CreatePR(ctx context.Context, prID string, prName string, authorID uuid.UUID) (*domain.PullRequest, error) {
	log := pr.log.With(zap.String("pr_id", prID), zap.String("method", "CreatePR"))
	exists, err := pr.prRepo.Exists(ctx, prID)
	if err != nil {
		log.Error("Failed to check pr existence", zap.Error(err))
		return nil, fmt.Errorf("failed to check pr existence: %w", err)
	}
	if exists {
		log.Error("Pull request already exists")
		return nil, domain.ErrPRExists
	}

	author, err := pr.userSvc.GetUserByID(ctx, authorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("author not found", zap.String("authorID", authorID.String()))
			return nil, domain.ErrNotFound
		}
		log.Error("Failed to get author by author id", zap.Error(err))
		return nil, fmt.Errorf("failed to get author by author id: %w", err)
	}

	if !author.IsActive {
		log.Warn("Attempted to create PR with an inactive author", zap.String("author_id", author.ID.String()))
		return nil, domain.ErrAuthorIsInactive
	}

	excludeID := []uuid.UUID{author.ID}
	activeMembers, err := pr.userSvc.GetActiveTeamMembers(ctx, author.TeamName, excludeID)
	if err != nil {
		log.Error("Failed to get active team members", zap.Error(err))
		return nil, fmt.Errorf("failed to get active team members: %w", err)
	}
	pullRequest := domain.PullRequest{
		ID:                prID,
		Name:              prName,
		Status:            domain.StatusOpen,
		AuthorID:          authorID,
		AssignedReviewers: pr.getRandomReviewers(activeMembers, countReviewers),
		CreatedAt:         time.Now().UTC(),
	}
	if err := pr.prRepo.Create(ctx, &pullRequest); err != nil {
		log.Error("Failed to create pull request", zap.Error(err))
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}
	return &pullRequest, nil

}

// ReassignmentReviewers обрабатывает логику замены одного ревьюера на другого
func (pr *PullRequestService) ReassignmentReviewers(ctx context.Context, prID string, oldUserID uuid.UUID) (*domain.PullRequest, string, error) {
	log := pr.log.With(zap.String("pr_id", prID), zap.String("method", "ReassignmentReviewers"))
	exists, err := pr.prRepo.Exists(ctx, prID)
	if err != nil {
		log.Error("Failed to check pr existence", zap.Error(err))
		return nil, "", fmt.Errorf("failed to check pr existence: %w", err)
	}
	if !exists {
		log.Error("Pull request does not exist")
		return nil, "", domain.ErrPRExists
	}

	pullRequest, err := pr.prRepo.GetPRByID(ctx, prID)
	if err != nil {
		log.Error("Failed to get pull request", zap.Error(err))
		return nil, "", fmt.Errorf("failed to get pull request: %w", err)
	}

	if pullRequest.Status == domain.StatusMerged {
		log.Warn("cannot reassign on a merged PR")
		return nil, "", domain.ErrPRMerged
	}

	author, err := pr.userSvc.GetUserByID(ctx, pullRequest.AuthorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("author not found", zap.String("authorID", pullRequest.AuthorID.String()))
			return nil, "", domain.ErrNotFound
		}
		log.Error("Failed to get user by author id", zap.Error(err))
		return nil, "", fmt.Errorf("failed to get user by author id: %w", err)
	}

	if author.ID == oldUserID {
		log.Warn("Author of the pull request cannot be deleted")
		return nil, "", domain.ErrAuthorCannotDelete
	}
	excludeIDs := make([]uuid.UUID, 0, countReviewers)
	excludeIDs = append(excludeIDs, pullRequest.AssignedReviewers...)
	excludeIDs = append(excludeIDs, author.ID)

	candidates, err := pr.userSvc.GetActiveTeamMembers(ctx, author.TeamName, excludeIDs)

	if err != nil {
		log.Error("Failed to get active team members", zap.Error(err))
		return nil, "", fmt.Errorf("failed to get active team members: %w", err)
	}
	newReviewerIDs := pr.getRandomReviewers(candidates, countReassignReviewer)
	if len(newReviewerIDs) == 0 {
		log.Warn("No active replacement candidate in team")
		return nil, "", domain.ErrNoCandidate
	}
	newReviewerID := newReviewerIDs[0]
	reassignment := domain.Reassignment{
		PullRequestID: prID,
		OldUserID:     oldUserID,
		NewUserID:     newReviewerID,
	}
	if err := pr.prRepo.ReassignReviewer(ctx, reassignment); err != nil {
		if errors.Is(err, domain.ErrUserNotAssigned) {
			log.Warn("user to be reassigned is not currently a reviewer", zap.String("old_user_id", oldUserID.String()))
			return nil, "", domain.ErrUserNotAssigned
		}

		log.Error("Failed to update Pull request", zap.Error(err))
		return nil, "", fmt.Errorf("failed to update Pull request: %w", err)
	}

	updatedPullRequest, err := pr.prRepo.GetPRByID(ctx, prID)
	if err != nil {
		pr.log.Error("Failed to get updated pull request", zap.Error(err))
		return nil, "", fmt.Errorf("failed to get updated pull request: %w", err)
	}

	return updatedPullRequest, newReviewerID.String(), nil
}

// SetMerge обрабатывает "мерж" Pull Request'а
func (pr *PullRequestService) SetMerge(ctx context.Context, prID string) (*domain.PullRequest, error) {
	log := pr.log.With(zap.String("pr_id", prID), zap.String("method", "SetMerge"))
	exists, err := pr.prRepo.Exists(ctx, prID)
	if err != nil {
		log.Error("Failed to check pr existence", zap.Error(err))
		return nil, fmt.Errorf("failed to check pr existence: %w", err)
	}
	if !exists {
		log.Error("Pull request does not exist")
		return nil, domain.ErrPRNotExist
	}
	err = pr.prRepo.SetMerge(ctx, prID)
	if err != nil {
		log.Error("Failed to set pull request merge", zap.Error(err))
		return nil, fmt.Errorf("failed to set pull request merge: %w", err)
	}

	pullRequest, err := pr.prRepo.GetPRByID(ctx, prID)
	if err != nil {
		log.Error("Failed to get pull request", zap.Error(err))
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	return pullRequest, nil
}

// getRandomReviewers метод для выбора случайных ревьюеров из списка кандидатов
func (pr *PullRequestService) getRandomReviewers(users []domain.User, count int) []uuid.UUID {
	if len(users) == 0 {
		pr.log.Warn("no active members available for review assignment")
		return []uuid.UUID{}
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(users), func(i, j int) {
		users[i], users[j] = users[j], users[i]
	})

	numToAssign := count
	if len(users) < count {
		numToAssign = len(users)
	}

	reviewers := make([]uuid.UUID, numToAssign)
	for i := 0; i < numToAssign; i++ {
		reviewers[i] = users[i].ID
	}

	pr.log.Debug("selected random reviewers", zap.Int("count", len(reviewers)))
	return reviewers
}
