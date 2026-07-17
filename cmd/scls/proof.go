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
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/blinklabs-io/go-scls"
	"github.com/spf13/cobra"
)

const proofEnvelopeVersion = 1

// proofMagic marks the binary envelope form; JSON envelopes never start
// with it, so verify can auto-detect.
var proofMagic = [4]byte{'S', 'C', 'L', 'P'}

// proofEnvelope is the self-contained, portable proof artifact. It carries
// everything VerifyProof needs plus the root the proof binds to. Hex/base64
// fields keep the JSON form human-inspectable.
type proofEnvelope struct {
	Version   int    `json:"version"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`   // hex
	Value     string `json:"value"` // hex
	Root      string `json:"root"`  // hex
	Proof     string `json:"proof"` // base64(Proof.MarshalBinary)
}

func newProofCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proof",
		Short: "Generate and verify Merkle inclusion proofs",
	}
	cmd.AddCommand(newProofGenerateCmd(), newProofVerifyCmd())
	return cmd
}

func newProofGenerateCmd() *cobra.Command {
	var outPath, format string
	cmd := &cobra.Command{
		Use:   "generate <file> <namespace> <key-hex>",
		Short: "Generate a portable inclusion proof for a key",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := hex.DecodeString(args[2])
			if err != nil {
				return fmt.Errorf("invalid key hex: %w", err)
			}
			f, err := os.Open(args[0]) //nolint:gosec // user-specified input path
			if err != nil {
				return err
			}
			defer f.Close()
			fi, err := f.Stat()
			if err != nil {
				return err
			}
			s, err := scls.Open(f, fi.Size())
			if err != nil {
				return err
			}
			value, proof, err := s.Prove(args[1], key)
			if err != nil {
				return err
			}
			pb, err := proof.MarshalBinary()
			if err != nil {
				return err
			}
			root := s.Manifest().RootHash
			env := proofEnvelope{
				Version:   proofEnvelopeVersion,
				Namespace: args[1],
				Key:       hex.EncodeToString(key),
				Value:     hex.EncodeToString(value),
				Root:      hex.EncodeToString(root[:]),
				Proof:     base64.StdEncoding.EncodeToString(pb),
			}
			data, err := marshalEnvelope(env, format)
			if err != nil {
				return err
			}
			return writeEnvelope(cmd, outPath, format, data)
		},
	}
	cmd.Flags().StringVarP(&outPath, "out", "o", "-", "output path (- for stdout)")
	cmd.Flags().StringVar(&format, "format", "json", "proof format: json|binary")
	return cmd
}

func newProofVerifyCmd() *cobra.Command {
	var expectedRoot string
	cmd := &cobra.Command{
		Use:   "verify <proof-file>",
		Short: "Verify a portable inclusion proof",
		Long: "Verify a portable inclusion proof.\n\n" +
			"The embedded root alone is self-referential; for real trust pass " +
			"--root with a value obtained from a source you trust (e.g. `scls " +
			"verify`/`info` on the authentic file).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := readInput(args[0])
			if err != nil {
				return err
			}
			env, err := unmarshalEnvelope(raw)
			if err != nil {
				return err
			}
			key, value, root, err := env.decodeFields()
			if err != nil {
				return err
			}
			if expectedRoot != "" && !strings.EqualFold(expectedRoot, env.Root) {
				return fmt.Errorf("proof root %s does not match --root %s", env.Root, expectedRoot)
			}
			pb, err := base64.StdEncoding.DecodeString(env.Proof)
			if err != nil {
				return fmt.Errorf("invalid proof base64: %w", err)
			}
			proof, err := scls.UnmarshalProofBinary(pb)
			if err != nil {
				return err
			}
			if err := scls.VerifyProof(root, env.Namespace, key, value, proof); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"ok": true, "root": env.Root,
					"namespace": env.Namespace, "key": env.Key,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK  ns=%s key=%s root=%s\n",
				env.Namespace, env.Key, env.Root)
			return nil
		},
	}
	cmd.Flags().StringVar(&expectedRoot, "root", "",
		"pin an externally-trusted root (hex) the proof must match")
	return cmd
}

// decodeFields decodes and validates the envelope's binary fields.
func (e proofEnvelope) decodeFields() (key, value []byte, root scls.Hash, err error) {
	key, err = hex.DecodeString(e.Key)
	if err != nil {
		return nil, nil, root, fmt.Errorf("invalid key hex: %w", err)
	}
	value, err = hex.DecodeString(e.Value)
	if err != nil {
		return nil, nil, root, fmt.Errorf("invalid value hex: %w", err)
	}
	rb, err := hex.DecodeString(e.Root)
	if err != nil {
		return nil, nil, root, fmt.Errorf("invalid root hex: %w", err)
	}
	if len(rb) != scls.HashSize {
		return nil, nil, root, fmt.Errorf("root must be %d bytes, got %d", scls.HashSize, len(rb))
	}
	copy(root[:], rb)
	return key, value, root, nil
}

