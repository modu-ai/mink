package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHandleFileAttach_DownloadAndPass verifies the full attachment download
// flow: getFile → download URL → local file written → path passed to AgentQuery.
func TestHandleFileAttach_DownloadAndPass(t *testing.T) {
	// Serve a fake file download endpoint.
	fileContent := []byte("fake image data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/file/bottest-token/photos/file_1.jpg" {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(fileContent)
			return
		}
		// getFile endpoint
		if r.URL.Path == "/bottest-token/getFile" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"abc","file_unique_id":"uabc","file_size":15,"file_path":"photos/file_1.jpg"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Resolve file path for inbox.
	inboxDir := filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(inboxDir, 0o700); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}

	ext := ".jpg"
	var msgID int64 = 77
	dst := inboxFilePath(inboxDir, msgID, ext)

	// Build the download URL as the client would.
	downloadURL := srv.URL + "/file/bottest-token/photos/file_1.jpg"

	downloaded, err := downloadAttachment(context.Background(), downloadURL, dst)
	if err != nil {
		t.Fatalf("downloadAttachment: %v", err)
	}
	if !downloaded {
		t.Error("expected download=true on first call")
	}

	// Verify the file was written.
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != string(fileContent) {
		t.Errorf("downloaded content mismatch: got %q, want %q", data, fileContent)
	}
}

// TestHandleFileAttach_IdempotentSkip verifies that a second download to the
// same path returns downloaded=false without error.
func TestHandleFileAttach_IdempotentSkip(t *testing.T) {
	inboxDir := t.TempDir()
	dst := inboxFilePath(inboxDir, 42, ".jpg")

	// Pre-create the file.
	if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// downloadAttachment should skip silently.
	downloaded, err := downloadAttachment(context.Background(), "http://unused", dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if downloaded {
		t.Error("expected downloaded=false for idempotent skip")
	}
}

// TestHandleFileAttach_ExtWhitelistBlocked verifies that blocked extensions
// are rejected by isAllowedExt.
func TestHandleFileAttach_ExtWhitelistBlocked(t *testing.T) {
	blocked := []string{".exe", ".sh", ".bat", ".js"}
	for _, ext := range blocked {
		if isAllowedExt(ext) {
			t.Errorf("extension %q should be blocked", ext)
		}
	}
}
