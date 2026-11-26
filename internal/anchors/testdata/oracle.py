# An independent oracle for plan step 5's hashes (ADR 0013): 1024.41's
# sha256_text and the sha256 of three of its nodes, computed without the Go
# code. Python's standard library only: unicodedata's NFC, and the raw XML
# read with regular expressions, not internal/parse. build_test.go pins its
# output (oracleText, oracleB, oracleC2v, oracleC2vA1).
#
# Run it from the corpus root, isolated from the environment (-I):
#
#   python3 -I internal/anchors/testdata/oracle.py \
#       sources/ecfr/title-12/1024.41/raw/2025-07-15.xml
#
# It prints the Unicode version of Python's tables, which must be the one
# x/text and Go use (17.0.0 for x/text v0.42.0 and Go 1.27), then each hash.
#
# Given a govinfo CFR granule as well, it also computes plan step 6's
# comparison (ADR 0014): the granule's sha256_text by the layout rule (each
# line break and up to 12 spaces after it deleted; the heading SECTNO, one
# space, SUBJECT; <E> markup dropped and its text kept; PRTPAGE dropped; the
# CITA left out), which must equal the stored sha256_text, and how many of
# its paragraphs match eCFR's. internal/govinfo's granule_test.go pins it:
#
#   python3 -I internal/anchors/testdata/oracle.py \
#       sources/ecfr/title-12/1024.41/raw/2025-07-15.xml \
#       internal/govinfo/testdata/CFR-2026-title12-vol8-sec1024-41.xml
import hashlib
import html
import re
import sys
import unicodedata

# Unicode's White_Space property (PropList.txt), all 25 characters.
WHITE_SPACE = set(
    map(
        chr,
        [0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x20, 0x85, 0xA0, 0x1680]
        + list(range(0x2000, 0x200B))
        + [0x2028, 0x2029, 0x202F, 0x205F, 0x3000],
    )
)
# The four curly quotes and the en and em dashes, mapped to ASCII.
MAPPED = {"‘": "'", "’": "'", "“": '"', "”": '"', "–": "-", "—": "-"}


def normalize(s):
    """NFC, then the mappings, then each White_Space run to one space."""
    s = unicodedata.normalize("NFC", s)
    out, space = [], False
    for c in s:
        if c in WHITE_SPACE:
            if not space:
                out.append(" ")
            space = True
            continue
        space = False
        out.append(MAPPED.get(c, c))
    return "".join(out)


def sha256(s):
    return hashlib.sha256(normalize(s).encode("utf-8")).hexdigest()


def chardata(x):
    """An element's character data: its <I> markup dropped, references read."""
    return html.unescape(re.sub(r"</?I>", "", x))


def main(path):
    raw = open(path, encoding="utf-8").read()
    head = chardata(re.search(r"<HEAD>(.*?)</HEAD>", raw, re.S).group(1))
    paras = [chardata(p) for p in re.findall(r"<P>(.*?)</P>", raw, re.S)]
    print("unicodedata.unidata_version", unicodedata.unidata_version)
    print("paragraphs", len(paras))
    # sha256_text: the HEAD, one space, then the paragraphs, one space
    # between, normalized as one string; the CITA left out.
    print("sha256_text", sha256(head + " " + " ".join(paras)))
    # (b): the second paragraph up to and including its heading's dash.
    b = paras[1][: paras[1].index("—") + 1]
    print("b", sha256(b), repr(b))
    # (c)(2)(v): the fifteenth paragraph up to its child's label, "(A)".
    c2v = paras[14][: paras[14].index("(A)")]
    print("c-2-v", sha256(c2v), repr(c2v))
    # (c)(2)(v)(A)(1): the whole sixteenth paragraph.
    print("c-2-v-A-1", sha256(paras[15]), repr(paras[15][:40] + "..."))
    return head, paras


# govinfo's layout (plan step 6, ADR 0014): a line break and up to 12 spaces
# after it, the most indentation govinfo's pretty-printer puts there. It is
# matched on the raw XML, so a run never crosses a tag.
LAYOUT = re.compile(r"\n {0,12}")


def granule_text(x):
    """An element's text in a govinfo granule: the layout deleted, the empty
    <PRTPAGE/> page breaks dropped, <E> markup dropped and its text kept,
    references read."""
    x = LAYOUT.sub("", x)
    x = re.sub(r"<PRTPAGE[^>]*/>", "", x)
    return html.unescape(re.sub(r"</?E( [^>]*)?>", "", x))


def granule(path, ecfr_head, ecfr_paras):
    """plan step 6's comparison: govinfo's sha256_text, computed the way the
    stored one is (the heading as SECTNO, one space and SUBJECT; the
    paragraphs one space apart; the CITA left out), and how many paragraphs
    match eCFR's, normalized one by one."""
    raw = open(path, encoding="utf-8").read()
    section = re.search(r"<SECTION>(.*)</SECTION>", raw, re.S).group(1)
    sectno = granule_text(re.search(r"<SECTNO>(.*?)</SECTNO>", section, re.S).group(1))
    subject = granule_text(re.search(r"<SUBJECT>(.*?)</SUBJECT>", section, re.S).group(1))
    head = sectno + " " + subject
    paras = [granule_text(p) for p in re.findall(r"<P>(.*?)</P>", section, re.S)]
    print("govinfo DATE", re.search(r"<DATE>(.*?)</DATE>", raw).group(1))
    print("govinfo paragraphs", len(paras))
    print("govinfo heading matches", normalize(head) == normalize(ecfr_head))
    same = sum(normalize(g) == normalize(e) for g, e in zip(paras, ecfr_paras))
    print("govinfo paragraphs matching eCFR's", same, "of", len(ecfr_paras))
    print("govinfo sha256_text", sha256(head + " " + " ".join(paras)))


if __name__ == "__main__":
    head, paras = main(sys.argv[1])
    if len(sys.argv) > 2:
        granule(sys.argv[2], head, paras)
