package java25

// CollapsedLeafKinds returns grammar nonterminals whose invisible children are
// collapsed by the selected runtime, so snapshots publish the nonterminal
// spelling as a source-bearing leaf. They need both node and token stable-kind
// identities because conversion classifies snapshot leaves as CST tokens.
func CollapsedLeafKinds() []string {
	return []string{
		"multiline_string_fragment",
	}
}
