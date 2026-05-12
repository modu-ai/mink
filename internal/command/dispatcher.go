// Package command implements the slash command system for MINK.
// SPEC: SPEC-GOOSE-COMMAND-001
package command

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/command/parser"
	"github.com/modu-ai/mink/internal/message"
	"go.uber.org/zap"
)

const defaultMaxExpandedPromptBytes int64 = 64 * 1024 // 64 KiB

// Config holds Dispatcher configuration.
type Config struct {
	// MaxExpandedPromptBytes is the maximum size of an expanded custom command prompt.
	// Zero uses the default (64 KiB). REQ-CMD-014.
	MaxExpandedPromptBytes int64
	// CustomCommandRoots lists additional filesystem paths scanned for custom commands.
	// REQ-CMD-017.
	CustomCommandRoots []string
}

// ProcessedKind describes the action the caller should take after ProcessUserInput.
type ProcessedKind int

const (
	// ProcessProceed means the caller should forward the prompt to QUERY-001.
	ProcessProceed ProcessedKind = iota
	// ProcessLocal means the caller should yield the Messages to the output stream
	// without making an LLM API call.
	ProcessLocal
	// ProcessExit means the CLI should terminate with ExitCode.
	ProcessExit
	// ProcessAbort means the user cancelled the operation.
	ProcessAbort
)

// ProcessedInput carries the result of ProcessUserInput.
type ProcessedInput struct {
	// Kind selects the caller's action branch.
	Kind ProcessedKind
	// Prompt is the (potentially expanded) prompt for ProcessProceed.
	Prompt string
	// Messages holds pre-built SDK messages for ProcessLocal.
	Messages []message.SDKMessage
	// ExitCode is the process exit code for ProcessExit.
	ExitCode int
}

// Dispatcher orchestrates slash command parsing, resolution, and execution.
//
// @MX:ANCHOR: [AUTO] Primary entry point for user input processing before QUERY-001.
// @MX:REASON: Called by QUERY-001 submitMessage on every user input line.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-004, REQ-CMD-005
type Dispatcher struct {
	registry *Registry
	cfg      Config
	logger   *zap.Logger
}

// NewDispatcher creates a Dispatcher backed by the provided Registry.
func NewDispatcher(reg *Registry, cfg Config, logger *zap.Logger) *Dispatcher {
	if cfg.MaxExpandedPromptBytes == 0 {
		cfg.MaxExpandedPromptBytes = defaultMaxExpandedPromptBytes
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Dispatcher{registry: reg, cfg: cfg, logger: logger}
}

// ProcessUserInput examines a single user input line and either dispatches a slash
// command or returns the prompt unchanged.
//
// Processing order (REQ-CMD-004):
//  1. Parse: if not a slash command, return ProcessProceed unchanged.
//  2. Resolve: if command not found, return ProcessLocal with informative message.
//  3. Plan-mode check: if mutating command in plan mode, return ProcessLocal (REQ-CMD-011).
//  4. Execute: route Result by Kind.
//  5. PromptExpansion size check (REQ-CMD-014).
//
// @MX:ANCHOR: [AUTO] Core dispatch loop; all slash command execution flows through here.
// @MX:REASON: Fan-in >= 3: QUERY-001, builtin tests, integration tests, dispatcher tests.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-004, REQ-CMD-005, REQ-CMD-011, REQ-CMD-014
func (d *Dispatcher) ProcessUserInput(
	ctx context.Context,
	input string,
	sctx SlashCommandContext,
) (ProcessedInput, error) {
	// Step 1: Parse.
	name, rawArgs, ok := parser.Parse(input)
	if !ok {
		return ProcessedInput{Kind: ProcessProceed, Prompt: input}, nil
	}

	// Step 2: Resolve.
	cmd, found := d.registry.Resolve(name)
	if !found {
		d.logger.Info("unknown slash command", zap.String("name", name))
		msg := fmt.Sprintf("unknown command: /%s. Type /help to list.", name)
		return ProcessedInput{
			Kind:     ProcessLocal,
			Messages: []message.SDKMessage{newSystemMessage(msg)},
		}, nil
	}

	// Step 3: Plan-mode check.
	// @MX:NOTE: [AUTO] Mutates=true 명령은 plan mode에서 차단됨. REQ-CMD-011: 변이형 명령은
	// QUERY-001이 plan mode로 동작 중일 때 사용자에게 LocalReply로 거부 메시지를 반환해야 한다.
	// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-011
	if cmd.Metadata().Mutates && sctx != nil && sctx.PlanModeActive() {
		msg := fmt.Sprintf("command '%s' disabled in plan mode", name)
		return ProcessedInput{
			Kind:     ProcessLocal,
			Messages: []message.SDKMessage{newSystemMessage(msg)},
		}, nil
	}

	// Build Args for the command.
	splitArgs, _ := parser.SplitArgs(rawArgs)
	parsedArgs := Args{
		RawArgs:      splitArgs.RawArgs,
		Positional:   splitArgs.Positional,
		Flags:        splitArgs.Flags,
		OriginalLine: input,
	}

	// Inject the SlashCommandContext via the context value.
	// Use the exported key from the builtin package so builtins can retrieve it.
	execCtx := ctx
	if sctx != nil {
		execCtx = context.WithValue(ctx, sctxInjectionKey{}, sctx)
	}

	// Step 4: Execute.
	result, err := cmd.Execute(execCtx, parsedArgs)
	if err != nil {
		return ProcessedInput{}, fmt.Errorf("execute /%s: %w", name, err)
	}

	// Step 5: Route by result kind.
	switch result.Kind {
	case ResultLocalReply:
		return ProcessedInput{
			Kind:     ProcessLocal,
			Messages: []message.SDKMessage{newSystemMessage(result.Text)},
		}, nil

	case ResultPromptExpansion:
		// Size guard. REQ-CMD-014.
		if int64(len(result.Prompt)) > d.cfg.MaxExpandedPromptBytes {
			msg := fmt.Sprintf("expanded prompt exceeds size limit (%d bytes)", len(result.Prompt))
			return ProcessedInput{
				Kind:     ProcessLocal,
				Messages: []message.SDKMessage{newSystemMessage(msg)},
			}, nil
		}
		return ProcessedInput{Kind: ProcessProceed, Prompt: result.Prompt}, nil

	case ResultExit:
		return ProcessedInput{Kind: ProcessExit, ExitCode: result.Exit}, nil

	case ResultAbort:
		return ProcessedInput{Kind: ProcessAbort}, nil

	default:
		return ProcessedInput{Kind: ProcessProceed, Prompt: input}, nil
	}
}

// sctxInjectionKey is the context key for the SlashCommandContext passed to Execute.
// It is unexported to prevent external callers from bypassing the Dispatcher.
// Builtin commands must use the same key — see SctxContextKey().
type sctxInjectionKey struct{}

// SctxContextKey returns the context key that the Dispatcher uses to inject the
// SlashCommandContext. Builtin command Execute methods must use this key to
// retrieve the context.
func SctxContextKey() interface{} { return sctxInjectionKey{} }

// newSystemMessage constructs an SDKMessage that carries a system-level text payload.
// The Dispatcher uses this to build local reply messages.
func newSystemMessage(text string) message.SDKMessage {
	return message.SDKMessage{
		Type:    message.SDKMsgMessage,
		Payload: text,
	}
}
