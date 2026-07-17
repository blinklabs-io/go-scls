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
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/blinklabs-io/go-scls"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls <file> [namespace]",
		Short: "List namespaces, or the keys of one namespace",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			if len(args) == 1 {
				m, err := loadManifest(args[0])
				if err != nil {
					return err
				}
				return renderNamespaces(cmd, m, asJSON)
			}
			return streamKeys(cmd, args[0], args[1], asJSON)
		},
	}
	return cmd
}

// streamKeys streams the keys of namespace ns from path straight to the
// command's output, so listing a large namespace uses constant memory instead
// of buffering every key. Namespaces are emitted in ascending order, so it
// stops once the stream passes ns.
func streamKeys(cmd *cobra.Command, path, ns string, asJSON bool) error {
	r, closeFn, err := openStream(path)
	if err != nil {
		return err
	}
	defer func() { _ = closeFn() }()
	sr, err := scls.NewReader(r)
	if err != nil {
		return err
	}
	kw := newKeyWriter(cmd.OutOrStdout(), ns, asJSON)
	if kw.err != nil {
		return kw.err
	}
	for {
		c, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if c.Namespace < ns {
			continue
		}
		if c.Namespace > ns {
			break
		}
		for _, e := range c.Entries {
			kw.add(e.Key)
			if kw.err != nil {
				return kw.err
			}
		}
	}
	return kw.close()
}

// keyWriter streams the keys of one namespace, as newline-delimited hex (text)
// or an incrementally-built JSON object. It holds no more than the current key,
// and stops writing after the first I/O error (which close reports).
type keyWriter struct {
	w     io.Writer
	json  bool
	wrote bool // at least one JSON array element emitted
	err   error
}

func newKeyWriter(w io.Writer, ns string, asJSON bool) *keyWriter {
	kw := &keyWriter{w: w, json: asJSON}
	if asJSON {
		// json.Marshal escapes the namespace string; it cannot fail for a
		// string value, but the error is propagated for completeness.
		nsJSON, err := json.Marshal(ns)
		if err != nil {
			kw.err = err
			return kw
		}
		_, kw.err = fmt.Fprintf(w, "{\n  \"namespace\": %s,\n  \"keys\": [", nsJSON)
	}
	return kw
}

func (kw *keyWriter) add(key []byte) {
	if kw.err != nil {
		return
	}
	// hexstr yields only [0-9a-f], so %q produces a valid JSON string with no
	// characters needing escaping.
	if kw.json {
		sep := ","
		if !kw.wrote {
			sep = ""
		}
		_, kw.err = fmt.Fprintf(kw.w, "%s\n    %q", sep, hexstr(key))
		kw.wrote = true
		return
	}
	_, kw.err = fmt.Fprintln(kw.w, hexstr(key))
}

func (kw *keyWriter) close() error {
	if kw.err != nil || !kw.json {
		return kw.err
	}
	if kw.wrote {
		_, kw.err = io.WriteString(kw.w, "\n  ]\n}\n")
	} else {
		_, kw.err = io.WriteString(kw.w, "]\n}\n")
	}
	return kw.err
}

func renderNamespaces(cmd *cobra.Command, m *scls.Manifest, asJSON bool) error {
	if asJSON {
		out := struct {
			Namespaces []namespaceJSON `json:"namespaces"`
		}{Namespaces: []namespaceJSON{}}
		for _, ns := range m.Namespaces {
			out.Namespaces = append(out.Namespaces, namespaceJSON{
				Name: ns.Name, Entries: ns.EntriesCount,
				Chunks: ns.ChunksCount, Digest: hexstr(ns.Digest[:]),
			})
		}
		return printJSON(cmd.OutOrStdout(), out)
	}
	w := cmd.OutOrStdout()
	for _, ns := range m.Namespaces {
		fmt.Fprintf(w, "%s  entries=%d chunks=%d\n", ns.Name, ns.EntriesCount, ns.ChunksCount)
	}
	return nil
}
