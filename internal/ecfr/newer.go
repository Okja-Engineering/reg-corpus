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
// version list, and writes nothing. A version dated after the stored
// source_version_date, or a latest version that removed s, is a *Finding
// (exit 1; F7: L1, report only). Dates alone decide, as R1b reads (F1: A):
// an entry for the stored date issued after retrieved_at, a reissue, is
// printed as information, for fetch --verify to compare its text.
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
	_, err = Latest(vs)
	var removed *RemovedError
	if err != nil && !errors.As(err, &removed) {
		return nil, err // no versions listed
	}
	var after []Date // the dates after the stored one, each once, in order
	gone := map[Date]bool{}
	var reissued []Date
	for _, v := range vs {
		switch {
		case v.Date > m.SourceVersionDate:
			if !slices.Contains(after, v.Date) {
				after = append(after, v.Date)
			}
			gone[v.Date] = gone[v.Date] || v.Removed
		case v.Date == m.SourceVersionDate && v.IssueDate > m.RetrievedAt && !slices.Contains(reissued, v.IssueDate):
			reissued = append(reissued, v.IssueDate)
		}
	}
	slices.Sort(after)
	slices.Sort(reissued)
	latest := m.SourceVersionDate
	listed := make([]string, len(after))
	for i, d := range after {
		listed[i] = string(d)
		if gone[d] {
			listed[i] += " (removed)"
		}
		latest = d
	}
	for _, v := range vs {
		latest = max(latest, v.Date)
	}
	prefix := s.Name + ": stored source_version_date " + string(m.SourceVersionDate) + ", eCFR's latest " + string(latest) + ": "
	lines := []string{prefix + "no newer version"}
	if len(after) > 0 {
		lines[0] = prefix + "newer versions " + strings.Join(listed, ", ")
	}
	for _, d := range reissued {
		lines = append(lines, s.Name+": information: eCFR lists "+string(m.SourceVersionDate)+" reissued on "+string(d)+
			", after retrieved_at "+string(m.RetrievedAt)+"; regcorpus fetch --verify "+s.Name+" compares its text")
	}
	switch {
	case len(after) > 0:
		msg := "eCFR lists a version of " + s.Name + " dated after its stored source_version_date " + string(m.SourceVersionDate) + ": " + strings.Join(listed, ", ")
		if removed != nil {
			msg += "; " + removed.Error()
		}
		return lines, findingf("%s (R1b); nothing is written", msg)
	case removed != nil:
		return lines, findingf("%v (R1b); nothing is written", removed)
	}
	return lines, nil
}
