// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package sqlite

import _ "embed"

// Schema holds the SQL DDL statements embedded from schema.sql at build time.
// It is used by MigrateSchema to initialise or upgrade the SQLite index.
//
//go:embed schema.sql
var Schema string
