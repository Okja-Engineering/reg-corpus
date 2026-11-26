package ecfr

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/l0"
)

// newer is `regcorpus fetch --latest` for s (R1b; plan step 6, ADR 0014),
// and returns the lines to print. It reads s's meta.yaml, after the
// stored-state check every writer of one makes (l0's Stored), and eCFR's
// version list, and writes nothing. Dates alone decide, as R1b reads (F1:
// A): unless eCFR's latest version is the stored one (standing), it is a
// *Finding (exit 1; F7: L1, report only), whose message is the line
// printed. An entry for the stored date issued on or after retrieved_at's
// day, a reissue, is printed as information, for fetch --verify to compare
// its text.
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
		note := s.Name + ": information: eCFR lists " + string(v.Date) + " with issue_date " + v.IssueDate
		switch d, err := ParseDate(v.IssueDate); {
		case err != nil:
			note += ", not a YYYY-MM-DD date, so whether it was reissued since it was fetched is unknown"
		case d < m.RetrievedAt:
			continue
		default:
			// retrieved_at is a UTC date and issue_date eCFR's: on the same
			// day either may be first, so that day counts (worklist §8).
			note += ", issued on or after the day the stored file was fetched (retrieved_at " + string(m.RetrievedAt) + ")"
		}
		lines = append(lines, note+"; run regcorpus fetch --verify "+s.Name)
	}
	if !sd.current {
		return lines, findingf("%s (R1b); nothing is written", line)
	}
	return lines, nil
}

// standing is where a stored version stands in eCFR's version list: the
// list's own latest date, whether an entry at it removed the source, and
// what that says of the stored date, by dates alone (R1b).
type standing struct {
	latest  Date
	removed bool
	current bool   // eCFR's latest version is the stored one
	older   bool   // eCFR's latest is dated before the stored one
	verdict string // what that is, as fetch --latest prints it
}

// standOf reads vs, a version list, against stored, the stored
// source_version_date. An empty list is an error.
func standOf(vs []Version, stored Date) (standing, error) {
	if len(vs) == 0 {
		return standing{}, errors.New("eCFR lists no versions")
	}
	var sd standing
	var after []Date // each date after the stored one, once
	gone := map[Date]bool{}
	for _, v := range vs {
		sd.latest = max(sd.latest, v.Date) // YYYY-MM-DD orders as text
		gone[v.Date] = gone[v.Date] || v.Removed
		if v.Date > stored && !slices.Contains(after, v.Date) {
			after = append(after, v.Date)
		}
	}
	sd.removed = gone[sd.latest]
	slices.Sort(after)
	switch {
	case sd.latest > stored:
		listed := make([]string, len(after))
		for i, d := range after {
			listed[i] = string(d)
			if gone[d] {
				listed[i] += " (removed)"
			}
		}
		sd.verdict = "newer versions " + strings.Join(listed, ", ")
	case sd.latest < stored:
		sd.older, sd.verdict = true, "eCFR no longer lists the stored version as latest"
	case sd.removed:
		sd.verdict = "the latest version removed it"
	default:
		sd.current, sd.verdict = true, "no newer version"
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
// stored one, by its direction: eCFR's list ends before the stored
// version, or fetch --latest lists what came after it.
func (sd standing) advice() string {
	if sd.older {
		return "eCFR's list ends before the stored version"
	}
	return "run regcorpus fetch --latest"
}
