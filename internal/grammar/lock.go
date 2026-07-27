// Package grammar verifies the exact runtime and Java grammar inputs used by
// the M0 backend spike.
package grammar

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

//go:embed grammar.lock.json
var lockFiles embed.FS

// Lock records immutable parser runtime and Java grammar provenance.
type Lock struct {
	Runtime RuntimeLock `json:"runtime"`
	Grammar GrammarLock `json:"grammar"`
}

// RuntimeLock identifies the pure-Go tree-sitter runtime module.
type RuntimeLock struct {
	Module    string `json:"module"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	ModuleSum string `json:"moduleSum"`
}

// GrammarLock identifies the Java grammar and generated blob.
type GrammarLock struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	BlobSHA256 string `json:"blobSHA256"`
}

// Load reads and validates the checked-in grammar lock.
func Load() (Lock, error) {
	data, err := lockFiles.ReadFile("grammar.lock.json")
	if err != nil {
		return Lock{}, fmt.Errorf("read embedded grammar lock: %w", err)
	}

	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return Lock{}, fmt.Errorf("decode grammar lock: %w", err)
	}
	if err := lock.Validate(); err != nil {
		return Lock{}, err
	}

	return lock, nil
}

// Validate checks that all required provenance and integrity fields exist.
func (l Lock) Validate() error {
	switch {
	case l.Runtime.Module == "":
		return errors.New("grammar lock: runtime module is empty")
	case l.Runtime.Version == "":
		return errors.New("grammar lock: runtime version is empty")
	case len(l.Runtime.Commit) != 40:
		return errors.New("grammar lock: runtime commit is not a 40-character hash")
	case l.Runtime.ModuleSum == "":
		return errors.New("grammar lock: runtime module sum is empty")
	case l.Grammar.Repository == "":
		return errors.New("grammar lock: grammar repository is empty")
	case len(l.Grammar.Commit) != 40:
		return errors.New("grammar lock: grammar commit is not a 40-character hash")
	}

	digest, err := hex.DecodeString(l.Grammar.BlobSHA256)
	if err != nil {
		return fmt.Errorf("grammar lock: decode blob SHA-256: %w", err)
	}
	if len(digest) != sha256.Size {
		return fmt.Errorf(
			"grammar lock: blob SHA-256 is %d bytes, want %d",
			len(digest),
			sha256.Size,
		)
	}

	return nil
}

// VerifyBlob checks raw Java grammar bytes against the locked SHA-256.
func (l Lock) VerifyBlob(blob []byte) error {
	got := sha256.Sum256(blob)
	if hex.EncodeToString(got[:]) != l.Grammar.BlobSHA256 {
		return fmt.Errorf(
			"java grammar blob SHA-256 = %x, want %s",
			got,
			l.Grammar.BlobSHA256,
		)
	}

	return nil
}
