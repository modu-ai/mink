// Package command implements the slash command system for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package command

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// validNameRe validates that a command name contains only [a-z0-9_-].
var validNameRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

// customLoader is the function used to load custom commands during Reload.
// It is set by an init function in the custom package via command.SetCustomLoader.
// This indirection breaks the import cycle between the command and custom packages.
// REQ-CMD-010.
var customLoader func(root string, src Source, logger *zap.Logger) ([]Command, error)

// SetCustomLoader registers the custom command loading function.
// It must be called by the custom package's init() before any Reload occurs.
//
// @MX:ANCHOR: [AUTO] import cycle 브릿지: command → custom은 순환 의존 발생, custom.init()이 이 함수를 통해 역방향 주입.
// @MX:REASON: Fan-in >= 3: custom.init(), Reload 호출 경로, 통합 테스트 setup.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-010
func SetCustomLoader(fn func(root string, src Source, logger *zap.Logger) ([]Command, error)) {
	customLoader = fn
}

// commandSnapshot holds an immutable point-in-time view of custom commands.
// The snapshot is atomically swapped by Reload. REQ-CMD-010, REQ-CMD-012.
type commandSnapshot struct {
	project map[string]Command // SourceCustomProject commands
	user    map[string]Command // SourceCustomUser commands
}

// Registry maps command names to Command implementations and enforces resolution
// precedence: builtin > custom-project > custom-user > skill-provided.
//
// @MX:ANCHOR: [AUTO] Core command lookup boundary; called by Dispatcher on every slash command.
// @MX:REASON: Fan-in >= 3: Dispatcher, builtin.Register, custom loader, test helpers.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-002
type Registry struct {
	logger *zap.Logger

	// builtin holds commands registered with SourceBuiltin; never swapped.
	builtinMu sync.RWMutex
	builtin   map[string]Command
	// aliases maps alias names to canonical builtin names (bypasses name validation).
	aliases map[string]string

	// snapshot holds the atomically-swapped custom command sets.
	snapshot atomic.Pointer[commandSnapshot]

	// providers holds skill-backed command providers.
	providerMu sync.RWMutex
	providers  []Provider

	// customRoots are the filesystem roots scanned by Reload.
	customRoots []string
}

// Provider supplies commands from an external source (e.g. SKILLS-001).
type Provider interface {
	Commands() []Command
}

// Option configures a Registry during construction.
type Option func(*Registry)

// WithLogger sets the structured logger.
func WithLogger(l *zap.Logger) Option {
	return func(r *Registry) { r.logger = l }
}

// WithCustomRoots sets the filesystem paths scanned during Reload.
func WithCustomRoots(roots ...string) Option {
	return func(r *Registry) { r.customRoots = roots }
}

// WithProvider pre-registers a command provider.
func WithProvider(p Provider) Option {
	return func(r *Registry) { r.providers = append(r.providers, p) }
}

// NewRegistry creates an empty Registry with the supplied options.
func NewRegistry(opts ...Option) (*Registry, error) {
	r := &Registry{
		logger:  zap.NewNop(),
		builtin: make(map[string]Command),
		aliases: make(map[string]string),
	}
	for _, o := range opts {
		o(r)
	}
	// Initialise snapshot with empty custom sets.
	r.snapshot.Store(&commandSnapshot{
		project: make(map[string]Command),
		user:    make(map[string]Command),
	})
	return r, nil
}

// Register adds a command to the registry at the specified source tier.
// Returns ErrInvalidCommandName if the lowercased name fails [a-z0-9_-] validation.
// When a name collision occurs the higher-precedence entry wins and a WARN is logged.
// Cross-tier shadowing is also warned: registering at a lower-priority tier when a
// higher-priority tier already holds the same name emits a WARN. REQ-CMD-002, REQ-CMD-003,
// AC-CMD-011.
func (r *Registry) Register(c Command, src Source) error {
	name := strings.ToLower(c.Name())
	if !validNameRe.MatchString(name) {
		return ErrInvalidCommandName
	}

	// Check whether any higher-priority tier (Source < src) already holds this name.
	// If so, the new command will be recorded but can never win resolution.
	// REQ-CMD-002, AC-CMD-011.
	r.warnCrossTierShadow(name, src, c)

	switch src {
	case SourceBuiltin:
		r.builtinMu.Lock()
		defer r.builtinMu.Unlock()
		if existing, ok := r.builtin[name]; ok {
			r.logger.Warn("builtin command shadowed",
				zap.String("name", name),
				zap.String("existing_source", sourceString(existing.Metadata().Source)),
			)
		}
		r.builtin[name] = c

	case SourceCustomProject:
		snap := r.snapshot.Load()
		newProject := copyMap(snap.project)
		if existing, ok := newProject[name]; ok {
			r.logger.Warn("custom-project command shadowed",
				zap.String("name", name),
				zap.String("shadowed_file", existing.Metadata().FilePath),
			)
		}
		newProject[name] = c
		r.snapshot.Store(&commandSnapshot{project: newProject, user: snap.user})

	case SourceCustomUser:
		snap := r.snapshot.Load()
		newUser := copyMap(snap.user)
		if existing, ok := newUser[name]; ok {
			r.logger.Warn("custom-user command shadowed",
				zap.String("name", name),
				zap.String("shadowed_file", existing.Metadata().FilePath),
			)
		}
		newUser[name] = c
		r.snapshot.Store(&commandSnapshot{project: snap.project, user: newUser})
	}

	return nil
}

