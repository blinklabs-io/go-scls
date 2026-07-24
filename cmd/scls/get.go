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
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/blinklabs-io/go-scls"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var asHex bool
	cmd := &cobra.Command{
		Use:   "get <file> <namespace> <key-hex>",
		Short: "Fetch a value by key",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := hex.DecodeString(args[2])
			if err != nil {
				return fmt.Errorf("invalid key hex: %w", err)
			}
			value, err := getValue(
				args[0],
				args[1],
				key,
				cmd.InOrStdin(),
			)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			asJSON, _ := cmd.Flags().GetBool("json")
			switch {
			case asJSON:
				return printJSON(w, map[string]string{
					"namespace": args[1],
					"key":       args[2],
					"value":     hex.EncodeToString(value),
				})
			case asHex:
				_, err := fmt.Fprintln(w, hex.EncodeToString(value))
				return err
			default:
				_, err := w.Write(value)
				return err
			}
		},
	}
	cmd.Flags().BoolVar(&asHex, "hex", false, "print the value as hex instead of raw bytes")
	return cmd
}

// getValue fetches (ns, key). A real file uses the indexed random-access path
// (Open+Get); "-" uses the single-pass streaming Lookup.
func getValue(
	path, ns string,
	key []byte,
	stdin io.Reader,
) ([]byte, error) {
	if path == "-" {
		return scls.Lookup(stdin, ns, key)
	}
	f, err := os.Open(path) //nolint:gosec // user-specified input path
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	s, err := scls.Open(f, fi.Size())
	if err != nil {
		return nil, err
	}
	return s.Get(ns, key)
}
