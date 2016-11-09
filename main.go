package main

import "net/http"

func main() {
	http.ListenAndServe("localhost:3232", nil)
}
