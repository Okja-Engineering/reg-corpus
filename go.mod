// Module for the regcorpus CLI (spec decision 6: CLI only; R5: the gate needs a binary).
// golang.org/x/text for NFC (internal/anchors, plan step 5); yaml.v3 arrives in a later plan step.
module github.com/okja-engineering/reg-corpus

go 1.27

require golang.org/x/text v0.42.0
