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

type namespaceJSON struct {
	Name    string `json:"name"`
	Entries uint64 `json:"entries"`
	Chunks  uint64 `json:"chunks"`
	Digest  string `json:"digest,omitempty"`
}

type verifyJSON struct {
	OK           bool            `json:"ok"`
	Level        string          `json:"level"`
	TotalEntries uint64          `json:"totalEntries"`
	TotalChunks  uint64          `json:"totalChunks"`
	Namespaces   []namespaceJSON `json:"namespaces"`
	RootHash     string          `json:"rootHash,omitempty"`
}

func newVerifyCmd() *cobra.Command {
	var level string
	cmd := &cobra.Command{
		Use:   "verify <file>",
		Short: "Verify the integrity of an SCLS file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lvl, err := parseLevel(level)
			if err != nil {
				return err
			}
			r, closeFn, err := openStream(args[0], cmd.InOrStdin())
			if err != nil {
				return err
			}
			defer func() { _ = closeFn() }()
			res, err := scls.Verify(r, lvl)
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			return renderVerify(cmd, res, level, asJSON)
		},
	}
	cmd.Flags().StringVar(&level, "level", "full",
		"verification level: structure|chunks|full")
	return cmd
}

func parseLevel(s string) (scls.VerifyLevel, error) {
	switch s {
	case "structure":
		return scls.VerifyStructure, nil
	case "chunks":
		return scls.VerifyChunks, nil
	case "full":
		return scls.VerifyFull, nil
	default:
		return 0, fmt.Errorf("unknown verify level %q (want structure|chunks|full)", s)
	}
}

func renderVerify(cmd *cobra.Command, res *scls.VerifyResult, level string, asJSON bool) error {
	if asJSON {
		out := verifyJSON{
			OK:           true,
			Level:        level,
			TotalEntries: res.TotalEntries,
			TotalChunks:  res.TotalChunks,
			Namespaces:   []namespaceJSON{},
		}
		if res.RootHash != (scls.Hash{}) {
			out.RootHash = hexstr(res.RootHash[:])
		}
		for _, ns := range res.Namespaces {
			nsOut := namespaceJSON{
				Name:    ns.Name,
				Entries: ns.EntriesCount,
				Chunks:  ns.ChunksCount,
			}
			if ns.Digest != (scls.Hash{}) {
				nsOut.Digest = hexstr(ns.Digest[:])
			}
			out.Namespaces = append(out.Namespaces, nsOut)
		}
		return printJSON(cmd.OutOrStdout(), out)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "OK (%s)\n", level)
	fmt.Fprintf(w, "entries: %d  chunks: %d\n", res.TotalEntries, res.TotalChunks)
	for _, ns := range res.Namespaces {
		fmt.Fprintf(w, "  %s  entries=%d chunks=%d\n", ns.Name, ns.EntriesCount, ns.ChunksCount)
	}
	if res.RootHash != (scls.Hash{}) {
		fmt.Fprintf(w, "root: %s\n", hexstr(res.RootHash[:]))
	}
	return nil
}
