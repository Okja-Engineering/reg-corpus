// Normalized text (spec "Definitions"; ADR 0013, pins N1–N6): Unicode NFC,
// then curly quotes to ASCII and en and em dashes to "-", then every run of
// Unicode White_Space collapsed to one space, never trimmed.
package anchors_test

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"golang.org/x/text/unicode/norm"
)

func TestNormalizePins(t *testing.T) {
	for _, tc := range []struct{ pin, in, want string }{
		// NFC, not NFKC.
		{"NFC composes", "Cafe\u0301", "Caf\u00E9"},
		{"NFC reorders and composes", "a\u0323\u0302", "\u1EAD"},
		{"NFC maps a singleton", "\u212B", "\u00C5"},
		{"NFC is not NFKC: a ligature stays", "\uFB01le", "\uFB01le"},
		{"NFC is not NFKC: a superscript stays", "x\u00B2", "x\u00B2"},
		{"NFC is not NFKC: a fullwidth letter stays", "\uFF21", "\uFF21"},

		// N1: Unicode White_Space is whitespace.
		{"N1 no-break space", "a\u00A0b", "a b"},
		{"N1 tab, LF, CR, VT, FF", "a\t\n\r\v\fb", "a b"},
		{"N1 next line U+0085", "a\u0085b", "a b"},
		{"N1 em space U+2003", "a\u2003b", "a b"},
		{"N1 narrow no-break space U+202F", "a\u202Fb", "a b"},
		{"N1 ideographic space U+3000", "a\u3000b", "a b"},
		{"N1 line separator U+2028", "a\u2028b", "a b"},
		{"N1 zero-width space U+200B stays", "a\u200Bb", "a\u200Bb"},
		{"N1 byte-order mark U+FEFF stays", "a\uFEFFb", "a\uFEFFb"},
		{"N1 zero-width joiner U+200D stays", "a\u200Db", "a\u200Db"},

		// N2: runs collapse to one space; nothing is trimmed.
		{"N2 a run collapses", "a  \u00A0\t b", "a b"},
		{"N2 a leading space stays", " a", " a"},
		{"N2 a trailing space stays", "a ", "a "},
		{"N2 runs at both ends collapse, not trimmed", "  a\n\n", " a "},
		{"N2 only whitespace", "\t\u00A0 ", " "},
		{"N2 empty", "", ""},

		// N3: these four curly quotes, and no others.
		{"N3 left single", "\u2018x", "'x"},
		{"N3 right single", "x\u2019s", "x's"},
		{"N3 left double", "\u201Cx", "\"x"},
		{"N3 right double", "x\u201D", "x\""},
		{"N3 low-9 single U+201A stays", "\u201Ax", "\u201Ax"},
		{"N3 reversed single U+201B stays", "\u201Bx", "\u201Bx"},
		{"N3 low-9 double U+201E stays", "\u201Ex", "\u201Ex"},
		{"N3 reversed double U+201F stays", "\u201Fx", "\u201Fx"},
		{"N3 prime U+2032 stays", "5\u2032", "5\u2032"},
		{"N3 guillemets stay", "\u00ABx\u00BB", "\u00ABx\u00BB"},
		{"N3 ASCII quotes stay", "'x\"", "'x\""},

		// N4: en and em dashes, and no other.
		{"N4 en dash", "1\u20132", "1-2"},
		{"N4 em dash", "a\u2014(1)", "a-(1)"},
		{"N4 hyphen U+2010 stays", "a\u2010b", "a\u2010b"},
		{"N4 non-breaking hyphen U+2011 stays", "a\u2011b", "a\u2011b"},
		{"N4 figure dash U+2012 stays", "a\u2012b", "a\u2012b"},
		{"N4 horizontal bar U+2015 stays", "a\u2015b", "a\u2015b"},
		{"N4 minus sign U+2212 stays", "a\u2212b", "a\u2212b"},
		{"N4 ASCII hyphen stays", "a-b", "a-b"},

		// Together, on text of 1024.41's form.
		{"a heading separator", "(b) Receipt of a loss mitigation application\u2014", "(b) Receipt of a loss mitigation application-"},
		{"quotes and dashes in one string", "\u201CA\u201D \u2013 \u2018b\u2019", "\"A\" - 'b'"},
	} {
		if got := normalize(t, tc.in); got != tc.want {
			t.Errorf("%s: Normalize(%+q) = %+q, want %+q", tc.pin, tc.in, got, tc.want)
		}
	}
}

