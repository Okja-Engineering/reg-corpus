// Package fsizetest is a test helper, imported only by tests: it runs part
// of a test in a child test binary under a lowered RLIMIT_FSIZE, so a write
// fails part-way (EFBIG) on every unix, Linux CI included, and returns what
// that part returned. internal/l0's and internal/ecfr's tests of a write
// that fails part-way (ADR 0013) share it. It builds on unix only.
package fsizetest
