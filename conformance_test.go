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

package scls

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestConformanceFixtures verifies every .scls file in testdata/ plus an
// optional externally generated file pointed to by SCLS_CONFORMANCE_FILE
// (e.g. produced by cardano-cls or cardano-scrawls).
func TestConformanceFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/*.scls")
	if err != nil {
		t.Fatal(err)
	}
	if extra := os.Getenv("SCLS_CONFORMANCE_FILE"); extra != "" {
		files = append(files, extra)
	}
	if len(files) == 0 {
		t.Skip("no conformance fixtures available")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Verify(bytes.NewReader(data), VerifyFull)
			if err != nil {
				t.Fatalf("verify %s: %v", f, err)
			}
			t.Logf("%s: %d entries, %d chunks, root %x",
				f, res.TotalEntries, res.TotalChunks, res.RootHash)
		})
	}
}
