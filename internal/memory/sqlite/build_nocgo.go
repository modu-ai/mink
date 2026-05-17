// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build !cgo

// Package sqlite requires cgo to compile the mattn/go-sqlite3 driver.
// This file is compiled when CGO_ENABLED=0 and provides a runtime error
// at Open time so that the binary compiles cleanly but fails with a clear
// diagnostic rather than a silent link error.
package sqlite

// CGOEnabled is false when the package was built without cgo.
// Store.Open returns ErrNoCGO when it detects this constant is false.
const CGOEnabled = false
