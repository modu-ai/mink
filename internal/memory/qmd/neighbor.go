// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package qmd

// LinkNeighbors fills the PrevChunkID and NextChunkID fields of each Chunk
// based on its position in the slice.
//
// Rules:
//   - First chunk: PrevChunkID is empty.
//   - Last chunk:  NextChunkID is empty.
//   - All other chunks: PrevChunkID = ID of the preceding chunk,
//     NextChunkID = ID of the following chunk.
//
// The input slice must have IDs already assigned (by the caller calling
// ChunkID) before LinkNeighbors is invoked.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T1.5
// REQ:  REQ-MEM-012
func LinkNeighbors(chunks []Chunk) []Chunk {
	for i := range chunks {
		if i > 0 {
			chunks[i].PrevChunkID = chunks[i-1].ID
		}
		if i < len(chunks)-1 {
			chunks[i].NextChunkID = chunks[i+1].ID
		}
	}
	return chunks
}
