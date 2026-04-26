package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidator_ReservedName_Rejected는 AC-PL-003을 검증한다.
// 예약된 이름(_evil, goose, claude 등)은 ErrReservedPluginName을 반환해야 한다.
func TestValidator_ReservedName_Rejected(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"_evil"},
		{"goose"},
		{"claude"},
		{"mcp"},
		{"plugin"},
		{"_anything"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := PluginManifest{Name: tc.name, Version: "1.0.0"}
			err := ValidateManifest(m)
			var e ErrReservedPluginName
			assert.ErrorAs(t, err, &e, "expected ErrReservedPluginName for %q", tc.name)
		})
	}
}

// TestValidator_ValidName는 올바른 이름은 통과함을 검증한다.
func TestValidator_ValidName(t *testing.T) {
	m := PluginManifest{Name: "my-plugin", Version: "1.0.0"}
	err := ValidateManifest(m)
	assert.NoError(t, err)
}

// TestValidator_InvalidName_Format은 이름 형식 오류를 검증한다.
func TestValidator_InvalidName_Format(t *testing.T) {
	tests := []struct {
		name string
	}{
		{""},
		{"My-Plugin"}, // 대문자
		{"a"},         // 너무 짧음 (< 2자)
		{"my plugin"}, // 공백
		{"my_plugin"}, // 밑줄(예약 아니지만 형식 불일치)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := PluginManifest{Name: tc.name, Version: "1.0.0"}
			err := ValidateManifest(m)
			assert.Error(t, err, "name %q should be invalid", tc.name)
		})
	}
}

// TestValidator_InvalidVersion은 유효하지 않은 semver 버전을 검증한다.
func TestValidator_InvalidVersion(t *testing.T) {
	tests := []string{"", "not-a-version", "1.2", "v1.0.0-bad!"}
	for _, v := range tests {
		t.Run(v, func(t *testing.T) {
			m := PluginManifest{Name: "my-plugin", Version: v}
			err := ValidateManifest(m)
			var e ErrInvalidManifest
			assert.ErrorAs(t, err, &e, "expected ErrInvalidManifest for version %q", v)
		})
	}
}

// TestValidator_ValidVersions는 유효한 semver를 검증한다.
func TestValidator_ValidVersions(t *testing.T) {
	tests := []string{"1.0.0", "2.3.4", "0.1.0-alpha.1", "1.0.0+build.1"}
	for _, v := range tests {
		t.Run(v, func(t *testing.T) {
			m := PluginManifest{Name: "my-plugin", Version: v}
			err := ValidateManifest(m)
			assert.NoError(t, err, "version %q should be valid semver", v)
		})
	}
}

// TestValidator_UnknownHookEvent는 AC-PL-004를 검증한다.
// HOOK-001의 24개 이벤트 외의 이벤트는 ErrUnknownHookEvent를 반환해야 한다.
func TestValidator_UnknownHookEvent(t *testing.T) {
	m := PluginManifest{
		Name:    "my-plugin",
		Version: "1.0.0",
		Hooks: map[string][]PluginHookGroup{
			"FrobnicateStart": {
				{Hooks: []PluginHookEntry{{Command: "echo hello"}}},
			},
		},
	}
	err := ValidateManifest(m)
	var e ErrUnknownHookEvent
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "FrobnicateStart", e.Event)
}

// TestValidator_KnownHookEvent는 유효한 hook 이벤트를 검증한다.
func TestValidator_KnownHookEvent(t *testing.T) {
	m := PluginManifest{
		Name:    "my-plugin",
		Version: "1.0.0",
		Hooks: map[string][]PluginHookGroup{
			"SessionStart": {
				{Hooks: []PluginHookEntry{{Command: "echo hello"}}},
			},
			"Stop": {
				{Hooks: []PluginHookEntry{{Command: "echo bye"}}},
			},
		},
	}
	err := ValidateManifest(m)
	assert.NoError(t, err)
}

// TestValidator_CredentialsInURI는 AC-PL-010을 검증한다.
// mcpServers URI에 자격증명(user:pass@host)이 포함되면 ErrCredentialsInURI를 반환해야 한다.
func TestValidator_CredentialsInURI(t *testing.T) {
	m := PluginManifest{
		Name:    "my-plugin",
		Version: "1.0.0",
		MCPServers: []PluginMCPServerConfig{
			{Name: "srv", URI: "https://user:secret@host.com/mcp"},
		},
	}
	err := ValidateManifest(m)
	var e ErrCredentialsInURI
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "https://user:secret@host.com/mcp", e.URI)
}

// TestValidator_CleanURI는 자격증명 없는 URI는 통과함을 검증한다.
func TestValidator_CleanURI(t *testing.T) {
	m := PluginManifest{
		Name:    "my-plugin",
		Version: "1.0.0",
		MCPServers: []PluginMCPServerConfig{
			{Name: "srv", URI: "https://host.com/mcp"},
		},
	}
	err := ValidateManifest(m)
	assert.NoError(t, err)
}
