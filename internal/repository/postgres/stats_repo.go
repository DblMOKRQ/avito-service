package postgres

import (
	"context"
	"fmt"
	"go.uber.org/zap"

	"avito/internal/domain"
)

// GetUserReviewStatsQuery - SQL-запрос для сбора статистики.
const GetUserReviewStatsQuery = `
	SELECT
		u.id,
		u.username,
		u.is_active,
		COALESCE(pr_counts.review_count, 0) as review_count
	FROM
		users u
	LEFT JOIN (
		SELECT
			reviewer_id,
			COUNT(*) as review_count
		FROM
			pull_request_reviewers
		GROUP BY
			reviewer_id
	) as pr_counts ON u.id = pr_counts.reviewer_id
	ORDER BY
		review_count DESC;
`

// GetUserReviewStats получает статистику по количеству назначенных ревью для каждого пользователя.
func (r *StatsRepository) GetUserReviewStats(ctx context.Context) ([]*domain.UserReviewStat, error) {
	log := r.log.With(zap.String("repo_method", "GetUserReviewStats"))
	log.Debug("Fetching user review statistics from database")

	rows, err := r.pool.Query(ctx, GetUserReviewStatsQuery)
	if err != nil {
		log.Error("Failed to query user review stats", zap.Error(err))
		return nil, fmt.Errorf("failed to query user review stats: %w", err)
	}
	defer rows.Close()

	var stats []*domain.UserReviewStat
	for rows.Next() {
		var stat domain.UserReviewStat
		if err := rows.Scan(&stat.UserID, &stat.Username, &stat.IsActive, &stat.ReviewCount); err != nil {
			log.Error("Failed to scan user stat row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan user stat: %w", err)
		}
		stats = append(stats, &stat)
	}

	if err := rows.Err(); err != nil {
		log.Error("Error after iterating over stats rows", zap.Error(err))
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	log.Info("Successfully fetched user review statistics", zap.Int("user_count", len(stats)))
	return stats, nil
}
