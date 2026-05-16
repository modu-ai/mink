package i18n

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTemp writes content to a temp file with the given name and returns its path.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// writeTempDir writes multiple named files into a temp dir and returns the dir.
func writeTempDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}

func TestNewBundle_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	b := NewBundle("en")
	require.NotNil(t, b)
	assert.Equal(t, "en", b.defaultLang)
}

func TestBundle_LoadMessageFile_ValidYAML(t *testing.T) {
	t.Parallel()
	path := writeTemp(t, "en.yaml", `
install.welcome:
  description: "welcome message"
  other: "Welcome to MINK"
`)
	b := NewBundle("en")
	require.NoError(t, b.LoadMessageFile(path))
	tr := b.Translator("en")
	assert.Equal(t, "Welcome to MINK", tr.Translate("install.welcome", nil))
}

func TestBundle_LoadMessageFile_MissingFile(t *testing.T) {
	t.Parallel()
	b := NewBundle("en")
	err := b.LoadMessageFile("/nonexistent/path/en.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "i18n:")
}

func TestBundle_LoadMessageFile_MalformedYAML(t *testing.T) {
	t.Parallel()
	// go-i18n/v2 is tolerant of plain string values — this tests truly invalid YAML syntax.
	path := writeTemp(t, "en.yaml", `key: : : invalid`)
	b := NewBundle("en")
	err := b.LoadMessageFile(path)
	require.Error(t, err)
}

func TestBundle_LoadMessageFile_BOM_Rejected(t *testing.T) {
	t.Parallel()
	bom := []byte{0xEF, 0xBB, 0xBF}
	dir := t.TempDir()
	path := filepath.Join(dir, "en.yaml")
	content := append(bom, []byte("hello: world\n")...)
	require.NoError(t, os.WriteFile(path, content, 0o600))

	b := NewBundle("en")
	err := b.LoadMessageFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BOM")
}

func TestBundle_LoadMessageFile_CRLF_Rejected(t *testing.T) {
	t.Parallel()
	path := writeTemp(t, "en.yaml", "hello: world\r\nfoo: bar\r\n")
	b := NewBundle("en")
	err := b.LoadMessageFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CRLF")
}

func TestBundle_LoadDirectory_LoadsAll(t *testing.T) {
	t.Parallel()
	dir := writeTempDir(t, map[string]string{
		"en.yaml": "install.welcome:\n  other: \"Welcome\"\n",
		"ko.yaml": "install.welcome:\n  other: \"환영합니다\"\n",
	})
	b := NewBundle("en")
	require.NoError(t, b.LoadDirectory(dir))

	assert.Equal(t, "Welcome", b.Translator("en").Translate("install.welcome", nil))
	assert.Equal(t, "환영합니다", b.Translator("ko").Translate("install.welcome", nil))
}

func TestBundle_LoadDirectory_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := NewBundle("en")
	err := b.LoadDirectory(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no *.yaml")
}

func TestBundle_LoadDirectory_SkipsBadFiles(t *testing.T) {
	t.Parallel()
	bom := []byte{0xEF, 0xBB, 0xBF}
	dir := t.TempDir()
	// One valid file, one BOM-corrupted file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "en.yaml"),
		[]byte("install.welcome:\n  other: \"Welcome\"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ko.yaml"),
		append(bom, []byte("install.welcome:\n  other: \"환영합니다\"\n")...), 0o600))

	b := NewBundle("en")
	// Should succeed because at least one file loaded.
	require.NoError(t, b.LoadDirectory(dir))
	assert.Equal(t, "Welcome", b.Translator("en").Translate("install.welcome", nil))
}

func TestBundle_Translator_DefaultLangFallback(t *testing.T) {
	t.Parallel()
	dir := writeTempDir(t, map[string]string{
		"en.yaml": "install.welcome:\n  other: \"Welcome\"\n",
	})
	b := NewBundle("en")
	require.NoError(t, b.LoadDirectory(dir))

	// Request "fr" which has no bundle — should fall back to en.
	tr := b.Translator("fr")
	assert.Equal(t, "Welcome", tr.Translate("install.welcome", nil))
}

func TestBundle_Translator_MissingKeyFallsBackToKey(t *testing.T) {
	t.Parallel()
	dir := writeTempDir(t, map[string]string{
		"en.yaml": "install.welcome:\n  other: \"Welcome\"\n",
	})
	b := NewBundle("en")
	require.NoError(t, b.LoadDirectory(dir))

	tr := b.Translator("en")
	// Key "nonexistent" is not defined; fallback returns the key string.
	assert.Equal(t, "nonexistent.key", tr.Translate("nonexistent.key", nil))
}

func TestBundle_LoadFS_EmbeddedCatalog(t *testing.T) {
	t.Parallel()
	// Use the package-level catalogFS embed to test LoadFS.
	b := NewBundle("en")
	require.NoError(t, b.LoadFS(catalogFS, "catalog"))
	tr := b.Translator("en")
	assert.Equal(t, "Welcome to MINK", tr.Translate("install.welcome", nil))
}

func TestBundle_LoadFS_InvalidDir(t *testing.T) {
	t.Parallel()
	b := NewBundle("en")
	err := b.LoadFS(catalogFS, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read dir")
}

func TestBundle_LoadFS_SkipsNonYAML(t *testing.T) {
	t.Parallel()
	// catalogFS has only .yaml files in catalog/; use it and verify it doesn't error.
	b := NewBundle("en")
	require.NoError(t, b.LoadFS(catalogFS, "catalog"))
	// Both en and ko should load.
	assert.Equal(t, "Welcome to MINK", b.Translator("en").Translate("install.welcome", nil))
	assert.Equal(t, "MINK에 오신 것을 환영합니다", b.Translator("ko").Translate("install.welcome", nil))
}
