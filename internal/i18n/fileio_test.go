package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBytes_Clean(t *testing.T) {
	t.Parallel()
	data := []byte("hello: world\nfoo: bar\n")
	out, err := validateBytes(data, "test.yaml")
	require.NoError(t, err)
	assert.Equal(t, data, out)
}

func TestValidateBytes_BOM_Rejected(t *testing.T) {
	t.Parallel()
	bom := []byte{0xEF, 0xBB, 0xBF}
	data := append(bom, []byte("hello: world\n")...)
	_, err := validateBytes(data, "ko.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BOM")
	assert.Contains(t, err.Error(), "ko.yaml")
}

func TestValidateBytes_CRLF_Rejected(t *testing.T) {
	t.Parallel()
	data := []byte("hello: world\r\nfoo: bar\r\n")
	_, err := validateBytes(data, "en.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CRLF")
	assert.Contains(t, err.Error(), "en.yaml")
}

func TestValidateBytes_EmptyContent(t *testing.T) {
	t.Parallel()
	data := []byte{}
	out, err := validateBytes(data, "empty.yaml")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestReadFileStrict_NonExistent(t *testing.T) {
	t.Parallel()
	_, err := readFileStrict("/does/not/exist/en.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "i18n: read")
}
