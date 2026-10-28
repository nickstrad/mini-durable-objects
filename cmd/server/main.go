package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/store"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	dir := flag.String("data", "data/objects", "directory for object databases")
	idle := flag.Duration("idle", time.Minute, "close an object's database after it is unused this long")
	flag.Parse()

	log.Printf("Listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, newRouter(store.NewSQLite(*dir, *idle))))
}
