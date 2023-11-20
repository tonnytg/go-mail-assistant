package authentication

import (
	"log"
	"net/http"
)

func AuthServer() {

	http.HandleFunc("/callback", CallbackHandler)
	http.HandleFunc("/auth", AuthHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
