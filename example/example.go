package main

import (
	"github.com/LyudvaDmitry/ebookcatalog"
	"log"
	"net/http"
)

func main() {
	bc := ebookcatalog.NewEbookcatalog("./template.html")
	if err := bc.UseFolder("./books"); err != nil {
		log.Fatal(err)
	}
	http.Handle("/view/", bc)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
