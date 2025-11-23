package postgres

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"

	"avito/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	createPullRequestQuery = `INSERT INTO pull_requests (id, name, status, author_id, created_at) 
							  VALUES ($1, $2, $3, $4, $5)`

	getPullRequestByIDQuery = `SELECT id, name, status, author_id, created_at, merged_at 
							   FROM pull_requests WHERE id = $1`

	getReviewersForPRQuery = `SELECT reviewer_id FROM pull_request_reviewers WHERE pull_request_id = $1`

	deleteSpecificReviewerQuery = `DELETE FROM pull_request_reviewers WHERE pull_request_id = $1 AND reviewer_id = $2`

	insertSpecificReviewerQuery = `INSERT INTO pull_request_reviewers (pull_request_id, reviewer_id) VALUES ($1, $2)`

	getPRsByReviewerIDQuery = `SELECT p.id, p.name, p.status, p.author_id, p.created_at, p.merged_at
							   FROM pull_requests p
							   JOIN pull_request_reviewers prr ON p.id = prr.pull_request_id
							   WHERE prr.reviewer_id = $1`

	existsPullRequestQuery = `SELECT EXISTS(SELECT 1 FROM pull_requests WHERE id = $1)`

	updatePullRequestStatusQuery = `UPDATE pull_requests SET status = $1, merged_at = NOW() WHERE id = $2`
)

// Create создает новый PR и его ревьюеров в одной транзакции
func (r *PullRequestRepository) Create(ctx context.Context, pr *domain.PullRequest) error {
	log := r.log.With(zap.String("pr_id", pr.ID))
	log.Debug("Creating pull request in a transaction")

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Error("Failed to begin transaction", zap.Error(err))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Error("Failed to rollback transaction", zap.Error(err))
		}
	}()

	log.Debug("Inserting pull request record")
	if _, err := tx.Exec(ctx, createPullRequestQuery, pr.ID, pr.Name, pr.Status, pr.AuthorID, pr.CreatedAt); err != nil {
		log.Error("Failed to insert pull request", zap.Error(err))
		return fmt.Errorf("failed to insert pull request: %w", err)
	}

	if len(pr.AssignedReviewers) > 0 {
		log.Debug("Bulk inserting reviewers", zap.Int("count", len(pr.AssignedReviewers)))

		rows := make([][]interface{}, len(pr.AssignedReviewers))
		for i, reviewerID := range pr.AssignedReviewers {
			rows[i] = []interface{}{pr.ID, reviewerID}
		}

		_, err = tx.CopyFrom(
			ctx,
			pgx.Identifier{"pull_request_reviewers"},
			[]string{"pull_request_id", "reviewer_id"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			log.Error("Failed to bulk insert reviewers", zap.Error(err))
			return fmt.Errorf("failed to copy reviewers: %w", err)
		}
	}

	log.Debug("Committing transaction")
	return tx.Commit(ctx)
}

// GetPRByID находит PR по ID и загружает его ревьюеров
func (r *PullRequestRepository) GetPRByID(ctx context.Context, id string) (*domain.PullRequest, error) {
	log := r.log.With(zap.String("pr_id", id))
	log.Debug("Getting pull request by ID")

	pr := &domain.PullRequest{}

	err := r.pool.QueryRow(ctx, getPullRequestByIDQuery, id).Scan(
		&pr.ID, &pr.Name, &pr.Status, &pr.AuthorID, &pr.CreatedAt, &pr.MergedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warn("Pull request not found")
			return nil, domain.ErrNotFound
		}
		log.Error("Failed to get pull request", zap.Error(err))
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	rows, err := r.pool.Query(ctx, getReviewersForPRQuery, id)
	if err != nil {
		log.Error("Failed to get reviewers for PR", zap.Error(err))
		return nil, fmt.Errorf("failed to get reviewers for PR: %w", err)
	}
	defer rows.Close()

	var reviewers []uuid.UUID
	for rows.Next() {
		var reviewerID uuid.UUID
		if err := rows.Scan(&reviewerID); err != nil {
			log.Error("Failed to scan reviewer ID", zap.Error(err))
			return nil, fmt.Errorf("failed to scan reviewer ID: %w", err)
		}
		reviewers = append(reviewers, reviewerID)
	}

	pr.AssignedReviewers = reviewers
	log.Debug("Pull request retrieved successfully", zap.Int("reviewers_count", len(reviewers)))
	return pr, nil
}

// ReassignReviewer атомарно заменяет одного ревьюера на другого в рамках одной транзакции.
func (r *PullRequestRepository) ReassignReviewer(ctx context.Context, reasReviewer domain.Reassignment) error {
	log := r.log.With(zap.String("pr_id", reasReviewer.PullRequestID))
	log.Debug("Reassigning reviewer in a manual transaction")

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Error("Failed to begin transaction", zap.Error(err))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Error("Failed to rollback transaction", zap.Error(err))
		}
	}()

	cmdTag, err := tx.Exec(ctx, deleteSpecificReviewerQuery, reasReviewer.PullRequestID, reasReviewer.OldUserID)
	if err != nil {
		log.Error("Failed to delete old reviewer", zap.Stringer("old_reviewer", reasReviewer.OldUserID), zap.Error(err))
		return fmt.Errorf("failed to delete old reviewer: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Warn("Old reviewer was not assigned to this PR", zap.Stringer("old_reviewer", reasReviewer.OldUserID))
		return domain.ErrUserNotAssigned
	}

	_, err = tx.Exec(ctx, insertSpecificReviewerQuery, reasReviewer.PullRequestID, reasReviewer.NewUserID)
	if err != nil {
		log.Error("Failed to insert new reviewer", zap.Stringer("new_reviewer", reasReviewer.NewUserID), zap.Error(err))
		return fmt.Errorf("failed to insert new reviewer: %w", err)
	}

	log.Debug("Committing the transaction")
	return tx.Commit(ctx)
}

// GetByReviewerID находит все PR, где пользователь является ревьюером.
func (r *PullRequestRepository) GetByReviewerID(ctx context.Context, reviewerID uuid.UUID) ([]*domain.PullRequest, error) {
	log := r.log.With(zap.String("reviewer_id", reviewerID.String()))
	log.Debug("Getting PRs by reviewer ID")

	rows, err := r.pool.Query(ctx, getPRsByReviewerIDQuery, reviewerID)
	if err != nil {
		log.Error("Failed to query PRs by reviewer ID", zap.Error(err))
		return nil, fmt.Errorf("failed to query PRs by reviewer ID: %w", err)
	}
	defer rows.Close()

	var prs []*domain.PullRequest
	for rows.Next() {
		var pr domain.PullRequest
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.Status, &pr.AuthorID, &pr.CreatedAt, &pr.MergedAt); err != nil {
			log.Error("Failed to scan PR row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan PR: %w", err)
		}
		prs = append(prs, &pr)
	}
	log.Debug("PRs retrieved successfully", zap.Int("count", len(prs)))
	return prs, nil
}

// Exists проверяет существование PR.
func (r *PullRequestRepository) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, existsPullRequestQuery, id).Scan(&exists)
	if err != nil {
		r.log.Error("Failed to check if PR exists", zap.String("id", id), zap.Error(err))
		return false, fmt.Errorf("failed to check existence: %w", err)
	}
	return exists, nil
}

func (r *PullRequestRepository) SetMerge(ctx context.Context, id string) error {
	log := r.log.With(zap.String("pr_id", id))
	log.Debug("Setting pull request status to MERGED")

	commandTag, err := r.pool.Exec(ctx, updatePullRequestStatusQuery, domain.StatusMerged, id)
	if err != nil {
		log.Error("Failed to execute update status query", zap.Error(err))
		return fmt.Errorf("failed to set merge status for PR %s: %w", id, err)
	}

	if commandTag.RowsAffected() == 0 {
		log.Warn("Pull request not found for setting merge status")
		return domain.ErrNotFound
	}

	log.Info("Successfully set pull request status to MERGED")
	return nil
}
