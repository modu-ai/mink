package adapter

import (
	"context"
	"os"
	"sync/atomic"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/subagent"
)

// Compile-time assertion: ContextAdapter must implement SlashCommandContext.
// REQ-CMDCTX-001, AC-CMDCTX-001.
var _ command.SlashCommandContext = (*ContextAdapter)(nil)

// ContextAdapter implements command.SlashCommandContext by composing
// router (read-only), loop controller (write), and subagent plan-mode
// awareness (read-only).
//
// nil dependencies are tolerated per REQ-CMDCTX-014 and REQ-CMDCTX-015:
//   - nil registry: ResolveModelAlias returns ErrUnknownModel.
//   - nil loopCtrl: OnClear/OnCompactRequest/OnModelChange return ErrLoopControllerUnavailable;
//     SessionSnapshot returns a zero-value snapshot.
//
// @MX:ANCHOR: [AUTO] Concrete SlashCommandContext implementation.
// @MX:REASON: Single instance per CLI/daemon process; consumed by dispatcher, CLI, and tests — fan_in >= 3.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-001 REQ-CMDCTX-001
type ContextAdapter struct {
	registry *router.ProviderRegistry // may be nil → REQ-CMDCTX-014
	loopCtrl LoopController           // may be nil → REQ-CMDCTX-015
	aliasMap map[string]string        // optional, may be empty
	// planMode is a *atomic.Bool (pointer indirection) so that WithContext
	// children share the same underlying flag without copying the atomic value.
	// sync/atomic.Bool carries a noCopy guard; copying the value triggers
	// "go vet copylocks". Storing a pointer avoids the guard.
	// SetPlanMode on the parent is observed by all children atomically.
	planMode *atomic.Bool           // shared across WithContext children
	getwdFn  func() (string, error) // injectable for testing; defaults to os.Getwd
	logger   Logger                 // optional, may be nil
	// metrics is the optional metrics emission sink. nil sink triggers graceful
	// emission skip per REQ-CMDCTX-TEL-009. Shared via shallow copy in WithContext.
	// SPEC: SPEC-GOOSE-CMDCTX-TELEMETRY-001 REQ-CMDCTX-TEL-005
	metrics MetricsSink
	// ctxHook is set via WithContext for per-call plan-mode lookups.
	// It carries a TeammateIdentity when the caller is a subagent.
	// @MX:NOTE: [AUTO] WithContext returns a shallow copy sharing the planMode pointer.
	// All mutable shared state is accessed via atomic operations only.
	ctxHook context.Context
}

// Options is the constructor parameter bag for ContextAdapter.
type Options struct {
	// Registry is the provider registry for model resolution. May be nil.
	Registry *router.ProviderRegistry
	// LoopController is the query loop abstraction. May be nil.
	LoopController LoopController
	// AliasMap maps short alias strings to canonical "provider/model" strings.
	// Optional. REQ-CMDCTX-017.
	AliasMap map[string]string
	// GetwdFn overrides os.Getwd for testing. Defaults to os.Getwd if nil.
	GetwdFn func() (string, error)
	// Logger receives best-effort warnings (e.g., os.Getwd failures).
	// REQ-CMDCTX-018. May be nil.
	Logger Logger
	// Metrics is the optional metrics emission sink.
	// nil sink triggers graceful emission skip per REQ-CMDCTX-TEL-009.
	// SPEC: SPEC-GOOSE-CMDCTX-TELEMETRY-001 REQ-CMDCTX-TEL-008
	Metrics MetricsSink
}

// New constructs a ContextAdapter from the given options.
// nil dependencies are tolerated (graceful degradation per REQ-CMDCTX-014, REQ-CMDCTX-015).
// New always allocates a fresh *atomic.Bool for planMode so that WithContext
// children can share state with the parent.
// opts.Metrics is stored as-is; nil triggers emission skip per REQ-CMDCTX-TEL-009.
func New(opts Options) *ContextAdapter {
	getwdFn := opts.GetwdFn
	if getwdFn == nil {
		getwdFn = os.Getwd
	}
	return &ContextAdapter{
		registry: opts.Registry,
		loopCtrl: opts.LoopController,
		aliasMap: opts.AliasMap,
		planMode: new(atomic.Bool),
		getwdFn:  getwdFn,
		logger:   opts.Logger,
		metrics:  opts.Metrics,
	}
}

// SetPlanMode toggles the top-level orchestrator plan-mode flag.
// Because planMode is *atomic.Bool, all WithContext children observe the
// same flag value immediately (REQ-CMDCTX-005, REQ-CMDCTX-011).
func (a *ContextAdapter) SetPlanMode(active bool) {
	a.planMode.Store(active)
}

// WithContext returns a new ContextAdapter that uses the provided ctx for
// PlanModeActive lookups. The original adapter is not modified.
//
// The returned clone is a shallow copy: registry, loopCtrl, aliasMap, logger,
// getwdFn, and the *atomic.Bool planMode pointer are all shared. Only ctxHook
// differs. This is safe because the only mutable shared state (planMode) is
// accessed via atomic operations.
//
// @MX:NOTE: [AUTO] Shallow copy + atomic.Bool pointer sharing invariant.
// All WithContext children share the parent's planMode. SetPlanMode on parent
// is immediately visible to all children without additional synchronisation.
func (a *ContextAdapter) WithContext(ctx context.Context) *ContextAdapter {
	clone := *a
	clone.ctxHook = ctx
	return &clone
}

