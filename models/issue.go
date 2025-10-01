package models

import (
	"go/token"
	"time"
)

// Issue represents a performance issue found in the code
type Issue struct {
	ID         string         `json:"id,omitempty"`
	File       string         `json:"file,omitempty"`
	Line       int            `json:"line,omitempty"`
	Column     int            `json:"column,omitempty"`
	Position   token.Position `json:"position"`
	Type       IssueType      `json:"type,omitempty"`
	Severity   SeverityLevel  `json:"severity,omitempty"`
	Message    string         `json:"message,omitempty"`
	Suggestion string         `json:"suggestion,omitempty"`
	Code       string         `json:"code,omitempty"`
	CanBeFixed bool           `json:"can_be_fixed,omitempty"`
	FixedAt    time.Time      `json:"fixed_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at,omitempty"`
	IgnoredAt  time.Time      `json:"ignored_at,omitempty"`
	IgnoreType IssueType      `json:"ignore_type,omitempty"`
	WhyBad     string         `json:"why_bad,omitempty"`
}
