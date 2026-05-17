// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build !cgo

// Package sqlite requires cgo to compile the mattn/go-sqlite3 driver.
// This file is compiled when CGO_ENABLED=0 and provides stub types and a
// runtime error at Open time so that the binary compiles cleanly but fails
// with a clear diagnostic rather than a silent link error.
package sqlite

import (
	"errors"
)

// CGOEnabled is false when the package was built without cgo.
// Store.Open returns ErrNoCGO when it detects this constant is false.
const CGOEnabled = false

// ErrNoCGO is returned by Open when the package was built without cgo support.
var ErrNoCGO = errors.New("internal/memory/sqlite requires cgo (set CGO_ENABLED=1 and rebuild)")

// Store is a stub type for no-cgo builds.  It provides no functionality.
type Store struct{}

// Open always returns ErrNoCGO in no-cgo builds.
func Open(_ string) (*Store, error) {
	return nil, ErrNoCGO
}

// HasVec0 always returns false in no-cgo builds.
func (s *Store) HasVec0() bool { return false }

// Close is a no-op in no-cgo builds.
func (s *Store) Close() error { return nil }

// DB returns nil in no-cgo builds.
func (s *Store) DB() interface{} { return nil }
