// Command webui serves the admin UI over a set of in-memory books.
//
// It exists so the pages can be looked at without a database and without the
// desktop shell: go run ./cmd/webui, then open the address it prints.
//
// It is not the application. The books are held in memory and forgotten when the
// process stops, so nothing entered here is a record of anything. Wiring the
// same handler into the Wails asset server over the MariaDB ledger is the next
// slice, and the handler does not change when it happens — it is built over
// web.Books either way.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"fingo/web"
	"fingo/web/books"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "address to listen on")
	flag.Parse()

	ledger, err := books.New()
	if err != nil {
		log.Fatalf("open the books: %v", err)
	}

	server, err := web.NewServer(ledger)
	if err != nil {
		log.Fatalf("build the admin UI: %v", err)
	}

	fmt.Printf("FinGo admin UI on http://%s\n", *addr)
	fmt.Println("The books are in memory: nothing entered here survives the process.")

	if err := http.ListenAndServe(*addr, server.Handler()); err != nil {
		log.Fatalf("serve the admin UI: %v", err)
	}
}
