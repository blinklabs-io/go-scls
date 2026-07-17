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
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/blinklabs-io/go-scls"
	"github.com/spf13/cobra"
)

type keyChange struct {
	Op  string `json:"op"`  // "+", "-", "~"
	Key string `json:"key"` // hex
}

type nsDiff struct {
	Name       string      `json:"name"`
	Status     string      `json:"status"` // added|removed|changed|unchanged
	OldEntries uint64      `json:"oldEntries"`
	NewEntries uint64      `json:"newEntries"`
	KeyChanges []keyChange `json:"keyChanges,omitempty"`
}

type diffReport struct {
	OldSlot    uint64   `json:"oldSlot"`
	NewSlot    uint64   `json:"newSlot"`
	Namespaces []nsDiff `json:"namespaces"`
}

func newDiffCmd() *cobra.Command {
	var detailed bool
	cmd := &cobra.Command{
		Use:   "diff <old.scls> <new.scls>",
		Short: "Compare two SCLS files",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldM, err := loadManifest(args[0])
			if err != nil {
				return err
			}
			newM, err := loadManifest(args[1])
			if err != nil {
				return err
			}
			report := buildDiff(oldM, newM)
			if detailed {
				if err := fillDetailed(args[0], args[1], report); err != nil {
					return err
				}
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			return renderDiff(cmd, report, detailed, asJSON)
		},
	}
	cmd.Flags().BoolVar(&detailed, "detailed", false,
		"list per-key added/removed/changed entries")
	return cmd
}

func buildDiff(oldM, newM *scls.Manifest) *diffReport {
	rep := &diffReport{OldSlot: oldM.SlotNo, NewSlot: newM.SlotNo}
	oldNS := indexNS(oldM)
	newNS := indexNS(newM)
	for _, name := range unionSorted(oldNS, newNS) {
		o, inOld := oldNS[name]
		n, inNew := newNS[name]
		d := nsDiff{Name: name}
		switch {
		case inOld && !inNew:
			d.Status, d.OldEntries = "removed", o.EntriesCount
		case !inOld && inNew:
			d.Status, d.NewEntries = "added", n.EntriesCount
		case o.Digest == n.Digest:
			d.Status, d.OldEntries, d.NewEntries = "unchanged", o.EntriesCount, n.EntriesCount
		default:
			d.Status, d.OldEntries, d.NewEntries = "changed", o.EntriesCount, n.EntriesCount
		}
		rep.Namespaces = append(rep.Namespaces, d)
	}
	return rep
}

func indexNS(m *scls.Manifest) map[string]scls.NamespaceInfo {
	out := make(map[string]scls.NamespaceInfo, len(m.Namespaces))
	for _, ns := range m.Namespaces {
		out[ns.Name] = ns
	}
	return out
}

func unionSorted(a, b map[string]scls.NamespaceInfo) []string {
	seen := map[string]struct{}{}
	var names []string
	for _, m := range []map[string]scls.NamespaceInfo{a, b} {
		for k := range m {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				names = append(names, k)
			}
		}
	}
	sort.Strings(names)
	return names
}

