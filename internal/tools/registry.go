package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/modu-ai/mink/internal/tools/mcp"
	"github.com/modu-ai/mink/internal/tools/naming"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.uber.org/zap"
)

// schemaCounter는 고유 schema URL 생성을 위한 전역 카운터이다.
var schemaCounter atomic.Uint64

// registryEntry는 Registry 내부에서 tool과 메타데이터를 저장하는 구조체이다.
type registryEntry struct {
	tool       Tool
	descriptor ToolDescriptor
	compiled   *jsonschema.Schema // 등록 시점에 컴파일된 JSON Schema (R6: 성능)
}

// Descriptor는 tool descriptor를 반환한다.
func (e registryEntry) Descriptor() ToolDescriptor {
	return e.descriptor
}

// Tool은 tool 인스턴스를 반환한다.
func (e registryEntry) Tool() Tool {
	return e.tool
}

// RegistryConfig는 Registry 설정이다.
type RegistryConfig struct {
	// Cwd는 FileWrite/FileEdit의 cwd boundary 기준 디렉토리이다.
	// REQ-TOOLS-015
	Cwd string
	// CoordinatorMode는 coordinator 모드 활성화 여부이다.
	// REQ-TOOLS-012
	CoordinatorMode bool
	// StrictSchema는 additionalProperties: false 강제 여부이다.
	// REQ-TOOLS-019
	StrictSchema bool
	// LogInvocations는 구조화 로그 출력 여부이다.
	// REQ-TOOLS-020
	LogInvocations bool
	// Logger는 zap 로거이다.
	Logger *zap.Logger
}

// Registry는 tool name → Tool 매핑을 RWMutex로 보호하는 저장소이다.
//
// @MX:ANCHOR: [AUTO] Tool Registry — Executor와 Inventory의 단일 데이터 소스
// @MX:REASON: SPEC-GOOSE-TOOLS-001 REQ-TOOLS-001 - fan_in >= 3 (Executor, Inventory, Search, test)
type Registry struct {
	mu       sync.RWMutex
	entries  map[string]registryEntry
	cfg      RegistryConfig
	draining atomic.Bool
	compiler *jsonschema.Compiler
	logger   *zap.Logger
}

// Option은 Registry 생성 옵션이다.
type Option func(*Registry)

// WithBuiltins는 built-in 6종 tool을 Registry에 등록한다.
func WithBuiltins() Option {
	return func(r *Registry) {
		// builtin 패키지에서 등록 — builtin.Register()를 통해 init()에서 호출된다.
		// 여기서는 registeredBuiltins 슬라이스를 순회하여 등록한다.
		for _, t := range globalBuiltins {
			if err := r.registerInternal(t, SourceBuiltin); err != nil {
				// REQ-TOOLS-013: 중복 built-in 등록 시 panic
				panic(fmt.Sprintf("builtin registration failed for %q: %v", t.Name(), err))
			}
		}
	}
}

// WithMCPConnections는 MCP Manager의 연결들을 adopt하고 ConnectionClosed 이벤트를 구독한다.
func WithMCPConnections(mgr mcp.Manager) Option {
	return func(r *Registry) {
		// 기존 연결 adopt
		for _, conn := range mgr.Connections() {
			if err := r.AdoptMCPServer(conn); err != nil {
				if r.logger != nil {
					r.logger.Error("failed to adopt MCP server",
						zap.String("serverID", conn.ServerID()),
						zap.Error(err),
					)
				}
			}
		}
		// ConnectionClosed 이벤트 구독
		go func() {
			for ev := range mgr.Subscribe() {
				r.unregisterMCPServer(ev.ServerID)
			}
		}()
	}
}

