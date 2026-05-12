package envalias

// ResetForTesting resets the package-level singleton so unit tests can call Init again.
// Exported only for testing; must not be used in production code.
func ResetForTesting() {
	resetForTesting()
}
