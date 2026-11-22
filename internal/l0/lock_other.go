//go:build !darwin && !linux

package l0

import "errors"

// lock has no flock(2) on this platform, so Update always refuses (exit 2):
// a meta.yaml is changed only under its source's lock.
func lock(string) (func(), error) {
	return nil, errors.New("meta.yaml is changed only under its source's lock (flock), built for darwin and linux only")
}
