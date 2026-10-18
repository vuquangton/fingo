package migrate

import (
	"reflect"
	"testing"
)

// The splitter is what stands between a migration file and the driver, and the
// only thing it can get wrong is cutting in the wrong place. Each case below is
// a place a semicolon looks like a separator and is not one.
func TestSplitStatements_CutsOnlyWhereASemicolonSeparates(t *testing.T) {
	cases := map[string]struct {
		script string
		want   []string
	}{
		"two statements": {
			script: "CREATE TABLE a (id INT);\nINSERT INTO a VALUES (1);",
			want:   []string{"CREATE TABLE a (id INT)", "INSERT INTO a VALUES (1)"},
		},
		"no trailing semicolon": {
			script: "SELECT 1",
			want:   []string{"SELECT 1"},
		},
		"a semicolon inside a string": {
			script: "INSERT INTO a VALUES ('one; two');",
			want:   []string{"INSERT INTO a VALUES ('one; two')"},
		},
		"an escaped quote inside a string": {
			script: `INSERT INTO a VALUES ('it\'s; fine');`,
			want:   []string{`INSERT INTO a VALUES ('it\'s; fine')`},
		},
		"a doubled quote inside a string": {
			script: "INSERT INTO a VALUES ('it''s; fine');",
			want:   []string{"INSERT INTO a VALUES ('it''s; fine')"},
		},
		"a backslash before the closing quote": {
			script: `INSERT INTO a VALUES ('ends with \\');`,
			want:   []string{`INSERT INTO a VALUES ('ends with \\')`},
		},
		"a semicolon inside a quoted identifier": {
			script: "SELECT `odd;name` FROM a;",
			want:   []string{"SELECT `odd;name` FROM a"},
		},
		"a semicolon inside a double-quoted string": {
			script: `INSERT INTO a VALUES ("one; two");`,
			want:   []string{`INSERT INTO a VALUES ("one; two")`},
		},
		"a line comment holding a semicolon": {
			script: "SELECT 1 -- and; not this\n;SELECT 2;",
			want:   []string{"SELECT 1 -- and; not this", "SELECT 2"},
		},
		"a hash comment holding a semicolon": {
			script: "SELECT 1 # and; not this\n;SELECT 2;",
			want:   []string{"SELECT 1 # and; not this", "SELECT 2"},
		},
		"a block comment holding a semicolon": {
			script: "SELECT 1 /* and; not this */;",
			want:   []string{"SELECT 1 /* and; not this */"},
		},
		"two hyphens with no space are not a comment": {
			script: "SELECT 1--2;",
			want:   []string{"SELECT 1--2"},
		},
		"a leading comment belongs to the statement": {
			script: "-- what this does\nCREATE TABLE a (id INT);",
			want:   []string{"-- what this does\nCREATE TABLE a (id INT)"},
		},
		"empty script": {
			script: "   \n\t\n",
			want:   nil,
		},
		"a trailing comment alone is not a statement": {
			script: "SELECT 1;\n-- that is all\n",
			want:   []string{"SELECT 1"},
		},
		"nothing but comments": {
			script: "-- one\n/* two */\n# three\n",
			want:   nil,
		},
		"trailing semicolons with nothing between them": {
			script: "SELECT 1;;;",
			want:   []string{"SELECT 1"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := splitStatements(tc.script); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitStatements(%q)\n got %q\nwant %q", tc.script, got, tc.want)
			}
		})
	}
}
