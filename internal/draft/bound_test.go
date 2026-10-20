package draft

import "testing"

// The invisible characters the model must not smuggle into a field: a
// bidirectional override and isolates, a zero-width space, a byte order mark,
// a tag character, a NUL and the next-line control. The ZWJ in an emoji
// sequence, and in a multi-line field the line break and the tab, stay.
const invisible = "\u202e\u2066\u2067\u2068\u2069\u200b\ufeff\U000E0041\x00\u0085"

func TestLine_DropsInvisibleCharacters(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"each invisible character", "a" + invisible + "b", "ab"},
		{"a bidi override", "Launch\u202e", "Launch"},
		{"the isolates", "\u2066a\u2067b\u2068c\u2069", "abc"},
		{"a zero-width space", "Lau\u200bnch", "Launch"},
		{"a byte order mark", "\ufeffLaunch", "Launch"},
		{"a tag character", "Launch\U000E0041", "Launch"},
		{"a ZWJ emoji sequence is kept", "👩\u200d💻 Launch", "👩\u200d💻 Launch"},
		{"a line break is still one line", "a\nb\tc", "a b c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := line(tc.in, 80); got != tc.want {
				t.Errorf("line(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMulti_DropsInvisibleCharacters(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"each invisible character", "a" + invisible + "b", "ab"},
		{"a bidi override", "Ship it\u202e.", "Ship it."},
		{"a zero-width space", "Sh\u200bip", "Ship"},
		{"a tag character", "Ship\U000E0041", "Ship"},
		{"a ZWJ emoji sequence is kept", "👩\u200d💻", "👩\u200d💻"},
		{"a newline and a tab are kept", "One.\n\tTwo.", "One.\n\tTwo."},
		{"invisible only is empty", "\u200b\ufeff", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := multi(tc.in, 4000); got != tc.want {
				t.Errorf("multi(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
