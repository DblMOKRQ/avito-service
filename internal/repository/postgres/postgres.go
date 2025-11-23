package postgres

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"time"

	"avito/internal/domain"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

type StatsRepository struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

type UserRepository struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

type TeamRepository struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

type PullRequestRepository struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

type Store struct {
	pool *pgxpool.Pool
	UserRepository
	TeamRepository
	PullRequestRepository
	StatsRepository
	log *zap.Logger
}

func NewStore(ctx context.Context, user string, password string, host string, port string, dbname string, sslmode string, log *zap.Logger) (*Store, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbname, sslmode)

	log = log.With(zap.String("dbname", dbname),
		zap.String("host:port", fmt.Sprintf("%s:%s", host, port)),
		zap.String("user", user),
	)

	log.Info("Connecting to PostgreSQL")

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Error("Error parsing connection string", zap.Error(err))
		return nil, fmt.Errorf("error parsing connection string: %w", err)
	}
	config.MaxConns = 50
	config.HealthCheckPeriod = 30 * time.Second
	config.MinConns = 2

	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Error("Failed connecting to PostgreSQL", zap.Error(err))
		return nil, fmt.Errorf("error connecting to PostgreSQL: %w", err)
	}

	log.Info("Testing database connection")
	if err := db.Ping(ctx); err != nil {
		log.Error("failed pinging PostgreSQL", zap.Error(err))
		return nil, fmt.Errorf("failed pinging PostgreSQL: %w", err)
	}

	log.Info("Successfully connected to PostgreSQL")

	log.Info("Starting database migrations")

	if err := runMigrations(connStr); err != nil {
		log.Error("Failed to run migrations", zap.Error(err))
		return nil, fmt.Errorf("failed to run migration: %w", err)
	}

	log.Info("Successfully migrated database")

	return &Store{
		pool:                  db,
		UserRepository:        UserRepository{pool: db, log: log},
		TeamRepository:        TeamRepository{pool: db, log: log},
		PullRequestRepository: PullRequestRepository{pool: db, log: log},
		StatsRepository:       StatsRepository{pool: db, log: log},
		log:                   log.Named("Repository"),
	}, nil
}

// CreateTeamWithMembersTx создает команду и ее участников в одной атомарной транзакции
func (s *Store) CreateTeamWithMembersTx(ctx context.Context, team domain.Team) error {
	log := s.log.With(zap.String("team_name", team.Name))
	log.Debug("Creating team with members in a transaction")

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Error("Failed to begin transaction", zap.Error(err))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Error("Failed to rollback transaction", zap.Error(err))
		}
	}()

	if _, err := tx.Exec(ctx, saveTeamQuery, team.Name); err != nil {
		log.Error("Failed to save team within transaction", zap.Error(err))
		return fmt.Errorf("failed to save team: %w", err)
	}

	for _, member := range team.Members {
		member.TeamName = team.Name
		_, err := tx.Exec(ctx, saveUserQuery, member.ID, member.Username, member.IsActive, member.TeamName)
		if err != nil {
			log.Error("Failed to save user within transaction", zap.String("user_id", member.ID.String()), zap.Error(err))
			return fmt.Errorf("failed to save user: %w", err)
		}
	}

	log.Debug("Committing transaction for team creation")
	return tx.Commit(ctx)
}

func (r *Store) Close() {
	r.log.Info("Closing database connection")
	r.pool.Close()
}

func runMigrations(connStr string) error {
	migratePath := os.Getenv("MIGRATE_PATH")
	if migratePath == "" {
		migratePath = "./migrations"
	}
	absPath, err := filepath.Abs(migratePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	absPath = filepath.ToSlash(absPath)
	migrateUrl := fmt.Sprintf("file://%s", absPath)
	m, err := migrate.New(migrateUrl, connStr)
	if err != nil {
		return fmt.Errorf("start migrations error %v", err)
	}
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("migration up error: %v", err)
	}
	return nil
}
