package postgres

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"strconv"

	"avito/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	saveUserQuery = `INSERT INTO users (id, username, is_active, team_name) 
					 VALUES ($1, $2, $3, $4)
					 ON CONFLICT (id) DO UPDATE 
					 SET username = EXCLUDED.username, is_active = EXCLUDED.is_active, team_name = EXCLUDED.team_name`

	getByIDQuery = `SELECT id, username, is_active, team_name FROM users WHERE id = $1`

	getActiveTeamMembersQuery = `SELECT id, username, is_active, team_name 
							     FROM users 
							     WHERE team_name = $1 AND is_active = true AND id != ALL($2::uuid[])`

	setIsActiveQuery = `UPDATE users SET is_active = $1 WHERE id = $2`
)

// SaveUser Сохраняет нового или обновляет существующего пользователя
func (r *UserRepository) SaveUser(ctx context.Context, user domain.User) error {
	r.log.Debug("Saving user", zap.Any("user", user))
	_, err := r.pool.Exec(ctx, saveUserQuery, user.ID, user.Username, user.IsActive, user.TeamName)
	if err != nil {
		r.log.Error("Error saving user", zap.Error(err))
		return fmt.Errorf("error saving user: %w", err)
	}
	r.log.Debug("Saved user", zap.Any("user", user))
	return nil
}

// GetUserByID Находит пользователя по ID
func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	r.log.Debug("Getting user by id", zap.String("id", id.String()))
	var user domain.User
	err := r.pool.QueryRow(ctx, getByIDQuery, id).Scan(
		&user.ID,
		&user.Username,
		&user.IsActive,
		&user.TeamName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Warn("User not found", zap.String("id", id.String()))
			return nil, domain.ErrNotFound
		}
		r.log.Error("Error getting user by id", zap.String("id", id.String()))
		return nil, fmt.Errorf("error getting user by id: %w", err)
	}
	r.log.Debug("User found", zap.String("id", id.String()))
	return &user, nil
}

// GetActiveTeamMembers Находит всех активных пользователей в команде, кроме автора
func (r *UserRepository) GetActiveTeamMembers(ctx context.Context, teamName string, excludeIDs []uuid.UUID) ([]domain.User, error) {
	r.log.Debug("Getting active team members", zap.String("team_name", teamName))
	rows, err := r.pool.Query(ctx, getActiveTeamMembersQuery, teamName, excludeIDs)
	if err != nil {
		r.log.Error("Error getting active team members", zap.Error(err))
		return nil, fmt.Errorf("error getting active team members: %w", err)
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		var user domain.User
		err := rows.Scan(&user.ID, &user.Username, &user.IsActive, &user.TeamName)
		if err != nil {
			r.log.Error("Error scanning active team members", zap.Error(err))
			return nil, fmt.Errorf("error scanning active team members: %w", err)
		}
		users = append(users, user)
	}
	r.log.Debug("Users found", zap.Int("count", len(users)))
	return users, nil
}

func (r *UserRepository) SetIsActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	r.log.Debug("Setting is_active", zap.String("id", id.String()), zap.String("is_active", strconv.FormatBool(isActive)))
	commandTag, err := r.pool.Exec(ctx, setIsActiveQuery, isActive, id)
	if err != nil {
		r.log.Error("Error setting IsActive", zap.Error(err))
		return fmt.Errorf("error saving IsActive: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		r.log.Warn("User not found for SetIsActive", zap.String("id", id.String()))
		return domain.ErrNotFound
	}
	r.log.Debug("is_active set successful", zap.String("id", id.String()), zap.String("is_active", strconv.FormatBool(isActive)))
	return nil
}
