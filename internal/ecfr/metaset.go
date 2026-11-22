package ecfr

import "io"

// MetaMain runs `regcorpus meta` in the current directory. Stub (red).
func MetaMain(args []string, stdout, stderr io.Writer) int { return metaMain(".", args, stdout, stderr) }

func metaMain(root string, args []string, stdout, stderr io.Writer) int { return 0 }
