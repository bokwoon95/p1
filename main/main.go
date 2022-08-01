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
	const port = ":8020"
	fmt.Println("listening on localhost" + port)
	fmt.Println(http.ListenAndServe(port, pm.Pagemanager(pm.NotFound())))
}
