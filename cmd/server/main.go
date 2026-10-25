package main

import (
	"log"
	"net/http"

	"github.com/nickstrad/mini-durable-objects/internal/store"
)

func main() {
	log.Println("Listening on http://127.0.0.1:8787")
	log.Fatal(http.ListenAndServe("127.0.0.1:8787", newRouter(store.NewSQLite("data/objects"))))
}