// TestNormalizeEveryRune: for every Unicode scalar value, alone, Normalize
// gives its NFC form with exactly the pins' changes: one of the six mapped
// characters becomes its ASCII form, a run of White_Space becomes one space,
// and every other character is left as NFC left it. So no character outside
// the pins is mapped, dropped or treated as whitespace.
func TestNormalizeEveryRune(t *testing.T) {
	mapped := map[rune]string{'\u2018': "'", '\u2019': "'", '\u201C': `"`, '\u201D': `"`, '\u2013': "-", '\u2014': "-"}
	bad := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		nfc := norm.NFC.String(string(r))
		want := ""
		space := false
		for _, c := range nfc {
			switch m, ok := mapped[c]; {
			case unicode.Is(unicode.White_Space, c):
				if !space {
					want += " "
				}
				space = true
				continue
			case ok:
				want += m
			default:
				want += string(c)
			}
			space = false
		}
		if got := normalize(t, string(r)); got != want {
			if bad++; bad <= 20 {
				t.Errorf("Normalize(%U) = %+q, want %+q", r, got, want)
			}
		}
	}
	if bad > 20 {
		t.Errorf("... %d runes in all", bad)
	}
}

// TestNormalizeIsIdempotent: normalized text normalizes to itself, so a
// quote copied from normalized text matches it (N5: NFC first, then the
// mappings, then the collapse; no mapped character or space composes).
func TestNormalizeIsIdempotent(t *testing.T) {
	for _, s := range []string{
		"Cafe\u0301 \u2014 \u201Cx\u201D\u00A0\u00A0y ",
		"\u212B\u2003\u2019\u0301",
		" \u0301\u2014\u0301\u00A0\u0301",
	} {
		once := normalize(t, s)
		if twice := normalize(t, once); twice != once {
			t.Errorf("Normalize(Normalize(%+q)) = %+q, want %+q", s, twice, once)
		}
	}
}

// TestHashIsOfNormalizedText: Hash is the lowercase hex sha256 of the UTF-8
// bytes of the normalized text.
func TestHashIsOfNormalizedText(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// sha256("") and sha256("a b"): the second input normalizes to "a b".
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"a\u00A0\u00A0b", "c8687a08aa5d6ed2044328fa6a697ab8e96dc34291e8c2034ae8c38e6fcc6d65"},
	} {
		if got, err := anchors.Hash(tc.in); err != nil || got != tc.want {
			t.Errorf("Hash(%+q) = %s, %v; want %s", tc.in, got, err, tc.want)
		}
	}
}

// TestInvalidUTF8IsRefused: text that isn't valid UTF-8 is an error from
// Normalize and Hash, never read as U+FFFD, so it can't normalize or hash
// as text holding U+FFFD does; that text itself is valid.
func TestInvalidUTF8IsRefused(t *testing.T) {
	for _, in := range []string{
		"a\xffb",         // a byte that starts no encoding
		"a\xc3",          // a truncated encoding
		"\xc0\xaf",       // an overlong encoding of "/"
		"x\xed\xa0\x80y", // an encoded surrogate, U+D800
	} {
		if got, err := anchors.Normalize(in); err == nil {
			t.Errorf("Normalize(%+q) = %+q, nil; want an error", in, got)
		}
		if got, err := anchors.Hash(in); err == nil {
			t.Errorf("Hash(%+q) = %s, nil; want an error", in, got)
		}
	}
	if got, err := anchors.Hash("a�b"); err != nil || got != sum("a�b") {
		t.Errorf("Hash(a U+FFFD b) = %s, %v; want the sha256 of its UTF-8", got, err)
	}
}
