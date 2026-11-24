package anchors

import (
	"strings"
	"unicode"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"golang.org/x/text/unicode/norm"
)

// Normalize returns s's normalized text (spec "Definitions"; ADR 0013), in
// this order (N5):
//  1. Unicode NFC (not NFKC), by golang.org/x/text/unicode/norm.
//  2. The four curly quotes U+2018 and U+2019 to ', U+201C and U+201D to "
//     (N3), and the en and em dashes U+2013 and U+2014 to - (N4). Every
//     other character is left as it is: U+201A, U+201B, U+201E, U+201F,
//     primes and guillemets, and U+2010 to U+2012, U+2015 and U+2212.
//  3. Every run of Unicode White_Space characters, such as U+00A0, U+2003
//     or LF, to one space (N1), with nothing trimmed at either end (N2).
//     Zero-width characters (U+200B, U+200D, U+FEFF) are not White_Space,
//     and stay.
//
// No mapped character, and no space, composes with what follows it, so
// normalized text normalizes to itself. Text that joins several pieces is
// joined first and normalized after, as one string (N6): see Join.
func Normalize(s string) string {
	s = norm.NFC.String(s)
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.Is(unicode.White_Space, r) {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		switch r {
		case 0x2018, 0x2019: // left and right single quotation marks
			b.WriteByte('\'')
		case 0x201C, 0x201D: // left and right double quotation marks
			b.WriteByte('"')
		case 0x2013, 0x2014: // en dash, em dash
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Hash is the sha256 of text's normalized form (Normalize), its UTF-8 bytes,
// as 64 lowercase hex digits: a node's sha256 in tree.json is Hash of its
// own text, and sha256_text is Hash of SectionText.
func Hash(text string) string {
	return l0.SHA256Hex([]byte(Normalize(text)))
}
