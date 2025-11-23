package dto

import (
	"time"

	"avito/internal/domain"

	"github.com/google/uuid"
)

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type UserRequest struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	IsActive bool      `json:"is_active"`
}

type CreateTeamDTO struct {
	Name    string        `json:"team_name"`
	Members []UserRequest `json:"members"`
}

type SetUserActiveStatusRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type PullRequestShort struct {
	PullRequestID   string          `json:"pull_request_id"`
	PullRequestName string          `json:"pull_request_name"`
	AuthorID        uuid.UUID       `json:"author_id"`
	Status          domain.StatusPR `json:"status"`
}

type ReviewUserResponse struct {
	UserID       string              `json:"user_id"`
	PullRequests []*PullRequestShort `json:"pull_requests"`
}

type CreatePullRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

type PullRequestResponse struct {
	PullRequestID     string      `json:"pull_request_id"`
	PullRequestName   string      `json:"pull_request_name"`
	Status            string      `json:"status"`
	AuthorID          uuid.UUID   `json:"author_id"`
	AssignedReviewers []uuid.UUID `json:"assigned_reviewers"`
	CreatedAt         time.Time   `json:"created_at"`
	MergedAt          *time.Time  `json:"merged_at,omitempty"`
}

type SetMergeRequest struct {
	PullRequestID string `json:"pull_request_id"`
}

type ReassignRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}

type ReassignResponse struct {
	PullRequest PullRequestResponse `json:"pr"`
	ReplacedBy  string              `json:"replaced_by"`
}

type UserStatDTO struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	IsActive    bool   `json:"is_active"`
	ReviewCount int    `json:"review_assignments_count"`
}

type StatsResponseDTO struct {
	Stats []UserStatDTO `json:"stats"`
}

// FromUserReviewStats преобразует срез доменных моделей в DTO для ответа.
func FromUserReviewStats(stats []*domain.UserReviewStat) StatsResponseDTO {
	statsDTO := make([]UserStatDTO, 0, len(stats))
	for _, stat := range stats {
		statsDTO = append(statsDTO, UserStatDTO{
			UserID:      stat.UserID.String(),
			Username:    stat.Username,
			IsActive:    stat.IsActive,
			ReviewCount: stat.ReviewCount,
		})
	}
	return StatsResponseDTO{Stats: statsDTO}
}
func FromUserDomain(user *domain.User) UserRequest {
	return UserRequest{
		UserID:   user.ID,
		Username: user.Username,
		IsActive: user.IsActive,
	}
}
func ToTeamDomain(tr CreateTeamDTO) domain.Team {
	members := make([]domain.User, 0, len(tr.Members))
	for _, member := range tr.Members {
		members = append(members, domain.User{
			ID:       member.UserID,
			Username: member.Username,
			IsActive: member.IsActive,
		})
	}
	return domain.Team{
		Name:    tr.Name,
		Members: members,
	}
}
func FromTeamDomain(team domain.Team) *CreateTeamDTO {
	members := make([]UserRequest, 0, len(team.Members))
	for _, member := range team.Members {
		members = append(members, UserRequest{
			UserID:   member.ID,
			Username: member.Username,
			IsActive: member.IsActive,
		})
	}
	return &CreateTeamDTO{
		Name:    team.Name,
		Members: members,
	}
}
func ToReviewUserResponse(pr []*domain.PullRequest, userID uuid.UUID) ReviewUserResponse {
	var prs []*PullRequestShort
	for _, pullRequest := range pr {
		prs = append(prs, &PullRequestShort{
			PullRequestID:   pullRequest.ID,
			PullRequestName: pullRequest.Name,
			AuthorID:        pullRequest.AuthorID,
			Status:          pullRequest.Status,
		})
	}
	return ReviewUserResponse{
		UserID:       userID.String(),
		PullRequests: prs,
	}
}
func ToPullRequestResponse(pr *domain.PullRequest) *PullRequestResponse {
	if pr == nil {
		return nil
	}
	response := &PullRequestResponse{
		PullRequestID:     pr.ID,
		PullRequestName:   pr.Name,
		Status:            string(pr.Status),
		AuthorID:          pr.AuthorID,
		AssignedReviewers: pr.AssignedReviewers,
		CreatedAt:         pr.CreatedAt,
	}
	if pr.MergedAt != nil {
		response.MergedAt = pr.MergedAt
	}
	return response
}
func ToReassignResponse(pr *domain.PullRequest, userID string) ReassignResponse {
	return ReassignResponse{
		PullRequest: *ToPullRequestResponse(pr),
		ReplacedBy:  userID,
	}
}
