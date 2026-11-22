// Module for the regcorpus CLI (spec decision 6: CLI only; R5: the gate needs a binary).
// Standard library plus yaml.v3 (meta.yaml, plan step 2); x/text arrives in a later plan step.
module github.com/okja-engineering/reg-corpus

go 1.27

require gopkg.in/yaml.v3 v3.0.1
