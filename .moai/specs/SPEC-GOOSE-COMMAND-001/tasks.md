## Task Decomposition

SPEC: SPEC-GOOSE-COMMAND-001
Mode: TDD (RED → GREEN → REFACTOR)
Harness: standard

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | Foundation types: Command interface, Metadata, Args, Result, Source enum, errors, SlashCommandContext interface, ModelInfo, SessionSnapshot | REQ-CMD-003 | - | internal/command/command.go, internal/command/errors.go, internal/command/context.go, internal/command/source.go | pending |
| T-002 | Parser: Parse(line) slash detection + SplitArgs (shell-like quote handling) | REQ-CMD-001, REQ-CMD-015 | T-001 | internal/command/parser/parser.go, internal/command/parser/parser_test.go | pending |
| T-003 | Substitute engine: $ARGUMENTS/$1..$9/$CWD/$$ single-pass expansion | REQ-CMD-006, REQ-CMD-013 | T-001 | internal/command/substitute/substitute.go, internal/command/substitute/substitute_test.go | pending |
| T-004 | Registry: Register/Resolve/Reload/RegisterProvider with builtin>project>user>skill precedence + atomic swap | REQ-CMD-002, REQ-CMD-003, REQ-CMD-010, REQ-CMD-012 | T-001 | internal/command/registry.go, internal/command/registry_test.go | pending |
| T-005 | Custom command loader: walk roots, parse YAML frontmatter, body extraction, malformed skip, symlink escape guard | REQ-CMD-009, REQ-CMD-016, REQ-CMD-017 | T-001, T-004 | internal/command/custom/loader.go, internal/command/custom/frontmatter.go, internal/command/custom/markdown.go, internal/command/custom/loader_test.go | pending |
| T-006 | Builtin commands (7): /help /clear /exit /model /compact /status /version + aliases (/quit /?) | REQ-CMD-005, REQ-CMD-007, REQ-CMD-008, REQ-CMD-019 | T-001, T-004 | internal/command/builtin/builtin.go, internal/command/builtin/help.go, internal/command/builtin/clear.go, internal/command/builtin/exit.go, internal/command/builtin/model.go, internal/command/builtin/compact.go, internal/command/builtin/status.go, internal/command/builtin/version.go, internal/command/builtin/builtin_test.go | pending |
| T-007 | Dispatcher: ProcessUserInput orchestration + MaxExpandedPromptBytes guard + plan-mode block | REQ-CMD-004, REQ-CMD-005, REQ-CMD-011, REQ-CMD-014 | T-002, T-003, T-004, T-005, T-006 | internal/command/dispatcher.go, internal/command/dispatcher_test.go | pending |

### Acceptance Criteria Coverage

| AC ID | Description | Covered by Task |
|-------|-------------|-----------------|
| AC-CMD-001 | /help lists all 7 builtins | T-006 + T-007 |
| AC-CMD-002 | Plain prompt passthrough | T-007 |
| AC-CMD-003 | Unknown command → LocalReply | T-007 |
| AC-CMD-004 | Custom command + $ARGUMENTS substitution | T-003 + T-005 + T-007 |
| AC-CMD-005 | Positional $1/$2 substitution | T-003 |
| AC-CMD-006 | /clear invokes OnClear | T-006 |
| AC-CMD-007 | /exit returns Exit{0} | T-006 |
| AC-CMD-008 | /model valid alias | T-006 |
| AC-CMD-009 | /model invalid alias | T-006 |
| AC-CMD-010 | Malformed frontmatter skipped | T-005 |
| AC-CMD-011 | builtin > custom precedence | T-004 + T-006 |
| AC-CMD-012 | $ARGUMENTS no recursion | T-003 |
| AC-CMD-013 | Max prompt size exceeded | T-007 |

### TDD Order (per SPEC §6.6)

1. RED #1-2 Parser slash detection
2. RED #3 Registry precedence
3. RED #4-5 Dispatcher unknown / passthrough
4. RED #6-10 Builtin commands
5. RED #11-13 Substitute engine
6. RED #14 Custom loader malformed skip
7. RED #15 Dispatcher MaxSize
8. GREEN: minimal implementations per RED
9. REFACTOR: substitute clarity, registry atomic.Pointer swap

### Test Coverage Targets

- Overall: 85% (quality.yaml test_coverage_target)
- substitute package: 100% (security boundary)
- All tests must run with `go test -race`
