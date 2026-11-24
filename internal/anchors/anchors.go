package anchors

import (
	"errors"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
	_ "golang.org/x/text/unicode/norm"
)

// Stubs for the red commit (plan step 5): the tests pin what these do.

var errNotBuilt = errors.New("anchors: not built yet (plan step 5)")

func Normalize(s string) string { return s }

func Hash(text string) string { return "" }

func ID(section string, path []string) string { return "" }

func Operative(n *parse.Node) (bool, error) { return false, errNotBuilt }

func Join(nodes []*parse.Node) (string, error) { return "", errNotBuilt }

type Derived struct {
	Tree       *parse.Tree
	TreeJSON   []byte
	TextMD     []byte
	SHA256Text meta.SHA256
}

func Build(raw []byte) (*Derived, error) { return nil, errNotBuilt }

func EncodeTree(t *parse.Tree) ([]byte, error) { return nil, errNotBuilt }

func TextMD(t *parse.Tree) []byte { return nil }

func SHA256Text(t *parse.Tree) meta.SHA256 { return "" }

type Written struct {
	Tree, Text, Meta bool
	SHA256Text       meta.SHA256
}

func Write(root string, s l0.Source) (Written, error) { return Written{}, errNotBuilt }
