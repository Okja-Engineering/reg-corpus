package sink

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Writer is stdout or stderr as regcorpus writes them: each Write one line.
// It keeps the first error its stream gave (Err).
type Writer struct {
	w   io.Writer
	err error
}

// New returns the Writer that writes w's lines.
func New(w io.Writer) *Writer { return &Writer{w: w} }

// Write writes p as one line, in one Write to the stream: escaped (escape)
// but for an LF that ends it. It returns len(p) once the stream has taken
// the line, and otherwise the stream's error, which it keeps if it is the
// first (Err).
func (s *Writer) Write(p []byte) (int, error) {
	line, end := string(p), ""
	if strings.HasSuffix(line, "\n") {
		line, end = line[:len(line)-1], "\n"
	}
	if _, err := io.WriteString(s.w, escape(line)+end); err != nil {
		if s.err == nil {
			s.err = err
		}
		return 0, err
	}
	return len(p), nil
}

// Err is the first error the stream gave a Write, nil if none has: so a
// line a caller wrote without looking at the error (fmt.Fprintf, Print) is
// still known to be lost, and cmd/regcorpus decides once, for every
// command, that a stdout it couldn't write is exit 2 (round 1 of PR #11,
// §3).
func (s *Writer) Err() error { return s.err }

// JSON writes v to w as one line, in one Write: v as encoding/json encodes
// it, nothing HTML-escaped ("<", ">" and "&" as they are), then an LF.
// encoding/json escapes each C0 control, U+2028 and U+2029, and writes
// nothing but ASCII outside a string; DEL, which it leaves and a Writer
// would write as \x7f, which is no JSON escape, is written \u007f here. A
// Writer writes a C1 control as \u0080 to \u009f, a JSON escape of itself.
// So through a Writer the line is valid JSON that decodes to v, a string
// in it to the same valid UTF-8, and holds nothing that could end a line or
// move a terminal (round 1 of PR #11, §4). A value encoding/json can't
// encode is its error, and nothing is written. The Write's error is not
// returned, as Print returns none: a Writer keeps it (Err), as for every
// line, and cmd/regcorpus decides on it once (§3).
func JSON(w io.Writer, v any) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	w.Write(bytes.ReplaceAll(b.Bytes(), []byte{0x7f}, []byte(`\u007f`)))
	return nil
}

// escape is s with every character that could end a line or move a
// terminal written as text: each C0 control, an LF too, DEL, and each byte
// that isn't UTF-8 as \xNN; each C1 control, U+2028 and U+2029 as \uNNNN.
func escape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1, r < 0x20, r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case 0x80 <= r && r <= 0x9f, r == 0x2028, r == 0x2029:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String()
}

// Text is regcorpus's own text of one or more lines, a help text: a
// constant, or a value converted to one where it is written.
type Text string

// Print writes t to w one line per Write, each with its LF, so a Writer
// writes it byte for byte.
func Print(w io.Writer, t Text) {
	for _, l := range strings.SplitAfter(string(t), "\n") {
		io.WriteString(w, l)
	}
}