// warnCrossTierShadow emits a WARN when a command at src is being registered but a
// higher-priority tier (Source < src) already holds the same name.
// The lower-priority command is still recorded; it simply can never win Resolve.
// AC-CMD-011.
//
// @MX:NOTE: [AUTO] 우선순위 체크 순서: builtin(0) > custom-project(1) > custom-user(2) > skill(3).
// 낮은 Source 정수값 = 높은 우선순위. 동일 이름 등록 시 상위 tier가 항상 Resolve에서 승리.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-002, AC-CMD-011
func (r *Registry) warnCrossTierShadow(name string, src Source, incoming Command) {
	if src == SourceBuiltin {
		// Builtin is the highest priority — nothing can shadow it from above.
		return
	}

	// Check builtin tier (always higher priority than any non-builtin).
	r.builtinMu.RLock()
	existing, ok := r.builtin[name]
	r.builtinMu.RUnlock()
	if ok {
		r.logger.Warn("command shadowed",
			zap.String("name", name),
			zap.String("shadowed_source", existing.Metadata().FilePath),
			zap.String("shadowed_by", incoming.Metadata().FilePath),
		)
		return
	}

	// Check custom-project tier when registering at a lower tier.
	if src > SourceCustomProject {
		snap := r.snapshot.Load()
		if existing, ok := snap.project[name]; ok {
			r.logger.Warn("command shadowed",
				zap.String("name", name),
				zap.String("shadowed_source", existing.Metadata().FilePath),
				zap.String("shadowed_by", incoming.Metadata().FilePath),
			)
			return
		}
	}

	// Check custom-user tier when registering at a lower tier (SourceSkill).
	if src > SourceCustomUser {
		snap := r.snapshot.Load()
		if existing, ok := snap.user[name]; ok {
			r.logger.Warn("command shadowed",
				zap.String("name", name),
				zap.String("shadowed_source", existing.Metadata().FilePath),
				zap.String("shadowed_by", incoming.Metadata().FilePath),
			)
			return
		}
	}
}

// RegisterAlias records name as an alias pointing to the canonical builtin command.
// Aliases bypass the strict name-validation regex (supporting e.g. "?" → "help").
func (r *Registry) RegisterAlias(alias, canonical string) {
	r.builtinMu.Lock()
	defer r.builtinMu.Unlock()
	r.aliases[alias] = canonical
}

// RegisterProvider appends a skill-backed command provider.
// REQ-CMD-018.
func (r *Registry) RegisterProvider(p Provider) {
	r.providerMu.Lock()
	defer r.providerMu.Unlock()
	r.providers = append(r.providers, p)
}

// Resolve looks up a command by its lowercased name, respecting the precedence:
//
//	builtin > custom-project > custom-user > skill-provided
//
// Returns (nil, false) when no command is found.
// REQ-CMD-002, REQ-CMD-012 (snapshot served atomically).
//
// @MX:ANCHOR: [AUTO] Single resolution path for all slash command lookups.
// @MX:REASON: Fan-in >= 5: Dispatcher, help command, builtin test, integration test, registry test.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-002
func (r *Registry) Resolve(name string) (Command, bool) {
	lower := strings.ToLower(name)

	// Check alias table first (alias → canonical builtin name).
	r.builtinMu.RLock()
	if canonical, ok := r.aliases[lower]; ok {
		if cmd, ok := r.builtin[canonical]; ok {
			r.builtinMu.RUnlock()
			return cmd, true
		}
	}

	// Builtin takes highest precedence.
	if cmd, ok := r.builtin[lower]; ok {
		r.builtinMu.RUnlock()
		return cmd, true
	}
	r.builtinMu.RUnlock()

	// Custom command snapshot (atomic load — no locking required).
	snap := r.snapshot.Load()

	if cmd, ok := snap.project[lower]; ok {
		// Warn when a builtin would shadow this (already won above, so this is informational).
		return cmd, true
	}
	if cmd, ok := snap.user[lower]; ok {
		return cmd, true
	}

	// Skill-backed providers (lowest precedence).
	r.providerMu.RLock()
	defer r.providerMu.RUnlock()
	for _, p := range r.providers {
		for _, c := range p.Commands() {
			if strings.ToLower(c.Name()) == lower {
				return c, true
			}
		}
	}

	return nil, false
}

