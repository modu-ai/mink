// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

//go:build cgo

// Package sqlite requires cgo to compile the mattn/go-sqlite3 driver.
// This file is compiled only when CGO_ENABLED=1 (the default).
package sqlite

// CGOEnabled is true when the package was built with cgo enabled.
// The Store.Open function checks this constant at runtime to provide a
// clear diagnostic on unsupported build configurations.
const CGOEnabled = true
