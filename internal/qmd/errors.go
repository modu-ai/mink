package qmd

import "errors"

// QMD-specific error types.
// @MX:NOTE: Error types enable precise error handling and testing

// ErrIndexNotFound is returned when the search index does not exist.
var ErrIndexNotFound = errors.New("qmd: index not found")

// ErrModelNotAvailable is returned when a required GGUF model is missing.
var ErrModelNotAvailable = errors.New("qmd: model not available")

// ErrQueryTooShort is returned when the query text is too short or invalid.
var ErrQueryTooShort = errors.New("qmd: query too short")

// ErrInvalidDocument is returned when a document fails validation.
var ErrInvalidDocument = errors.New("qmd: invalid document")

// ErrIndexPathInvalid is returned when the index path is invalid.
var ErrIndexPathInvalid = errors.New("qmd: invalid index path")

// ErrQMDDisabled is returned when QMD is disabled via configuration.
var ErrQMDDisabled = errors.New("qmd: QMD is disabled")

// ErrModelNotReady is returned when models are still downloading.
var ErrModelNotReady = errors.New("qmd: model not ready")

// ErrMCPNetworkBindProhibited is returned when MCP server tries to bind to network.
var ErrMCPNetworkBindProhibited = errors.New("qmd: MCP network bind prohibited")
