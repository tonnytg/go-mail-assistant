package authentication

import (
	"log"
	"net/http"
)

func AuthServer() {
	http.HandleFunc("/auth", AuthHandler)
	http.HandleFunc("/emails", EmailsHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