// fillDetailed populates KeyChanges for changed/added/removed namespaces by
// merge-joining the two files' entry streams in lockstep. Both files emit
// entries in ascending (namespace, key) order, so the join needs only one
// CHUNK buffered per side at a time — it never materializes whole namespaces
// (keys and values) the way a collect-then-diff approach would.
func fillDetailed(oldPath, newPath string, rep *diffReport) error {
	want := map[string]bool{}
	byName := map[string]*nsDiff{}
	for i := range rep.Namespaces {
		d := &rep.Namespaces[i]
		if d.Status != "unchanged" {
			want[d.Name] = true
			byName[d.Name] = d
		}
	}
	if len(want) == 0 {
		return nil
	}
	oldC, err := newEntryCursor(oldPath, want)
	if err != nil {
		return err
	}
	defer oldC.close()
	newC, err := newEntryCursor(newPath, want)
	if err != nil {
		return err
	}
	defer newC.close()

	// Every surfaced entry belongs to a wanted namespace, so byName[ns] is
	// always non-nil here.
	emit := func(ns string, kc keyChange) {
		byName[ns].KeyChanges = append(byName[ns].KeyChanges, kc)
	}
	for oldC.ok && newC.ok {
		o, n := oldC.cur, newC.cur
		switch cmp := compareItems(o, n); {
		case cmp < 0:
			emit(o.ns, keyChange{"-", hex.EncodeToString(o.key)})
			oldC.advance()
		case cmp > 0:
			emit(n.ns, keyChange{"+", hex.EncodeToString(n.key)})
			newC.advance()
		default:
			if !bytes.Equal(o.value, n.value) {
				emit(o.ns, keyChange{"~", hex.EncodeToString(o.key)})
			}
			oldC.advance()
			newC.advance()
		}
	}
	for oldC.ok {
		emit(oldC.cur.ns, keyChange{"-", hex.EncodeToString(oldC.cur.key)})
		oldC.advance()
	}
	for newC.ok {
		emit(newC.cur.ns, keyChange{"+", hex.EncodeToString(newC.cur.key)})
		newC.advance()
	}
	if oldC.err != nil {
		return oldC.err
	}
	return newC.err
}

// cursorItem is one entry surfaced by an entryCursor.
type cursorItem struct {
	ns    string
	key   []byte
	value []byte
}

// compareItems orders items by (namespace, key), the order both files stream.
func compareItems(a, b cursorItem) int {
	if a.ns != b.ns {
		if a.ns < b.ns {
			return -1
		}
		return 1
	}
	return bytes.Compare(a.key, b.key)
}

// entryCursor streams the entries of the wanted namespaces from an SCLS file in
// ascending (namespace, key) order, buffering at most one CHUNK at a time. cur
// holds the current item while ok is true; err records a read failure once the
// cursor stops.
type entryCursor struct {
	sr      *scls.Reader
	closeFn func() error
	want    map[string]bool
	buf     []scls.Entry // remaining entries of the current chunk
	ns      string       // namespace of buf
	cur     cursorItem
	ok      bool
	err     error
}

func newEntryCursor(path string, want map[string]bool) (*entryCursor, error) {
	f, err := os.Open(path) //nolint:gosec // user-specified input path
	if err != nil {
		return nil, err
	}
	sr, err := scls.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	c := &entryCursor{sr: sr, closeFn: f.Close, want: want}
	return primeEntryCursor(c)
}

func primeEntryCursor(c *entryCursor) (*entryCursor, error) {
	c.advance()
	if c.err != nil {
		err := c.err
		_ = c.close()
		return nil, err
	}
	return c, nil
}

func (c *entryCursor) advance() {
	if c.err != nil {
		c.ok = false
		return
	}
	for len(c.buf) == 0 {
		chunk, err := c.sr.Next()
		if errors.Is(err, io.EOF) {
			c.ok = false
			return
		}
		if err != nil {
			c.err = err
			c.ok = false
			return
		}
		if c.want[chunk.Namespace] && len(chunk.Entries) > 0 {
			c.ns = chunk.Namespace
			c.buf = chunk.Entries
		}
	}
	e := c.buf[0]
	c.buf = c.buf[1:]
	c.cur = cursorItem{ns: c.ns, key: e.Key, value: e.Value}
	c.ok = true
}

func (c *entryCursor) close() error {
	if c.closeFn != nil {
		return c.closeFn()
	}
	return nil
}

func renderDiff(cmd *cobra.Command, rep *diffReport, detailed, asJSON bool) error {
	if asJSON {
		return printJSON(cmd.OutOrStdout(), rep)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "slot: %d -> %d\n", rep.OldSlot, rep.NewSlot)
	for _, d := range rep.Namespaces {
		fmt.Fprintf(w, "%-10s %s (entries %d -> %d)\n",
			d.Name, d.Status, d.OldEntries, d.NewEntries)
		if detailed {
			for _, kc := range d.KeyChanges {
				fmt.Fprintf(w, "    %s %s\n", kc.Op, kc.Key)
			}
		}
	}
	return nil
}
