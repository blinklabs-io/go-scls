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
	"errors"
	"io"
	"os"

	"github.com/blinklabs-io/go-scls"
)

// openStream returns a reader over path, or os.Stdin for "-". The returned
// close func is always safe to call (a no-op for stdin).
func openStream(path string) (io.Reader, func() error, error) {
	if path == "-" {
		return os.Stdin, func() error { return nil }, nil
	}
	f, err := os.Open(path) //nolint:gosec // user-specified input path
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// drainReader validates the SCLS header on r, then consumes every CHUNK to EOF
// so the parsed MANIFEST (and header) are available on the returned Reader. It
// is the streaming path used for stdin ("-"), where the trailing-offset
// bookend that ReadManifest relies on cannot be seeked to. Callers that also
// need per-namespace entries stream them directly rather than going through
// here.
func drainReader(r io.Reader) (*scls.Reader, error) {
	sr, err := scls.NewReader(r)
	if err != nil {
		return nil, err
	}
	for {
		_, err := sr.Next()
		if errors.Is(err, io.EOF) {
			return sr, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// readHeaderAndManifest returns the format version and MANIFEST, validating the
// HDR record in both cases. A real file reads the header, then locates the
// manifest via the trailing-offset bookend (no full scan); "-" streams stdin to
// EOF.
func readHeaderAndManifest(path string) (uint32, *scls.Manifest, error) {
	if path == "-" {
		sr, err := drainReader(os.Stdin)
		if err != nil {
			return 0, nil, err
		}
		return sr.Header().Version, sr.Manifest(), nil
	}
	f, err := os.Open(path) //nolint:gosec // user-specified input path
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()
	sr, err := scls.NewReader(f) // validates the HDR record
	if err != nil {
		return 0, nil, err
	}
	ver := sr.Header().Version
	// ReadManifest seeks to the end itself, so the file position left by
	// NewReader is irrelevant here — no reset seek is needed.
	m, err := scls.ReadManifest(f)
	if err != nil {
		return 0, nil, err
	}
	return ver, m, nil
}

// loadManifest returns the file's MANIFEST, validating the HDR first so that a
// non-SCLS blob carrying a syntactically valid trailing manifest is rejected
// rather than presented as SCLS metadata.
func loadManifest(path string) (*scls.Manifest, error) {
	_, m, err := readHeaderAndManifest(path)
	return m, err
}
