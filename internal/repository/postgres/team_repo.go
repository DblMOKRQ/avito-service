package postgres

import (
	"context"
	"fmt"
	"go.uber.org/zap"

	"avito/internal/domain"

	"github.com/google/uuid"
)

const (
	saveTeamQuery      = `INSERT INTO teams (name) VALUES ($1)`
	getTeamByNameQuery = `SELECT t.name, u.id, u.username, u.is_active, u.team_name
                            FROM teams t
                            LEFT JOIN users u ON t.name = u.team_name
                            WHERE t.name = $1`

	existsTeamQuery = `SELECT EXISTS(SELECT name FROM teams WHERE name = $1)`
)

// SaveTeam Сохраняет новую команду.
func (r *TeamRepository) SaveTeam(ctx context.Context, team domain.Team) error {
	r.log.Debug("Saving team", zap.Any("team", team))
	_, err := r.pool.Exec(ctx, saveTeamQuery, team.Name)
	if err != nil {
		r.log.Error("Failed to save team", zap.Any("team", team), zap.Error(err))
		return fmt.Errorf("failed to save team: %w", err)
	}
	r.log.Debug("Saved team", zap.Any("team", team))
	return nil
}

func (r *TeamRepository) GetTeamByName(ctx context.Context, name string) (*domain.Team, error) {
	r.log.Debug("Getting team by name", zap.String("name", name))

	rows, err := r.pool.Query(ctx, getTeamByNameQuery, name)
	if err != nil {
		r.log.Error("Failed to query team by name", zap.String("name", name), zap.Error(err))
		return nil, fmt.Errorf("failed to query team by name: %w", err)
	}
	defer rows.Close()

	var team *domain.Team
	members := make([]domain.User, 0)

	for rows.Next() {
		var user domain.User
		var teamName string
		err = rows.Scan(&teamName, &user.ID, &user.Username, &user.IsActive, &user.TeamName)
		if err != nil {
			r.log.Error("Failed to scan team member row", zap.String("name", name), zap.Error(err))
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if team == nil {
			team = &domain.Team{Name: teamName}
		}

		if user.ID != uuid.Nil {
			members = append(members, user)
		}
	}

	if err := rows.Err(); err != nil {
		r.log.Error("Error after iterating over team members", zap.String("name", name), zap.Error(err))
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	if team == nil {
		r.log.Warn("Team not found", zap.String("name", name))
		return nil, domain.ErrNotFound
	}

	team.Members = members
	r.log.Debug("Team found", zap.Any("team", team))
	return team, nil
}

func (r *TeamRepository) ExistsTeam(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, existsTeamQuery, name).Scan(&exists)
	if err != nil {
		r.log.Error("Failed to check if team exists", zap.String("name", name), zap.Error(err))
		return false, fmt.Errorf("failed to check existence: %w", err)
	}
	return exists, nil
}
