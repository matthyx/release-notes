package gh

import (
	"errors"
	"time"
)

// Release describes the metadata for a single GitHub release.
type Release struct {
	TagName   string    `json:"tag_name"`
	Name      string    `json:"name"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// PR describes a pull request as returned by the GitHub REST API.
type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	HTMLURL  string    `json:"html_url"`
	MergedAt time.Time `json:"merged_at"`
}

// CompareResult is the slice of commits returned by /compare.
type CompareResult struct {
	Commits []Commit `json:"commits"`
}

// Commit is a single commit entry in a compare response.
type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

// content is the GitHub /contents response shape.
type content struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// Sentinel errors. Callers can errors.Is against these.
var (
	ErrNotFound       = errors.New("not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrRateLimited    = errors.New("rate limited")
	ErrNoPredecessor  = errors.New("no predecessor release")
	ErrInvalidContent = errors.New("invalid content")
)
