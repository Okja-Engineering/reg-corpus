// Tests for the one sink every line regcorpus writes passes through
// (worklist round 3, §1): what it escapes, visibly, and what it keeps.
// Every character the sink escapes is written here as an escape, or from
// its code point, so no invisible character is in this file.
package sink_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/sink"
)

// The line and paragraph separators, U+2028 and U+2029, and how the sink
// writes them: as text, a backslash and u and four hex digits.
var (
	lineSep, paraSep         = string(rune(0x2028)), string(rune(0x2029))
	lineSepText, paraSepText = `\` + "u2028", `\` + "u2029"
)

// TestAWriteIsOneLine: each Write is one line, and everything in it that
// could end a line or move a terminal is written escaped, an LF too, but
// the one LF that ends it; the rest of the text, any language's, is kept.
func TestAWriteIsOneLine(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"a plain line", "1024.41: tree: equal\n", "1024.41: tree: equal\n"},
		{"no LF at the end", "1024.41", "1024.41"},
		{"an empty line", "\n", "\n"},
		{"text kept", "§ 1024.41 — “quoted” a b ſ é\n", "§ 1024.41 — “quoted” a b ſ é\n"},
		{"a backslash kept", `a\nb` + "\n", `a\nb` + "\n"},
		{"an LF inside", "x: y\n1024.41: wrote meta.yaml\n", `x: y\x0a1024.41: wrote meta.yaml` + "\n"},
		{"two LFs at the end", "a\n\n", `a\x0a` + "\n"},
		{"a CR", "a\rb\n", `a\x0db` + "\n"},
		{"CR LF", "a\r\n", `a\x0d` + "\n"},
		{"an escape sequence", "\x1b[2K\x1b[1Ax\n", `\x1b[2K\x1b[1Ax` + "\n"},
		{"a tab", "a\tb\n", `a\x09b` + "\n"},
		{"NUL", "a\x00b\n", `a\x00b` + "\n"},
		{"the last C0 control", "a\x1fb\n", `a\x1fb` + "\n"},
		{"DEL", "a\x7fb\n", `a\x7fb` + "\n"},
		{"a next line, C1", "a\u0085b\n", `a\u0085b` + "\n"},
		{"the first C1 control", "a\u0080b\n", `a\u0080b` + "\n"},
		{"a control sequence introducer, C1", "a\u009b2Kb\n", `a\u009b2Kb` + "\n"},
		{"the last C1 control", "a\u009fb\n", `a\u009fb` + "\n"},
		{"a line separator", "a" + lineSep + "b\n", "a" + lineSepText + "b\n"},
		{"a paragraph separator", "a" + paraSep + "b\n", "a" + paraSepText + "b\n"},
		{"a byte that isn't UTF-8", "a\x9bb\xffc\n", `a\x9bb\xffc` + "\n"},
		{"the first character past C1", "a b\n", "a b\n"},
		{"space and tilde", " ~\n", " ~\n"},
	} {
		var b bytes.Buffer
		w := sink.New(&b)
		n, err := w.Write([]byte(tc.in))
		if err != nil || n != len(tc.in) || b.String() != tc.want {
			t.Errorf("%s: Write(%q) = %d, %v, wrote %q; want %d, nil, %q", tc.name, tc.in, n, err, b.String(), len(tc.in), tc.want)
		}
	}
}

// writes records each Write it is given.
type writes struct{ got []string }

func (w *writes) Write(p []byte) (int, error) {
	w.got = append(w.got, string(p))
	return len(p), nil
}

// TestPrintWritesItsTextALineAtATime: regcorpus's own text of many lines,
// a help text, is written one line per Write, so the sink keeps each
// line's LF and writes the text byte for byte; to any other writer the
// bytes are the same.
func TestPrintWritesItsTextALineAtATime(t *testing.T) {
	const help = "usage: regcorpus x\n\nExit 0: ok.\n  --flag  does y\nno LF at the end"
	var lines writes
	sink.Print(&lines, help)
	if want := []string{"usage: regcorpus x\n", "\n", "Exit 0: ok.\n", "  --flag  does y\n", "no LF at the end"}; strings.Join(lines.got, "|") != strings.Join(want, "|") {
		t.Errorf("Print wrote %q; want %q", lines.got, want)
	}
	var b, plain bytes.Buffer
	sink.Print(sink.New(&b), help)
	sink.Print(&plain, help)
	if b.String() != help || plain.String() != help {
		t.Errorf("Print through the sink wrote %q, to a buffer %q; want %q both", b.String(), plain.String(), help)
	}
}
