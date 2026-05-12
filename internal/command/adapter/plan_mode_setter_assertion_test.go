package adapter_test

import (
	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/command/adapter"
)

// Compile-time assertion: *ContextAdapter satisfies command.PlanModeSetter.
// REQ-PMC-002, AC-PMC-002.
var _ command.PlanModeSetter = (*adapter.ContextAdapter)(nil)
