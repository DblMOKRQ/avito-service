package handler

import (
	"errors"
	"go.uber.org/zap"
	"net/http"

	"avito/internal/domain"
	"avito/internal/service"
	"avito/internal/transport/http/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	codeInternalError       = "INTERNAL_ERROR"
	codeInvalidBody         = "INVALID_BODY"
	codeParametersIncorrect = "PARAMETERS_INCORRECT"
	codeTeamExists          = "TEAM_EXISTS"
	codeNotFound            = "NOT_FOUND"
	codePRExists            = "PR_EXISTS"
	codePRMerged            = "PR_MERGED"
	codeNoCandidate         = "NO_CANDIDATE"
	codeNotAssigned         = "NOT_ASSIGNED"
	codeAuthorInactive      = "AUTHOR_INACTIVE"
)

type Handler struct {
	teamService  service.TeamService
	userService  service.UserService
	statsService service.StatsService
	prService    service.PullRequestService
}

func NewHandler(teamService service.TeamService, userService service.UserService, statsService service.StatsService, prService service.PullRequestService) *Handler {
	return &Handler{
		teamService:  teamService,
		userService:  userService,
		statsService: statsService,
		prService:    prService,
	}
}

func (h *Handler) CreateTeam(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	var req dto.CreateTeamDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("Failed to decode request body", zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid request body")
		return
	}
	team, err := h.teamService.CreateTeamWithMembers(c.Request.Context(), dto.ToTeamDomain(req))
	if err != nil {
		if errors.Is(err, domain.ErrOneOfParametersNil) {
			log.Warn("One of the parameters is nil", zap.Error(err))
			h.responseError(c, http.StatusBadRequest, codeParametersIncorrect, "one of the parameters is incorrect")
			return
		}
		if errors.Is(err, domain.ErrTeamExists) {
			log.Warn("Team already exists", zap.Error(err))
			h.responseError(c, http.StatusConflict, codeTeamExists, "team already exists")
			return
		}
		log.Error("Failed to create team", zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "failed to create team")
		return
	}
	c.JSON(http.StatusCreated, dto.FromTeamDomain(*team))
}

func (h *Handler) GetTeam(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	teamName := c.Query("team_name")
	if teamName == "" {
		log.Warn("team_name query parameter is missing")
		h.responseError(c, http.StatusBadRequest, codeParametersIncorrect, "team_name query parameter is required")
		return
	}
	team, err := h.teamService.GetTeamByName(c.Request.Context(), teamName)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("Team not found", zap.String("team_name", teamName))
			h.responseError(c, http.StatusNotFound, codeNotFound, "team not found")
			return
		}
		log.Error("Failed to get team", zap.String("team_name", teamName), zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "failed to get team")
		return
	}

	c.JSON(http.StatusOK, dto.FromTeamDomain(*team))
}

func (h *Handler) SetUserActiveStatus(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	var req dto.SetUserActiveStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("Failed to decode request body", zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid request body")
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		log.Warn("Failed to parse user ID", zap.String("user_id", req.UserID), zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid user ID")
		return
	}
	user, err := h.userService.SetIsActive(c.Request.Context(), userID, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrOneOfParametersNil) {
			log.Warn("One of the parameters is nil", zap.Error(err))
			h.responseError(c, http.StatusBadRequest, codeParametersIncorrect, "one of the parameters is incorrect")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("User not found", zap.String("user_id", req.UserID))
			h.responseError(c, http.StatusNotFound, codeNotFound, "user not found")
			return
		}
		log.Error("Failed to set user active status", zap.String("user_id", req.UserID), zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "failed to set user active status")
		return
	}

	c.JSON(http.StatusOK, dto.FromUserDomain(user))
}

func (h *Handler) GetUserReview(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	userIDStr := c.Query("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Warn("Invalid user_id query parameter", zap.String("user_id", userIDStr), zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid user_id query parameter")
		return
	}

	reviews, err := h.userService.GetReviewsForUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("user not found", zap.String("user_id", userID.String()))
			h.responseError(c, http.StatusNotFound, codeNotFound, "user not found")
			return
		}
		log.Error("Failed to get reviews for user", zap.String("user_id", userID.String()), zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "failed to get reviews for user")
		return
	}

	//TODO: возвращать json объект с нужной структурой
	c.JSON(http.StatusOK, dto.ToReviewUserResponse(reviews, userID))
}

