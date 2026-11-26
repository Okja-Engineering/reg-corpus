package sink

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Writer is stdout or stderr as regcorpus writes them: each Write one line.
type Writer struct{ w io.Writer }

// New returns the Writer that writes w's lines.
func New(w io.Writer) *Writer { return &Writer{w} }

// Write writes p as one line, in one Write to the stream: escaped (escape)
// but for an LF that ends it. It returns len(p) once the stream has taken
// the line.
func (s *Writer) Write(p []byte) (int, error) {
	line, end := string(p), ""
	if strings.HasSuffix(line, "\n") {
		line, end = line[:len(line)-1], "\n"
	}
	if _, err := io.WriteString(s.w, escape(line)+end); err != nil {
		return 0, err
	}
	return len(p), nil
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