// NewRegistry는 옵션을 적용한 새 Registry를 생성한다.
func NewRegistry(opts ...Option) *Registry {
	compiler := jsonschema.NewCompiler()
	r := &Registry{
		entries:  make(map[string]registryEntry),
		compiler: compiler,
		logger:   zap.NewNop(),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// NewRegistryWithConfig는 설정과 옵션을 적용한 새 Registry를 생성한다.
func NewRegistryWithConfig(cfg RegistryConfig, opts ...Option) *Registry {
	compiler := jsonschema.NewCompiler()
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	r := &Registry{
		entries:  make(map[string]registryEntry),
		cfg:      cfg,
		compiler: compiler,
		logger:   logger,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Register는 tool을 Registry에 등록한다.
// REQ-TOOLS-002, REQ-TOOLS-003, REQ-TOOLS-013
func (r *Registry) Register(t Tool, src Source) error {
	return r.registerInternal(t, src)
}

// registerInternal은 내부 등록 로직이다.
func (r *Registry) registerInternal(t Tool, src Source) error {
	name := t.Name()
	schema := t.Schema()

	// REQ-TOOLS-002: Schema 유효성 검증
	compiled, err := r.compileSchema(name, schema)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidSchema, name, err)
	}

	// REQ-TOOLS-019: strict_schema 강제
	if r.cfg.StrictSchema {
		if err := validateStrictSchema(schema); err != nil {
			return fmt.Errorf("%w: %s: strict_schema: %v", ErrInvalidSchema, name, err)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 중복 검사
	if _, exists := r.entries[name]; exists {
		if src == SourceBuiltin {
			// REQ-TOOLS-013: built-in 중복 등록은 panic
			panic(fmt.Sprintf("duplicate builtin tool registration: %q", name))
		}
		return fmt.Errorf("%w: %s", ErrDuplicateName, name)
	}

	desc := ToolDescriptor{
		Name:   name,
		Schema: schema,
		Scope:  t.Scope(),
		Source: src,
	}

	r.entries[name] = registryEntry{
		tool:       t,
		descriptor: desc,
		compiled:   compiled,
	}
	return nil
}

// AdoptMCPServer는 MCP 연결에서 tool을 채택한다.
// REQ-TOOLS-004, REQ-TOOLS-017, REQ-TOOLS-021
func (r *Registry) AdoptMCPServer(conn mcp.Connection) error {
	serverID := conn.ServerID()

	// REQ-TOOLS-004: serverID 유효성 검사
	if !naming.IsValidServerID(serverID) {
		return fmt.Errorf("invalid serverID %q: must match [a-z0-9_-]{1,64}", serverID)
	}

	manifests := conn.ListTools()
	var firstErr error

	for _, m := range manifests {
		toolName := m.Name

		// REQ-TOOLS-017: tool 이름에 __ 포함 시 거부
		if naming.HasDoubleUnderscore(toolName) {
			r.logger.Error("rejected MCP tool with double-underscore in name",
				zap.String("serverID", serverID),
				zap.String("toolName", toolName),
			)
			continue
		}

		// REQ-TOOLS-003: built-in 예약어 침범 불가
		if naming.IsReservedName(toolName) {
			r.logger.Error("rejected MCP tool claiming reserved builtin name",
				zap.String("serverID", serverID),
				zap.String("toolName", toolName),
			)
			continue
		}

		canonicalName := naming.MCPToolName(serverID, toolName)

		// REQ-TOOLS-021: 중복 adoption 거부
		r.mu.RLock()
		_, exists := r.entries[canonicalName]
		r.mu.RUnlock()

		if exists {
			r.logger.Error("duplicate MCP tool adoption rejected",
				zap.String("serverID", serverID),
				zap.String("toolName", toolName),
				zap.String("canonicalName", canonicalName),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: %s", ErrDuplicateName, canonicalName)
			}
			continue
		}

		// MCP stub tool 생성 (deferred loading)
		stub := newMCPStubTool(conn, serverID, toolName, m)

		schema := buildMCPSchema(m)
		compiled, err := r.compileSchema(canonicalName, schema)
		if err != nil {
			r.logger.Error("failed to compile MCP tool schema",
				zap.String("canonicalName", canonicalName),
				zap.Error(err),
			)
			continue
		}

		r.mu.Lock()
		// 재확인 (TOCTOU)
		if _, exists := r.entries[canonicalName]; exists {
			r.mu.Unlock()
			r.logger.Error("duplicate MCP tool adoption rejected (race)",
				zap.String("serverID", serverID),
				zap.String("toolName", toolName),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("%w: %s", ErrDuplicateName, canonicalName)
			}
			continue
		}
		r.entries[canonicalName] = registryEntry{
			tool: stub,
			descriptor: ToolDescriptor{
				Name:     canonicalName,
				Schema:   schema,
				Scope:    ScopeShared,
				Source:   SourceMCP,
				ServerID: serverID,
			},
			compiled: compiled,
		}
		r.mu.Unlock()
	}

	return firstErr
}

// Resolve는 이름으로 Tool을 반환한다.
// REQ-TOOLS-001: RWMutex로 동시성 안전.
func (r *Registry) Resolve(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[name]
	if !ok {
		return nil, false
	}
	return entry.tool, true
}

// ResolveEntry는 이름으로 registryEntry를 반환한다 (내부 사용).
func (r *Registry) ResolveEntry(name string) (registryEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[name]
	return e, ok
}

// ListNames는 등록된 tool 이름의 정렬된 목록을 반환한다.
// REQ-TOOLS-005: alphabetical sort
func (r *Registry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.entries))
	for n := range r.entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Drain은 Registry를 Draining 상태로 전환한다.
// REQ-TOOLS-011: 이후 Executor.Run은 즉시 에러 반환.
func (r *Registry) Drain() {
	r.draining.Store(true)
}

// IsDraining은 Registry가 Draining 상태인지 반환한다.
func (r *Registry) IsDraining() bool {
	return r.draining.Load()
}

// unregisterMCPServer는 serverID에 속하는 모든 MCP tool을 제거한다.
// REQ-TOOLS-009: ConnectionClosed 이벤트 처리
func (r *Registry) unregisterMCPServer(serverID string) {
	prefix := naming.MCPPrefix + serverID + "__"
	r.mu.Lock()
	defer r.mu.Unlock()
	for name := range r.entries {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			delete(r.entries, name)
		}
	}
}

// Config는 Registry의 설정을 반환한다.
func (r *Registry) Config() RegistryConfig {
	return r.cfg
}

// compileSchema는 JSON Schema를 컴파일한다.
// R6: 등록 시점에 1회 컴파일 후 캐시
func (r *Registry) compileSchema(name string, schema json.RawMessage) (*jsonschema.Schema, error) {
	if len(schema) == 0 {
		return nil, fmt.Errorf("empty schema")
	}

	// JSON 유효성 먼저 확인
	var schemaMap map[string]any
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}

	// 고유 URL 생성으로 URL 충돌 방지
	id := strconv.FormatUint(schemaCounter.Add(1), 10)
	schemaURL := "https://tools.goose.internal/schema/" + id + "/" + name

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaURL, schemaMap); err != nil {
		return nil, fmt.Errorf("add resource: %v", err)
	}
	compiled, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile: %v", err)
	}
	return compiled, nil
}

