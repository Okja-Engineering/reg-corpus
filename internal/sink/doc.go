// Package sink is where every line regcorpus writes to stdout or stderr
// passes (worklist round 3, §1; ADR 0014). The output is saved as evidence
// (fetch --verify's, stage 01), and a line may carry eCFR's or govinfo's
// bytes, in a value or in an error: so no byte may forge a line of it or
// move the terminal it is read in, whatever a call site does.
//
//   - cmd/regcorpus gives every command a Writer for stdout and one for
//     stderr (New), never the files, so no command writes past one. (verify
//     is given the stdout file to test it for a terminal, and writes only to
//     its stderr Writer.)
//   - A Writer takes each Write as one line, as fmt.Fprintf and
//     fmt.Fprintln make it, and writes it with every C0 control but the one
//     LF that ends it, DEL, every C1 control (U+0085 among them), U+2028 and
//     U+2029, and any byte that isn't UTF-8, escaped visibly: \xNN for a
//     byte, \uNNNN for a C1 control or a separator. So an LF inside a line,
//     a CR or an escape sequence is written as text, on its line. Nothing
//     else changes: a line without them is written byte for byte.
//   - regcorpus's own text of many lines, a help text, is a Text, which
//     Print writes one line per Write; a Text is a constant, or a value
//     converted to one where it is written.
//
// The per-site quoting of upstream values (strconv.Quote, %q) stays where
// it is, and keeps a value on its line in a form that reads back; this is
// the guarantee that holds whatever a call site prints.
package sink
