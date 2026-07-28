package javacst

import "git.alberto.engineer/alberto/java-cst-go/language"

// Options controls one Java parse.
type Options struct {
	// Level selects a Java release and preview policy. Its zero value resolves
	// to Java 25 without preview features.
	Level language.Level
}

func (o Options) resolveLevel() (language.Level, bool) {
	if o.Level == (language.Level{}) {
		return language.Level{Release: language.Release25}, true
	}
	if !o.Level.Valid() {
		return language.Level{}, false
	}
	return o.Level, true
}
