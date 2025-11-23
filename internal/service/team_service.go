package service

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"

	"avito/internal/domain"
)

type TeamStore interface {
	SaveTeam(ctx context.Context, team domain.Team) error
	GetTeamByName(ctx context.Context, name string) (*domain.Team, error)
	ExistsTeam(ctx context.Context, name string) (bool, error)
	CreateTeamWithMembersTx(ctx context.Context, team domain.Team) error
}

type UserRepositoryForTeamService interface {
	SaveUser(ctx context.Context, user domain.User) error
}

type TeamService struct {
	teamRepo TeamStore
	userRepo UserRepositoryForTeamService
	log      *zap.Logger
}

func NewTeamService(repo TeamStore, userRepo UserRepositoryForTeamService, log *zap.Logger) *TeamService {
	return &TeamService{
		teamRepo: repo,
		userRepo: userRepo,
		log:      log.Named("TeamService"),
	}
}

// CreateTeamWithMembers создает команду с ее членами
func (ts *TeamService) CreateTeamWithMembers(ctx context.Context, team domain.Team) (*domain.Team, error) {
	// TODO: сделать добавление/ изменение участников
	if team.Name == "" {
		ts.log.Warn("attempt to create team with empty name")
		return nil, domain.ErrOneOfParametersNil
	}
	if len(team.Members) == 0 {
		ts.log.Warn("attempt to create team with empty members")
		return nil, domain.ErrOneOfParametersNil
	}
	exists, err := ts.teamRepo.ExistsTeam(ctx, team.Name)
	if err != nil {
		ts.log.Error("Failed to check if team exists", zap.String("name", team.Name), zap.Error(err))
		return nil, fmt.Errorf("failed to check if team exists: %w", err)
	}
	if exists {
		ts.log.Warn("Team already exists", zap.String("name", team.Name))
		return nil, domain.ErrTeamExists
	}

	if err := ts.teamRepo.CreateTeamWithMembersTx(ctx, team); err != nil {
		ts.log.Error("Failed to create team", zap.String("name", team.Name), zap.Error(err))
		return nil, fmt.Errorf("failed to create team: %w", err)
	}
	ts.log.Info("Team with members created", zap.String("name", team.Name), zap.Int("members", len(team.Members)))
	return &team, nil
}

func (ts *TeamService) GetTeamByName(ctx context.Context, name string) (*domain.Team, error) {
	team, err := ts.teamRepo.GetTeamByName(ctx, name)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			ts.log.Warn("team not found", zap.String("name", name))
			return nil, domain.ErrNotFound
		}
		ts.log.Error("failed to retrieve team", zap.String("name", name), zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve team: %w", err)
	}
	return team, nil
}

func (ts *TeamService) ExistsTeam(ctx context.Context, name string) (bool, error) {
	if name == "" {
		ts.log.Warn("name is null", zap.String("name", name))
		return false, domain.ErrOneOfParametersNil
	}
	exists, err := ts.teamRepo.ExistsTeam(ctx, name)
	if err != nil {
		ts.log.Error("failed to check if team exists", zap.String("name", name), zap.Error(err))
		return false, fmt.Errorf("failed to check if team exists: %w", err)
	}
	return exists, nil
}
