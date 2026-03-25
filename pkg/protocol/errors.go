// Copyright 2026 — see LICENSE file for terms.
// Package protocol defines shared types and error codes for aifr.
package protocol

import "fmt"

// Error codes returned by aifr operations.
const (
	ErrAccessDenied          = "ACCESS_DENIED"
	ErrAccessDeniedSensitive = "ACCESS_DENIED_SENSITIVE"
	ErrNotFound              = "NOT_FOUND"
	ErrIsDirectory           = "IS_DIRECTORY"
	ErrInvalidRef            = "INVALID_REF"
	ErrChunkOutOfRange       = "CHUNK_OUT_OF_RANGE"
	ErrStaleContinuation     = "STALE_CONTINUATION"
)

// Exit codes for the CLI.
const (
	ExitSuccess      = 0
	ExitError        = 1
	ExitAccessDenied = 2
	ExitSensitive    = 3
	ExitNotFound     = 4
	ExitInvalidArgs  = 10
)

// AifrError is a structured error carrying an error code.
type AifrError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

func (e *AifrError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Path)
	}
	return e.Code + ": " + e.Message
}

// NewError creates an AifrError with the given code and message.
func NewError(code, message string) *AifrError {
	return &AifrError{Code: code, Message: message}
}

// NewPathError creates an AifrError with the given code, message, and path.
func NewPathError(code, path, message string) *AifrError {
	return &AifrError{Code: code, Message: message, Path: path}
}

// ExitCodeForError returns the CLI exit code for an AifrError.
func ExitCodeForError(err error) int {
	if ae, ok := err.(*AifrError); ok {
		switch ae.Code {
		case ErrAccessDenied:
			return ExitAccessDenied
		case ErrAccessDeniedSensitive:
			return ExitSensitive
		case ErrNotFound:
			return ExitNotFound
		}
	}
	return ExitError
}
