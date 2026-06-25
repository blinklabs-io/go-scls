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

import "errors"

var (
	ErrBadMagic               = errors.New("scls: bad magic bytes")
	ErrInvalidHeader          = errors.New("scls: invalid header record")
	ErrUnsupportedVersion     = errors.New("scls: unsupported format version")
	ErrUnsupportedChunkFormat = errors.New("scls: unsupported chunk format")
	ErrTruncatedRecord        = errors.New("scls: truncated record")
	ErrTruncatedChunk         = errors.New("scls: truncated chunk payload")
	ErrTrailingChunkData      = errors.New("scls: trailing bytes in chunk payload")
	ErrRecordTooLarge         = errors.New("scls: record size exceeds limit")
	ErrClosed                 = errors.New("scls: writer is closed")
	ErrEntryOrder             = errors.New("scls: entry keys not in strictly ascending order")
	ErrNamespaceOrder         = errors.New("scls: namespaces not in strictly ascending order")
	ErrChunkSequence          = errors.New("scls: chunk sequence numbers not strictly increasing")
	ErrKeyLength              = errors.New("scls: inconsistent key length within namespace")
	ErrHashMismatch           = errors.New("scls: hash mismatch")
	ErrCountMismatch          = errors.New("scls: count mismatch")
	ErrMissingHeader          = errors.New("scls: missing or misplaced HDR record")
	ErrMissingManifest        = errors.New("scls: missing MANIFEST record")
	ErrInvalidManifest        = errors.New("scls: invalid MANIFEST record")
	ErrUnexpectedRecord       = errors.New("scls: unexpected record after manifest")
	ErrNotFound               = errors.New("scls: entry or namespace not found")
	ErrProofMismatch          = errors.New("scls: proof does not verify against root")
)
