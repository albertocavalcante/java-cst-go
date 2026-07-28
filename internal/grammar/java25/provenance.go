package java25

const (
	// SharedCSTVersion and SharedCSTCommit identify the hardened red/green
	// substrate used by the Java conversion pipeline.
	SharedCSTVersion = "v0.2.1-0.20260727192650-0dd634a2fbae"
	SharedCSTCommit  = "0dd634a2fbae8201cb1c521b7f05c3cc5bc6aac2"

	// RuntimeVersion and RuntimeCommit identify the pure-Go runtime used by
	// this generated grammar package.
	RuntimeVersion = "v0.1.0"
	RuntimeCommit  = "cf63d32385984f5accbd17557c0e26fcd281f61a"

	// GrammarBaseCommit identifies the upstream source before patch.json.
	GrammarBaseCommit = "e10607b45ff745f5f876bfa3e94fbcc6b44bdc11"
	// GrammarPatchSHA256 identifies the repository-owned grammar delta.
	GrammarPatchSHA256 = "c5aa5286759b7c6bee77fb2725efbff8b1ce3d985c45b38bb403a7072d7c5030"
	// GrammarRevision combines the immutable upstream base and local patch.
	GrammarRevision = GrammarBaseCommit + "+patch-sha256:" + GrammarPatchSHA256
)
