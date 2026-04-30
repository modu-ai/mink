package memory

import (
	"errors"
)

// Is is a helper for error checking in tests.
func Is(err error, target error) bool {
	return errors.Is(err, target)
}
