package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"pagemanager"
)

func main() {
	pm, err := pagemanager.New(&pagemanager.Config{
		FS: os.DirFS("."),
	})
	if err != nil {
		log.Fatal(err)
	}
	const addr = "127.0.0.1:8020"
	fmt.Println("listening on " + addr)
	fmt.Println(http.ListenAndServe(addr, pm.Pagemanager(pm.NotFound())))
}
