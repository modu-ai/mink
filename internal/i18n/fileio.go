package i18n

import (
	"bytes"
	"fmt"
	"os"
)

// readFileStrict reads path and validates that the content does not start with a
// UTF-8 BOM (0xEF 0xBB 0xBF) and does not use Windows CRLF line endings.
// Both are rejected per REQ-I18N-003 (AC-I18N-016).
func readFileStrict(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("i18n: read %s: %w", path, err)
	}
	return validateBytes(data, path)
}

// validateBytes checks raw YAML bytes for BOM and CRLF (REQ-I18N-003).
func validateBytes(data []byte, name string) ([]byte, error) {
	// UTF-8 BOM is three bytes: 0xEF 0xBB 0xBF.
	bom := []byte{0xEF, 0xBB, 0xBF}
	if bytes.HasPrefix(data, bom) {
		return nil, fmt.Errorf("i18n: rejected: BOM present in %s", name)
	}
	// CRLF check: any occurrence of \r\n means the file uses Windows line endings.
	if bytes.Contains(data, []byte("\r\n")) {
		return nil, fmt.Errorf("i18n: rejected: CRLF line endings in %s", name)
	}
	return data, nil
}
