// Command genchart writes the TT133 chart as SQL value rows for the migration
// seed, from the same table the engine validates against.
//
//	go run ./tools/genchart -out .local/chart_rows.sql
//
// It writes the file itself rather than stdout: the console on some Windows
// terminals re-encodes a pipe and mangles Vietnamese account names.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"fingo/internal/domain/gl"
)

func main() {
	out := flag.String("out", ".local/chart_rows.sql", "output file")
	flag.Parse()

	var b strings.Builder
	for _, a := range gl.TT133Chart() {
		parent := "NULL"
		if a.ParentCode != "" {
			parent = "'" + a.ParentCode + "'"
		}
		fmt.Fprintf(&b, "    ('%s', '%s', %s, '%s', '%s'),\n", a.Code, strings.ReplaceAll(a.Name, "'", "''"), parent, a.Type, a.Nature)
	}

	if err := os.MkdirAll(dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "genchart:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "genchart:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s", *out)
}

func dir(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[:i]
	}
	return "."
}
