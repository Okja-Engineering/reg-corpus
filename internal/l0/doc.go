// Package l0 is a stored L0 source on disk (R1): how a file under sources/
// is stored and read. Every directory on the way must be a real one
// (RealDirs); raw XML and a new meta.yaml are stored once, by hard link, and
// never overwritten (StoreOnce); a meta.yaml is replaced whole, by a
// temporary file renamed over it (Replace). A refusal is a *Conflict (exit
// 1). It holds no rule content.
package l0