// OnClear implements SlashCommandContext.OnClear.
// It delegates to LoopController.RequestClear exactly once.
// Returns ErrLoopControllerUnavailable if loopCtrl is nil. REQ-CMDCTX-006, REQ-CMDCTX-015.
// Emits calls counter and duration histogram via instrumentErr. REQ-CMDCTX-TEL-001.
func (a *ContextAdapter) OnClear() error {
	_, err := instrumentErr(a, "OnClear", func() (struct{}, error) {
		if a.loopCtrl == nil {
			return struct{}{}, ErrLoopControllerUnavailable
		}
		ctx := a.effectiveCtx()
		return struct{}{}, a.loopCtrl.RequestClear(ctx)
	})
	return err
}

// OnCompactRequest implements SlashCommandContext.OnCompactRequest.
// It delegates to LoopController.RequestReactiveCompact exactly once.
// target == 0 means "use compactor default". REQ-CMDCTX-007, REQ-CMDCTX-015.
// Emits calls counter and duration histogram via instrumentErr. REQ-CMDCTX-TEL-001.
func (a *ContextAdapter) OnCompactRequest(target int) error {
	_, err := instrumentErr(a, "OnCompactRequest", func() (struct{}, error) {
		if a.loopCtrl == nil {
			return struct{}{}, ErrLoopControllerUnavailable
		}
		ctx := a.effectiveCtx()
		return struct{}{}, a.loopCtrl.RequestReactiveCompact(ctx, target)
	})
	return err
}

// OnModelChange implements SlashCommandContext.OnModelChange.
// It delegates to LoopController.RequestModelChange exactly once.
// The adapter does not re-validate info; ResolveModelAlias does that upstream.
// REQ-CMDCTX-008, REQ-CMDCTX-015.
// Emits calls counter and duration histogram via instrumentErr. REQ-CMDCTX-TEL-001.
func (a *ContextAdapter) OnModelChange(info command.ModelInfo) error {
	_, err := instrumentErr(a, "OnModelChange", func() (struct{}, error) {
		if a.loopCtrl == nil {
			return struct{}{}, ErrLoopControllerUnavailable
		}
		ctx := a.effectiveCtx()
		return struct{}{}, a.loopCtrl.RequestModelChange(ctx, info)
	})
	return err
}

// ResolveModelAlias implements SlashCommandContext.ResolveModelAlias.
// It looks up the alias in the alias table first, then falls back to
// ProviderRegistry SuggestedModels (strict mode). REQ-CMDCTX-002, REQ-CMDCTX-009, REQ-CMDCTX-014.
// Emits calls counter and duration histogram via instrumentErr. REQ-CMDCTX-TEL-001.
func (a *ContextAdapter) ResolveModelAlias(alias string) (*command.ModelInfo, error) {
	return instrumentErr(a, "ResolveModelAlias", func() (*command.ModelInfo, error) {
		return resolveAlias(a.registry, a.aliasMap, alias)
	})
}

// SessionSnapshot implements SlashCommandContext.SessionSnapshot.
// It combines LoopController.Snapshot() with os.Getwd(). If loopCtrl is nil,
// a zero-value snapshot is returned. If os.Getwd() fails, CWD is set to
// "<unknown>" and a warning is logged. REQ-CMDCTX-003, REQ-CMDCTX-010, REQ-CMDCTX-015, REQ-CMDCTX-018.
// Emits calls counter and duration histogram via instrumentVoid. REQ-CMDCTX-TEL-001.
func (a *ContextAdapter) SessionSnapshot() command.SessionSnapshot {
	return instrumentVoid(a, "SessionSnapshot", func() command.SessionSnapshot {
		var turnCount int
		var model string
		if a.loopCtrl != nil {
			snap := a.loopCtrl.Snapshot()
			turnCount = snap.TurnCount
			model = snap.Model
		} else {
			model = "<unknown>"
		}

		cwd, err := a.getwdFn()
		if err != nil {
			if a.logger != nil {
				a.logger.Warn("os.Getwd failed in SessionSnapshot", "error", err)
			}
			cwd = "<unknown>"
		}

		return command.SessionSnapshot{
			TurnCount: turnCount,
			Model:     model,
			CWD:       cwd,
		}
	})
}

// PlanModeActive implements SlashCommandContext.PlanModeActive.
// Returns true if (a) the adapter's internal atomic flag is set, or (b) the
// calling context carries a TeammateIdentity with PlanModeRequired == true.
// REQ-CMDCTX-004, REQ-CMDCTX-012.
// Emits calls counter and duration histogram via instrumentVoid. REQ-CMDCTX-TEL-001.
func (a *ContextAdapter) PlanModeActive() bool {
	return instrumentVoid(a, "PlanModeActive", func() bool {
		if a.planMode != nil && a.planMode.Load() {
			return true
		}
		if a.ctxHook != nil {
			id, ok := subagent.TeammateIdentityFromContext(a.ctxHook)
			if ok && id.PlanModeRequired {
				return true
			}
		}
		return false
	})
}

// effectiveCtx returns ctxHook if set, otherwise context.Background().
func (a *ContextAdapter) effectiveCtx() context.Context {
	if a.ctxHook != nil {
		return a.ctxHook
	}
	return context.Background()
}