// validateStrictSchema는 schema에 additionalProperties: false가 있는지 검사한다.
// REQ-TOOLS-019
func validateStrictSchema(schema json.RawMessage) error {
	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		return err
	}
	ap, ok := m["additionalProperties"]
	if !ok {
		return fmt.Errorf("missing additionalProperties field")
	}
	if b, ok := ap.(bool); !ok || b {
		return fmt.Errorf("additionalProperties must be false, got %v", ap)
	}
	return nil
}

// buildMCPSchema는 MCP manifest에서 JSON Schema를 생성한다.
func buildMCPSchema(m mcp.ToolManifest) json.RawMessage {
	if m.InputSchema != nil {
		b, err := json.Marshal(m.InputSchema)
		if err == nil {
			return json.RawMessage(b)
		}
	}
	// 기본 빈 스키마
	return json.RawMessage(`{"type":"object"}`)
}

// globalBuiltins는 init()에서 등록되는 built-in tool 목록이다.
var (
	globalBuiltinsMu sync.Mutex
	globalBuiltins   []Tool
)

// RegisterBuiltin은 builtin 패키지에서 전역 builtin 목록에 tool을 추가한다.
func RegisterBuiltin(t Tool) {
	globalBuiltinsMu.Lock()
	defer globalBuiltinsMu.Unlock()
	globalBuiltins = append(globalBuiltins, t)
}
