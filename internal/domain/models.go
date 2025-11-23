package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound означает, что какой-то элемент не был найден в базе данных
	ErrNotFound = errors.New("not found")

	// ErrOneOfParametersNil означает, что один из обязательных параметров,
	// переданных в метод сервиса, был пустым
	ErrOneOfParametersNil = errors.New("one of parameters is nil")
	ErrUserIDNil          = errors.New("userID is nil")
	ErrTeamExists         = errors.New("team already exists")
	ErrPRExists           = errors.New("pull request already exists")
	ErrPRNotExist         = errors.New("pull request does not exist")
	ErrAuthorCannotDelete = errors.New("author cannot be deleted")
	ErrNoCandidate        = errors.New("no active replacement candidate in team")
	ErrPRMerged           = errors.New("cannot reassign on a merged PR")
	ErrUserNotAssigned    = errors.New("user to be reassigned is not currently a reviewer")
	ErrAuthorIsInactive   = errors.New("author is inactive and cannot create pull requests")
)

type StatusPR string

const (
	StatusOpen   StatusPR = "OPEN"
	StatusMerged StatusPR = "MERGED"
)

type User struct {
	ID       uuid.UUID
	Username string
	IsActive bool
	TeamName string
}

type Team struct {
	Name    string
	Members []User
}

type PullRequest struct {
	ID                string
	Name              string
	Status            StatusPR
	AuthorID          uuid.UUID
	AssignedReviewers []uuid.UUID
	CreatedAt         time.Time
	MergedAt          *time.Time
}

type Reassignment struct {
	PullRequestID string
	OldUserID     uuid.UUID
	NewUserID     uuid.UUID
}

type UserReviewStat struct {
	UserID      uuid.UUID
	Username    string
	IsActive    bool
	ReviewCount int
}
