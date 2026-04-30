// Package redact implements the PII redaction chain for trajectory entries.
// Rules are applied in declaration order; panics in individual rules are
// caught and replaced with a "<REDACT_FAILED>" sentinel (REQ-TRAJECTORY-014).
package redact

import (
	"fmt"
	"regexp"

	"go.uber.org/zap"
)

// Entry mirrors trajectory.TrajectoryEntry to avoid a circular import.
// Callers cast to this type before passing to Apply.
type Entry struct {
	From  string // "system" | "human" | "gpt" | "tool"
	Value string
}

// Rule is a single PII replacement rule.
type Rule struct {
	// Name is a human-readable identifier used in log messages.
	Name string

	// Pattern is the compiled regex. Used when ApplyFn is nil.
	Pattern *regexp.Regexp

	// Replacement is the substitution string (e.g. "<REDACTED:email>").
	// $1, $2 back-references are supported via regexp.ReplaceAllString.
	Replacement string

	// AppliesToSystem controls whether this rule runs on "from":"system" entries.
	// Default false preserves system prompts verbatim (REQ-TRAJECTORY-016).
	AppliesToSystem bool

	// ApplyFn is an optional custom apply function. When non-nil, it overrides
	// the Pattern/Replacement pair. Used primarily in tests (AC-TRAJECTORY-015).
	ApplyFn func(value string) string
}

// apply returns the redacted string for the given input.
// Panics inside this function are caught by Chain.Apply.
func (r Rule) apply(value string) string {
	if r.ApplyFn != nil {
		return r.ApplyFn(value)
	}
	if r.Pattern == nil {
		return value
	}
	return r.Pattern.ReplaceAllString(value, r.Replacement)
}

// Chain holds an ordered list of rules to apply sequentially.
type Chain struct {
	rules  []Rule
	logger *zap.Logger
}

// NewChain creates a Chain from the given rules, applying built-ins last.
// userRules are applied first per REQ-TRAJECTORY-017 (user-defined take priority).
func NewChain(userRules []Rule, logger *zap.Logger) Chain {
	all := make([]Rule, 0, len(userRules)+len(BuiltinRules()))
	all = append(all, userRules...)
	all = append(all, BuiltinRules()...)
	return Chain{rules: all, logger: logger}
}

// NewBuiltinChain creates a Chain with only the 6 built-in rules.
func NewBuiltinChain(logger *zap.Logger) Chain {
	return Chain{rules: BuiltinRules(), logger: logger}
}

// NewChainWithRules creates a Chain from explicit user rules (without appending built-ins).
// Used primarily in tests to inject controlled rule sets.
func NewChainWithRules(rules []Rule, _ []Rule, logger *zap.Logger) Chain {
	return Chain{rules: rules, logger: logger}
}

// Apply runs all rules against entry.Value in declaration order.
// If a rule panics, the entry value is replaced with "<REDACT_FAILED>" and
// processing continues with the next entry call (REQ-TRAJECTORY-014).
// System-role entries are skipped for rules where AppliesToSystem==false
// (REQ-TRAJECTORY-016).
func (c *Chain) Apply(entry *Entry) {
	if entry == nil {
		return
	}

	isSystem := entry.From == "system"

	for _, rule := range c.rules {
		if isSystem && !rule.AppliesToSystem {
			continue
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					entry.Value = "<REDACT_FAILED>"
					if c.logger != nil {
						c.logger.Error("redact rule panicked",
							zap.String("rule", rule.Name),
							zap.String("panic", fmt.Sprintf("%v", r)),
						)
					}
				}
			}()
			entry.Value = rule.apply(entry.Value)
		}()
	}
}
