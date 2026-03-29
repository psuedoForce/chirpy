package main

import "net/http"

func main() {
	servMux := http.NewServeMux()
	server := http.Server{
		Handler: servMux,
		Addr:    ":8080",
	}

	e := server.ListenAndServe()
	if e != nil {
		return
	}
}
