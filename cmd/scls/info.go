// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"

	"github.com/blinklabs-io/go-scls"
	"github.com/spf13/cobra"
)

type infoJSON struct {
	Version      uint32          `json:"version"`
	Slot         uint64          `json:"slot"`
	CreatedAt    string          `json:"createdAt"`
	Tool         string          `json:"tool"`
	Comment      string          `json:"comment"`
	TotalEntries uint64          `json:"totalEntries"`
	TotalChunks  uint64          `json:"totalChunks"`
	PrevManifest uint64          `json:"prevManifest"`
	RootHash     string          `json:"rootHash"`
	Namespaces   []namespaceJSON `json:"namespaces"`
}

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <file>",
		Short: "Show SCLS file metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ver, m, err := readHeaderAndManifest(args[0])
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			return renderInfo(cmd, ver, m, asJSON)
		},
	}
	return cmd
}

func renderInfo(cmd *cobra.Command, ver uint32, m *scls.Manifest, asJSON bool) error {
	if asJSON {
		out := infoJSON{
			Version:      ver,
			Slot:         m.SlotNo,
			CreatedAt:    m.Summary.CreatedAt,
			Tool:         m.Summary.Tool,
			Comment:      m.Summary.Comment,
			TotalEntries: m.TotalEntries,
			TotalChunks:  m.TotalChunks,
			PrevManifest: m.PrevManifest,
			RootHash:     hexstr(m.RootHash[:]),
		}
		for _, ns := range m.Namespaces {
			out.Namespaces = append(out.Namespaces, namespaceJSON{
				Name: ns.Name, Entries: ns.EntriesCount,
				Chunks: ns.ChunksCount, Digest: hexstr(ns.Digest[:]),
			})
		}
		return printJSON(cmd.OutOrStdout(), out)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "format version: %d\n", ver)
	fmt.Fprintf(w, "slot:           %d\n", m.SlotNo)
	fmt.Fprintf(w, "created-at:     %s\n", m.Summary.CreatedAt)
	fmt.Fprintf(w, "tool:           %s\n", m.Summary.Tool)
	fmt.Fprintf(w, "comment:        %s\n", m.Summary.Comment)
	fmt.Fprintf(w, "entries:        %d\n", m.TotalEntries)
	fmt.Fprintf(w, "chunks:         %d\n", m.TotalChunks)
	fmt.Fprintf(w, "prev-manifest:  %d\n", m.PrevManifest)
	fmt.Fprintf(w, "root:           %s\n", hexstr(m.RootHash[:]))
	fmt.Fprintln(w, "namespaces:")
	for _, ns := range m.Namespaces {
		fmt.Fprintf(w, "  %s  entries=%d chunks=%d digest=%s\n",
			ns.Name, ns.EntriesCount, ns.ChunksCount, hexstr(ns.Digest[:]))
	}
	return nil
}