// Reload atomically rebuilds the custom command snapshot by re-scanning
// all configured custom roots. In-flight Resolve calls complete against the
// pre-swap snapshot because Resolve loads the snapshot pointer once at the
// start and holds it for the duration of the call. REQ-CMD-010, REQ-CMD-012.
//
// @MX:WARN: [AUTO] Reload performs filesystem I/O and atomic pointer swap; ensure callers tolerate latency.
// @MX:REASON: WalkDir on large directories may block; snapshot swap is lock-free but not instantaneous.
func (r *Registry) Reload(_ context.Context) error {
	newProject := make(map[string]Command)

	if customLoader != nil {
		for _, root := range r.customRoots {
			// @MX:TODO: [AUTO] WithCustomRoots가 tier 메타데이터를 가질 때 project vs user root 구분 필요.
			// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-010
			cmds, err := customLoader(root, SourceCustomProject, r.logger)
			if err != nil {
				r.logger.Error("failed to reload custom commands",
					zap.String("root", root),
					zap.Error(err),
				)
				continue
			}
			for _, cmd := range cmds {
				name := strings.ToLower(cmd.Name())
				newProject[name] = cmd
			}
		}
	}

	newSnap := &commandSnapshot{
		project: newProject,
		user:    make(map[string]Command),
	}
	r.snapshot.Store(newSnap)
	return nil
}

// List returns the Metadata for all registered commands, sorted alphabetically.
// Used by /help.
func (r *Registry) List() []Metadata {
	seen := make(map[string]Metadata)

	r.builtinMu.RLock()
	for name, cmd := range r.builtin {
		seen[name] = cmd.Metadata()
	}
	r.builtinMu.RUnlock()

	snap := r.snapshot.Load()
	for name, cmd := range snap.project {
		if _, ok := seen[name]; !ok {
			seen[name] = cmd.Metadata()
		}
	}
	for name, cmd := range snap.user {
		if _, ok := seen[name]; !ok {
			seen[name] = cmd.Metadata()
		}
	}

	r.providerMu.RLock()
	for _, p := range r.providers {
		for _, c := range p.Commands() {
			name := strings.ToLower(c.Name())
			if _, ok := seen[name]; !ok {
				seen[name] = c.Metadata()
			}
		}
	}
	r.providerMu.RUnlock()

	result := make([]Metadata, 0, len(seen))
	for _, m := range seen {
		result = append(result, m)
	}

	// Return in deterministic alphabetical order.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Description < result[j].Description
	})

	return result
}

// ListNamed returns name+Metadata pairs for all registered commands, sorted by name.
// It is used by the /help command to display alphabetically ordered command listings.
func (r *Registry) ListNamed() []NamedMetadata {
	seen := make(map[string]NamedMetadata)

	r.builtinMu.RLock()
	for name, cmd := range r.builtin {
		seen[name] = NamedMetadata{Name: name, Metadata: cmd.Metadata()}
	}
	r.builtinMu.RUnlock()

	snap := r.snapshot.Load()
	for name, cmd := range snap.project {
		if _, ok := seen[name]; !ok {
			seen[name] = NamedMetadata{Name: name, Metadata: cmd.Metadata()}
		}
	}
	for name, cmd := range snap.user {
		if _, ok := seen[name]; !ok {
			seen[name] = NamedMetadata{Name: name, Metadata: cmd.Metadata()}
		}
	}

	r.providerMu.RLock()
	for _, p := range r.providers {
		for _, c := range p.Commands() {
			name := strings.ToLower(c.Name())
			if _, ok := seen[name]; !ok {
				seen[name] = NamedMetadata{Name: name, Metadata: c.Metadata()}
			}
		}
	}
	r.providerMu.RUnlock()

	result := make([]NamedMetadata, 0, len(seen))
	for _, nm := range seen {
		result = append(result, nm)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// copyMap returns a shallow copy of a command map.
func copyMap(m map[string]Command) map[string]Command {
	out := make(map[string]Command, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// sourceString converts a Source constant to a human-readable string for logging.
func sourceString(s Source) string {
	switch s {
	case SourceBuiltin:
		return "builtin"
	case SourceCustomProject:
		return "custom-project"
	case SourceCustomUser:
		return "custom-user"
	case SourceSkill:
		return "skill"
	default:
		return "unknown"
	}
}
