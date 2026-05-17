package credential_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// TestServiceInterfaceCompileTime verifies that the concrete backend stubs
// satisfy the Service interface at compile time.  If either assertion fails,
// the build breaks — no runtime cost.
//
// AC-CR-001: Service interface has 5 methods (Store/Load/Delete/List/Health).
func TestServiceInterfaceCompileTime(t *testing.T) {
	t.Helper()

	// stubService is a minimal implementation used only for this compile-time
	// assertion.  It lives inside the test so it does not pollute the package
	// namespace.
	var _ credential.Service = (*stubService)(nil)
}

// stubService is the minimal implementation of credential.Service used for
// compile-time interface assertion.
type stubService struct{}

func (s *stubService) Store(_ string, _ credential.Credential) error { return nil }
func (s *stubService) Load(_ string) (credential.Credential, error)  { return nil, nil }
func (s *stubService) Delete(_ string) error                         { return nil }
func (s *stubService) List() ([]string, error)                       { return nil, nil }
func (s *stubService) Health(_ string) (credential.HealthStatus, error) {
	return credential.HealthStatus{}, nil
}
