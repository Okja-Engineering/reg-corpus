// Tests for the one sink every line regcorpus writes passes through
// (worklist round 3, §1): what it escapes, visibly, and what it keeps.
// Every character the sink escapes is written here as an escape, or from
// its code point, so no invisible character is in this file.
package sink_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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

// failing fails each Write from the after-th on (counting from 0), each with
// its own error.
type failing struct {
	after, n int
}

func (f *failing) Write(p []byte) (int, error) {
	defer func() { f.n++ }()
	if f.n >= f.after {
		return 0, fmt.Errorf("write %d failed", f.n)
	}
	return len(p), nil
}

// TestWriterKeepsItsFirstError: a Writer keeps the first error its stream
// gave (Err), whichever call made the Write and whether or not that call
// looked at the error, so cmd/regcorpus decides once, for every command,
// that output that wasn't written is not exit 0 (round 1, §3). Err is nil
// until a Write fails, and a later Write, failing or not, never changes it.
func TestWriterKeepsItsFirstError(t *testing.T) {
	w := sink.New(&failing{after: 2})
	fmt.Fprintln(w, "one")
	fmt.Fprintln(w, "two")
	if err := w.Err(); err != nil {
		t.Fatalf("Err after two written lines: %v; want nil", err)
	}
	if _, err := w.Write([]byte("three\n")); err == nil {
		t.Fatal("Write to a failing stream: no error")
	}
	fmt.Fprintln(w, "four")
	sink.Print(w, "five\n")
	if err := w.Err(); err == nil || err.Error() != "write 2 failed" {
		t.Errorf("Err: %v; want the first failure, write 2 failed", err)
	}
	if err := sink.New(&bytes.Buffer{}).Err(); err != nil {
		t.Errorf("Err of a Writer never written: %v", err)
	}
}

// jsonCases are the output hunter's (PR #11, round 1): every C0 control,
// DEL, every C1 control, U+2028, U+2029, U+FFFD, U+FEFF, U+200B, a
// backslash, a quotation mark, "<>&", the text \x7f and \u0085 as six
// characters each, an emoji and a no-break space, each written from its
// code point.
func jsonCases() []string {
	var cases []string
	for r := rune(0); r < 0x20; r++ {
		cases = append(cases, string(r))
	}
	cases = append(cases, "\x7f")
	for r := rune(0x80); r <= 0x9f; r++ {
		cases = append(cases, string(r))
	}
	return append(cases, lineSep, paraSep, string(rune(0xfffd)), string(rune(0xfeff)), string(rune(0x200b)),
		`\`, `"`, "<>&", `\`+"x7f", `\`+"u0085", string(rune(0x1f600)), string(rune(0xa0)))
}

// rawControl says where s, as a stream holds it, has a character that
// could end a line or move a terminal: a C0 control but LF, DEL, a C1
// control, U+2028 or U+2029, or a byte that isn't UTF-8; "" when none.
func rawControl(s string) string {
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 || r < 0x20 && r != '\n' || r == 0x7f || 0x80 <= r && r <= 0x9f || r == 0x2028 || r == 0x2029 {
			return fmt.Sprintf("%q at byte %d", s[i:i+n], i)
		}
		i += n
	}
	return ""
}

// TestJSONIsOneLineThatDecodes: JSON writes a value through the sink as one
// line, in one Write, of JSON that decodes to the value, for each of the
// output hunter's 77 cases inside a string (round 1, §4): the stream holds
// no character that could end a line or move a terminal, and nothing is
// HTML-escaped ("<>&" as they are).
func TestJSONIsOneLineThatDecodes(t *testing.T) {
	cases := jsonCases()
	if len(cases) != 77 {
		t.Fatalf("%d cases; want the output hunter's 77", len(cases))
	}
	type value struct {
		Text string   `json:"text"`
		List []string `json:"list"`
	}
	for _, c := range cases {
		in := value{"a" + c + "b", []string{c}}
		var lines writes
		if err := sink.JSON(sink.New(&lines), in); err != nil {
			t.Errorf("JSON(%q): %v", c, err)
			continue
		}
		if len(lines.got) != 1 {
			t.Errorf("JSON(%q): %d Writes %q; want one", c, len(lines.got), lines.got)
			continue
		}
		line := lines.got[0]
		var out value
		if err := json.Unmarshal([]byte(line), &out); err != nil || out.Text != in.Text || len(out.List) != 1 || out.List[0] != c {
			t.Errorf("JSON(%q) wrote %q, which decodes to %+v, %v; want %+v", c, line, out, err, in)
		}
		if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
			t.Errorf("JSON(%q) wrote %q; want one line", c, line)
		}
		if at := rawControl(line); at != "" {
			t.Errorf("JSON(%q) wrote %q, holding %s", c, line, at)
		}
	}
	var lines writes
	if err := sink.JSON(sink.New(&lines), value{Text: "a<b>&c"}); err != nil || strings.Join(lines.got, "") != `{"text":"a<b>&c","list":null}`+"\n" {
		t.Errorf("JSON wrote %q, %v; want <, > and & as they are", lines.got, err)
	}
}

// TestJSONWritesNothingItCantEncode: a value encoding/json can't encode is
// an error, and nothing is written.
func TestJSONWritesNothingItCantEncode(t *testing.T) {
	var lines writes
	if err := sink.JSON(sink.New(&lines), map[string]any{"f": func() {}}); err == nil || len(lines.got) != 0 {
		t.Errorf("JSON of a func: %v, wrote %q; want an error and nothing written", err, lines.got)
	}
}
