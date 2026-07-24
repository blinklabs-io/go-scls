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

package version

import "testing"

func TestVersionString(t *testing.T) {
	originalVersion, originalCommitHash := Version, CommitHash
	t.Cleanup(func() {
		Version, CommitHash = originalVersion, originalCommitHash
	})

	tests := []struct {
		name, version, commit, want string
	}{
		{name: "release", version: "v1.2.3", commit: "abc123", want: "v1.2.3 (commit abc123)"},
		{name: "release without commit", version: "v1.2.3", want: "v1.2.3"},
		{name: "development commit", commit: "abc123", want: "devel (commit abc123)"},
		{name: "development", want: "devel"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			Version, CommitHash = tc.version, tc.commit
			if got := VersionString(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
