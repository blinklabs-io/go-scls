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
	"encoding/binary"
	"fmt"
)

var magic = [4]byte{'S', 'C', 'L', 'S'}

// FormatVersion is the SCLS format version this library reads and writes.
// VERIFY(#2): answered — u32 BE on the wire (CIP prose "u16" is outdated).
const FormatVersion uint32 = 1

// Header is the HDR record payload: magic "SCLS" || version u32 BE.
type Header struct {
	Version uint32
}

func (h *Header) encode() []byte {
	buf := make([]byte, 8)
	copy(buf[0:4], magic[:])
	binary.BigEndian.PutUint32(buf[4:8], h.Version)
	return buf
}

func decodeHeader(payload []byte) (*Header, error) {
	if len(payload) < 8 {
		return nil, ErrInvalidHeader
	}
	if !bytes.Equal(payload[0:4], magic[:]) {
		return nil, ErrBadMagic
	}
	h := &Header{Version: binary.BigEndian.Uint32(payload[4:8])}
	if h.Version != FormatVersion {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedVersion, h.Version)
	}
	// Trailing payload bytes are tolerated for forward compatibility
	// (CIP extensibility: unknown HDR fields may be skipped). We write
	// exactly 8 bytes, matching both reference implementations.
	return h, nil
}
