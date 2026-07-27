package language_test

import (
	"strconv"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestReleasesAreConcreteAndContiguous(t *testing.T) {
	t.Parallel()

	for number := 8; number <= 26; number++ {
		release := language.Release(number)
		if !release.Valid() {
			t.Errorf("Release(%d).Valid() = false", number)
		}
		if got, want := release.String(), strconv.Itoa(number); got != want {
			t.Errorf("Release(%d).String() = %q, want %q", number, got, want)
		}
	}

	for _, release := range []language.Release{0, 7, 27, 255} {
		if release.Valid() {
			t.Errorf("Release(%d).Valid() = true", release)
		}
	}
}

func TestParseRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want language.Release
	}{
		{text: "1.8", want: language.Release8},
		{text: "8", want: language.Release8},
		{text: "11", want: language.Release11},
		{text: "17", want: language.Release17},
		{text: "21", want: language.Release21},
		{text: "25", want: language.Release25},
		{text: "26", want: language.Release26},
	}
	for _, test := range tests {
		got, err := language.ParseRelease(test.text)
		if err != nil {
			t.Errorf("ParseRelease(%q): %v", test.text, err)
			continue
		}
		if got != test.want {
			t.Errorf("ParseRelease(%q) = %v, want %v", test.text, got, test.want)
		}
	}
}

func TestParseReleaseRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"", "1.7", "7", "27", "latest", " 21"} {
		if got, err := language.ParseRelease(text); err == nil {
			t.Errorf("ParseRelease(%q) = %v, nil; want error", text, got)
		}
	}
}

func TestLevelString(t *testing.T) {
	t.Parallel()

	if got, want := (language.Level{Release: language.Release21}).String(), "21"; got != want {
		t.Fatalf("stable level String() = %q, want %q", got, want)
	}
	if got, want := (language.Level{
		Release: language.Release21,
		Preview: true,
	}).String(), "21-preview"; got != want {
		t.Fatalf("preview level String() = %q, want %q", got, want)
	}
	if got, want := (language.Level{}).String(), "invalid"; got != want {
		t.Fatalf("zero level String() = %q, want %q", got, want)
	}
}
