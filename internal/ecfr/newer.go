package ecfr

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/l0"
)

// newer is `regcorpus fetch --latest` for s (R1b; plan step 6, ADR 0014),
// and returns the lines to print. It reads s's meta.yaml, after the
// stored-state check every writer of one makes (l0's Stored), and eCFR's
// version list, and writes nothing. Dates alone decide, as R1b reads (F1:
// A): unless eCFR's latest version is the stored one (standing), it is a
// *Finding, exit 1, whose message is the line printed: a newer version is
// the owner's F7 (L1, report only); an older one, or one that removed the
// section, the chief of staff's rulings (F8; worklist round 1, §5). An
// entry for the stored date issued on or after retrieved_at's day, a
// reissue, is printed as information, for fetch --verify to compare its
// text; its issue_date is printed as a date only once read as one, and
// quoted otherwise (worklist round 2, §3).
func (f fetcher) newer(ctx context.Context, s Source) ([]string, error) {
	st, err := l0.Read(f.root, s.stored())
	if err != nil {
		return nil, err
	}
	if _, err := st.Stored(f.root, s.stored()); err != nil {
		return nil, err
	}
	m := st.Meta
	vs, err := f.client.Versions(ctx, s)
	if err != nil {
		return nil, err
	}
	sd, err := standOf(vs, m.SourceVersionDate)
	if err != nil {
		return nil, err
	}
	line := "stored source_version_date " + string(m.SourceVersionDate) + ", eCFR's latest " + sd.latestText() + ": " + sd.verdict
	lines := []string{s.Name + ": " + line}
	var noted []string
	for _, v := range vs {
		if v.Date != m.SourceVersionDate || v.IssueDate == "" || slices.Contains(noted, v.IssueDate) {
			continue
		}
		noted = append(noted, v.IssueDate)
		note := s.Name + ": information: eCFR lists " + string(v.Date) + " with issue_date "
		switch d, err := ParseDate(v.IssueDate); {
		case err != nil:
			note += strconv.Quote(v.IssueDate) + ", not a YYYY-MM-DD date, so whether it was reissued since it was fetched is unknown"
		case d < m.RetrievedAt:
			continue
		default:
			// retrieved_at is a UTC date and issue_date eCFR's: on the same
			// day either may be first, so that day counts (worklist §8).
			note += string(d) + ", issued on or after the day the stored file was fetched (retrieved_at " + string(m.RetrievedAt) + ")"
		}
		lines = append(lines, note+"; run regcorpus fetch --verify "+s.Name)
	}
	if sd.stand != standCurrent {
		return lines, findingf("%s (R1b); nothing is written", line)
	}
	return lines, nil
}

// stand is where a stored version stands in eCFR's version list.
type stand int

const (
	standCurrent stand = iota // eCFR's latest version is the stored one
	standNewer                // eCFR lists versions dated after the stored one
	standOlder                // eCFR's latest is dated before the stored one
	standRemoved              // eCFR's latest entry, at the stored date, removed the source
)

// standing is where a stored version stands in eCFR's version list: the
// list's own latest date and whether an entry at it removed the source
// (Latest), where that leaves the stored date, by dates alone (R1b), and
// what that is, as fetch --latest prints it.
type standing struct {
	latest  Date
	removed bool
	stand   stand
	verdict string
}

// standOf reads vs, a version list, against stored, the stored
// source_version_date. An empty list is an error.
func standOf(vs []Version, stored Date) (standing, error) {
	latest, removed, err := Latest(vs)
	if err != nil {
		return standing{}, err
	}
	sd := standing{latest: latest, removed: removed}
	switch {
	case latest > stored:
		var after []Date // each date after the stored one, once
		gone := map[Date]bool{}
		for _, v := range vs {
			gone[v.Date] = gone[v.Date] || v.Removed
			if v.Date > stored && !slices.Contains(after, v.Date) {
				after = append(after, v.Date)
			}
		}
		slices.Sort(after)
		listed := make([]string, len(after))
		for i, d := range after {
			listed[i] = string(d)
			if gone[d] {
				listed[i] += " (removed)"
			}
		}
		sd.stand, sd.verdict = standNewer, "newer versions "+strings.Join(listed, ", ")
	case latest < stored:
		sd.stand, sd.verdict = standOlder, "eCFR no longer lists the stored version as latest"
	case removed:
		sd.stand, sd.verdict = standRemoved, "the latest entry, at the stored date, removed the section"
	default:
		sd.stand, sd.verdict = standCurrent, "no newer version"
	}
	return sd, nil
}

// latestText is eCFR's latest date, marked when it removed the source.
func (sd standing) latestText() string {
	if sd.removed {
		return string(sd.latest) + " (removed)"
	}
	return string(sd.latest)
}

// advice is what fetch --verify adds when eCFR's latest version isn't the
// stored one, by its direction, after "; ": fetch --latest lists what came
// after it, or eCFR's list ends before it. A removal at the stored date
// gets none: the verdict says all fetch --latest would.
func (sd standing) advice() string {
	switch sd.stand {
	case standNewer:
		return "; run regcorpus fetch --latest"
	case standOlder:
		return "; eCFR's list ends before the stored version"
	}
	return ""
}
