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

import "github.com/spf13/cobra"

// newRootCmd builds a fresh root command tree. A fresh tree per call keeps the
// command testable (no shared global flag state between invocations).
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "scls",
		Short:         "Inspect and verify Cardano SCLS ledger-state files",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("json", false, "emit output as JSON")
	root.AddCommand(newVersionCmd(), newVerifyCmd(), newInfoCmd(),
		newLsCmd(), newGetCmd(), newProofCmd(), newDiffCmd())
	return root
}
