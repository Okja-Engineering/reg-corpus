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
# comparison (ADR 0014): the granule, decoded by xml.etree first and refused
# if it holds a CR, or inside its root a CDATA section or a character
# reference to a White_Space character, as Go's reader refuses it, has its
# sha256_text computed by the layout rule (in each run of character data
# on its own, each line break and up to 12 spaces after it deleted, and
# nothing else: no edge is trimmed; the heading SECTNO, one space, SUBJECT;
# <E> markup dropped and its text kept; PRTPAGE dropped; the CITA left out),
# which must equal the stored sha256_text, and how many of its paragraphs
# match eCFR's, each side normalized as it is. internal/govinfo's
# granule_test.go pins it:
#
#   python3 -I internal/anchors/testdata/oracle.py \
#       sources/ecfr/title-12/1024.41/raw/2025-07-15.xml \
#       internal/govinfo/testdata/CFR-2026-title12-vol8-sec1024-41.xml
import hashlib
import html
import re
import sys
import unicodedata
from xml.etree import ElementTree
from xml.parsers import expat

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
# deleted from each run of decoded character data on its own (an element's
# text, or the tail after a child), so a run never crosses a tag.
LAYOUT = re.compile(r"\n {0,12}")
# A character reference, decimal or hexadecimal, its number captured.
REFERENCE = re.compile(rb"&#([0-9]+|[xX][0-9a-fA-F]+);")


def run(x):
    """One run of a granule's decoded character data, its layout deleted."""
    return LAYOUT.sub("", x or "")


def paragraph(p):
    """A <P>'s text: its runs, each <E>'s text kept, each <PRTPAGE/> page
    break dropped, nothing trimmed."""
    out = [run(p.text)]
    for child in p:
        if child.tag == "E":
            out.append(run(child.text))
        out.append(run(child.tail))
    return "".join(out)


def inside_root(data):
    """The bytes of data inside its root element, from the root's start tag
    to its end tag, as Python's own XML parser (expat) finds them."""
    parser, depth, span = expat.ParserCreate(), [0], [0, 0]

    def start(name, attrs):
        if depth[0] == 0:
            span[0] = parser.CurrentByteIndex
        depth[0] += 1

    def end(name):
        depth[0] -= 1
        if depth[0] == 0:
            span[1] = parser.CurrentByteIndex

    parser.StartElementHandler, parser.EndElementHandler = start, end
    parser.Parse(data, True)
    return data[span[0] : span[1]]


def refused(data):
    """Why Go's reader refuses the granule data, for what this reads: a CR
    anywhere; inside the root a CDATA section, or a character reference to a
    White_Space character. None when neither."""
    if b"\r" in data:
        return "a CR"
    inner = inside_root(data)
    if b"<![CDATA[" in inner:
        return "a CDATA section"
    for m in REFERENCE.finditer(inner):
        n = m.group(1).decode()
        c = int(n[1:], 16) if n[0] in "xX" else int(n)
        if c <= 0x10FFFF and chr(c) in WHITE_SPACE:
            return "a reference to a White_Space character, " + m.group(0).decode()
    return None


def granule(path, ecfr_head, ecfr_paras):
    """plan step 6's comparison: govinfo's sha256_text, computed the way the
    stored one is (the heading as SECTNO, one space and SUBJECT; the
    paragraphs one space apart; the CITA left out), and how many paragraphs
    match eCFR's, each side normalized as it is, nothing trimmed. The
    granule is decoded by Python's own XML parser before the layout is
    deleted, and refused, as Go's reader refuses it, if it holds a CR, or
    inside its root a CDATA section or a reference to a White_Space
    character."""
    data = open(path, "rb").read()
    why = refused(data)
    if why:
        sys.exit("govinfo: refused: " + why)
    root = ElementTree.fromstring(data)
    section = root.find("SECTION")
    head = run(section.find("SECTNO").text) + " " + run(section.find("SUBJECT").text)
    paras = [paragraph(p) for p in section.findall("P")]
    print("govinfo DATE", root.find("FDSYS/DATE").text)
    print("govinfo paragraphs", len(paras))
    print("govinfo heading matches", normalize(head) == normalize(ecfr_head))
    same = sum(normalize(g) == normalize(e) for g, e in zip(paras, ecfr_paras))
    print("govinfo paragraphs matching eCFR's", same, "of", len(ecfr_paras))
    print("govinfo sha256_text", sha256(head + " " + " ".join(paras)))


if __name__ == "__main__":
    head, paras = main(sys.argv[1])
    if len(sys.argv) > 2:
        granule(sys.argv[2], head, paras)
