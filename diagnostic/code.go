package diagnostic

// Code is a stable machine-readable Java diagnostic identifier.
type Code string

const (
	// CodeInvalidLanguageLevel reports invalid or unsupported parse
	// configuration.
	CodeInvalidLanguageLevel Code = "JAV0001"

	// CodeInvalidUTF8 reports an invalid UTF-8 byte in raw Java source.
	CodeInvalidUTF8 Code = "JAV1001"
	// CodeInvalidUnicodeEscape reports a malformed Java Unicode escape or an
	// isolated UTF-16 surrogate escape.
	CodeInvalidUnicodeEscape Code = "JAV1002"
	// CodeUnterminatedComment reports an unterminated block comment.
	CodeUnterminatedComment Code = "JAV1003"
	// CodeUnterminatedLiteral reports an unterminated string, character
	// literal, or text block.
	CodeUnterminatedLiteral Code = "JAV1004"

	// CodeUnexpectedToken reports unexpected syntax retained through recovery.
	CodeUnexpectedToken Code = "JAV2001"
	// CodeMissingToken reports an expected zero-width token.
	CodeMissingToken Code = "JAV2002"

	// CodeFeatureUnavailable reports syntax unavailable at the selected
	// release.
	CodeFeatureUnavailable Code = "JAV3001"
	// CodePreviewDisabled reports recognized preview syntax when preview is
	// disabled.
	CodePreviewDisabled Code = "JAV3002"
	// CodeFeatureWithdrawn reports syntax from a withdrawn preview feature.
	CodeFeatureWithdrawn Code = "JAV3003"
	// CodeBackendLimit reports a known grammar or backend limitation.
	CodeBackendLimit Code = "JAV3004"
	// CodeFeatureRestriction reports syntax that uses an enabled feature but
	// violates a release-specific restriction of that feature generation.
	CodeFeatureRestriction Code = "JAV3005"

	// CodeResourceLimit reports a configured source, node, or depth limit.
	CodeResourceLimit Code = "JAV9001"
)