func (h *Handler) CreatePR(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	var req dto.CreatePullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("Failed to decode request body", zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid request body")
		return
	}
	authorID, err := uuid.Parse(req.AuthorID)
	if err != nil {
		log.Warn("Invalid author ID query parameter", zap.String("author_id", req.AuthorID), zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid author ID query parameter")
		return
	}

	pullRequest, err := h.prService.CreatePR(c.Request.Context(), req.PullRequestID, req.PullRequestName, authorID)
	if err != nil {
		if errors.Is(err, domain.ErrPRExists) {
			log.Warn("Pull request already exists")
			h.responseError(c, http.StatusConflict, codePRExists, "PR id already exists")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("Author not found", zap.String("author_id", req.AuthorID))
			h.responseError(c, http.StatusNotFound, codeNotFound, "author not found")
			return
		}
		if errors.Is(err, domain.ErrAuthorIsInactive) {
			log.Warn("Author is inactive", zap.String("author_id", req.AuthorID))
			h.responseError(c, http.StatusForbidden, codeAuthorInactive, "author is inactive and cannot create pull requests")
			return
		}
		log.Error("Failed to create pull request", zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "failed to create pull request")
		return
	}
	c.JSON(http.StatusCreated, dto.ToPullRequestResponse(pullRequest))

}

func (h *Handler) SetMerge(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	var req dto.SetMergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("Failed to decode request body", zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid request body")
		return
	}
	pr, err := h.prService.SetMerge(c.Request.Context(), req.PullRequestID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("Pull request not found", zap.String("pull_request_id", req.PullRequestID))
			h.responseError(c, http.StatusNotFound, codeNotFound, "pull request not found")
			return
		}
		log.Error("Failed to set merge pull request", zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "failed to set merge pull request")
		return
	}
	c.JSON(http.StatusOK, dto.ToPullRequestResponse(pr))
}

func (h *Handler) Reassign(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	var req dto.ReassignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("Failed to decode request body", zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid request body")
		return
	}
	oldUserID, err := uuid.Parse(req.OldUserID)
	if err != nil {
		log.Warn("Failed to parse old user id", zap.String("old_user_id", req.OldUserID), zap.Error(err))
		h.responseError(c, http.StatusBadRequest, codeInvalidBody, "invalid old user id")
	}
	pr, newUserID, err := h.prService.ReassignmentReviewers(c.Request.Context(), req.PullRequestID, oldUserID)
	if err != nil {
		if errors.Is(err, domain.ErrPRNotExist) {
			log.Warn("Pull request not found", zap.String("pull_request_id", req.PullRequestID))
			h.responseError(c, http.StatusNotFound, codeNotFound, "pull request not found")
			return
		}
		if errors.Is(err, domain.ErrPRMerged) {
			log.Warn("Pull request is merged", zap.Error(err))
			h.responseError(c, http.StatusConflict, codePRMerged, "cannot reassign on merged PR")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("Author not found on merged PR", zap.String("pull_request_id", req.PullRequestID))
			h.responseError(c, http.StatusNotFound, codeNotFound, "author pull request not found")
			return
		}
		if errors.Is(err, domain.ErrNoCandidate) {
			log.Warn("Candidate not found on merged PR", zap.String("pull_request_id", req.PullRequestID))
			h.responseError(c, http.StatusConflict, codeNoCandidate, "no active replacement candidate in team")
			return
		}
		if errors.Is(err, domain.ErrUserNotAssigned) {
			log.Warn("The user was not assigned as a reviewer", zap.String("pull_request_id", req.PullRequestID), zap.String("id", req.OldUserID))
			h.responseError(c, http.StatusConflict, codeNotAssigned, "reviewer is not assigned to this PR")
			return
		}
		log.Error("Failed to reassign pull request", zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "failed to reassign pull request")
		return
	}

	c.JSON(http.StatusOK, dto.ToReassignResponse(pr, newUserID))
}

func (h *Handler) GetStats(c *gin.Context) {
	log := c.MustGet("logger").(*zap.Logger)
	log.Info("Handling get statistics request")

	stats, err := h.statsService.GetUserReviewStats(c.Request.Context())
	if err != nil {
		log.Error("Failed to get stats from service", zap.Error(err))
		h.responseError(c, http.StatusInternalServerError, codeInternalError, "could not retrieve statistics")
		return
	}

	response := dto.FromUserReviewStats(stats)
	c.JSON(http.StatusOK, response)
}

func (h *Handler) responseError(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{
		Error: dto.ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}