func marshalEnvelope(env proofEnvelope, format string) ([]byte, error) {
	if !utf8.ValidString(env.Namespace) {
		return nil, errors.New("proof namespace is not valid UTF-8")
	}
	switch format {
	case "json":
		return json.MarshalIndent(env, "", "  ")
	case "binary":
		return marshalEnvelopeBinary(env)
	default:
		return nil, fmt.Errorf("unknown proof format %q (want json|binary)", format)
	}
}

// marshalEnvelopeBinary frames the envelope as (RECONCILIATION E8):
// magic "SCLP" | version u8 | (u32 len || bytes) x { ns, key, value, root, proof }.
func marshalEnvelopeBinary(env proofEnvelope) ([]byte, error) {
	key, err := hex.DecodeString(env.Key)
	if err != nil {
		return nil, err
	}
	value, err := hex.DecodeString(env.Value)
	if err != nil {
		return nil, err
	}
	root, err := hex.DecodeString(env.Root)
	if err != nil {
		return nil, err
	}
	proof, err := base64.StdEncoding.DecodeString(env.Proof)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(proofMagic[:])
	b.WriteByte(byte(env.Version)) //nolint:gosec // proofEnvelopeVersion is a small constant
	for _, field := range [][]byte{[]byte(env.Namespace), key, value, root, proof} {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(field))) //nolint:gosec // CLI-bounded
		b.Write(l[:])
		b.Write(field)
	}
	return b.Bytes(), nil
}

func unmarshalEnvelope(raw []byte) (proofEnvelope, error) {
	var (
		env proofEnvelope
		err error
	)
	if len(raw) >= 4 && bytes.Equal(raw[:4], proofMagic[:]) {
		env, err = unmarshalEnvelopeBinary(raw)
	} else if err = json.Unmarshal(raw, &env); err != nil {
		err = fmt.Errorf("parse proof: %w", err)
	}
	if err != nil {
		return proofEnvelope{}, err
	}
	// Reject unknown/missing versions rather than silently treating them as
	// v1: a future envelope layout must not be misread through the v1 decoder.
	if env.Version != proofEnvelopeVersion {
		return proofEnvelope{}, fmt.Errorf(
			"unsupported proof envelope version %d (want %d)",
			env.Version, proofEnvelopeVersion)
	}
	return env, nil
}

func unmarshalEnvelopeBinary(raw []byte) (proofEnvelope, error) {
	r := &binReader{b: raw, off: 4} // skip magic
	ver := r.u8()
	ns := r.field()
	key := r.field()
	value := r.field()
	root := r.field()
	proof := r.field()
	if r.err != nil {
		return proofEnvelope{}, r.err
	}
	if !utf8.Valid(ns) {
		return proofEnvelope{}, errors.New("proof namespace is not valid UTF-8")
	}
	if r.off != len(raw) {
		return proofEnvelope{}, errors.New("trailing bytes in binary proof")
	}
	return proofEnvelope{
		Version:   int(ver),
		Namespace: string(ns),
		Key:       hex.EncodeToString(key),
		Value:     hex.EncodeToString(value),
		Root:      hex.EncodeToString(root),
		Proof:     base64.StdEncoding.EncodeToString(proof),
	}, nil
}

// binReader is a bounds-checked cursor over a binary envelope; first error sticks.
type binReader struct {
	b   []byte
	off int
	err error
}

func (r *binReader) u8() byte {
	if r.err != nil || r.off+1 > len(r.b) {
		r.err = errors.New("truncated binary proof")
		return 0
	}
	v := r.b[r.off]
	r.off++
	return v
}

func (r *binReader) field() []byte {
	if r.err != nil || r.off+4 > len(r.b) {
		r.err = errors.New("truncated binary proof")
		return nil
	}
	n := int(binary.BigEndian.Uint32(r.b[r.off : r.off+4]))
	r.off += 4
	if n < 0 || r.off+n > len(r.b) {
		r.err = errors.New("truncated binary proof field")
		return nil
	}
	f := r.b[r.off : r.off+n]
	r.off += n
	return f
}

// writeEnvelope writes data to outPath ("-" for stdout). For the JSON format
// it ensures a trailing newline for shell-friendliness; the binary format is
// a length-framed blob and must be written byte-exact, so no padding is added
// (a stray trailing byte would make unmarshalEnvelopeBinary reject it as
// trailing garbage).
func writeEnvelope(cmd *cobra.Command, outPath, format string, data []byte) error {
	if outPath == "-" {
		w := cmd.OutOrStdout()
		if _, err := w.Write(data); err != nil {
			return err
		}
		if format == "json" && (len(data) == 0 || data[len(data)-1] != '\n') {
			_, err := w.Write([]byte{'\n'})
			return err
		}
		return nil
	}
	return os.WriteFile(outPath, data, 0o600)
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path) //nolint:gosec // user-specified input path
}
