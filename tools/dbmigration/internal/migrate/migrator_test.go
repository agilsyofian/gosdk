package migrate

import (
	"reflect"
	"testing"
)

func TestSplitStatementsIgnoresSemicolonsInCommentsAndStrings(t *testing.T) {
	script := `-- a comment; with a terminator
INSERT INTO test (value) VALUES ('one; two');
/* a block comment; with a terminator */
INSERT INTO test (value) VALUES ('three');`

	got := splitStatements(script)
	want := []string{
		"INSERT INTO test (value) VALUES ('one; two')",
		"/* a block comment; with a terminator */\nINSERT INTO test (value) VALUES ('three')",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitStatements() = %#v, want %#v", got, want)
	}
}
