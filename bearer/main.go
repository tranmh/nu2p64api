package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

func main() {
	request, _ := http.NewRequest("GET", "http://whatever.com", &bytes.Buffer{})
	request.Header.Add("Authorization", "Bearer asd98765rdxfcyuoklasdopi08hgnmigs7d6o43weavgfthyuoijpk...")
	verifyTokenController(nil, request)
}

func responseWithJSON(writer io.Writer, content []byte, status int) {
}

func errWithJSON(writeri io.Writer, content string, status int) {

}

func verifyTokenController(w http.ResponseWriter, r *http.Request) {
	prefix := "Bearer "
	authHeader := r.Header.Get("Authorization")
	reqToken := strings.TrimPrefix(authHeader, prefix)

	if authHeader == "" || reqToken == authHeader {
		errWithJSON(w, "Authentication header not present or malformed", http.StatusUnauthorized)
		return
	}

	responseWithJSON(w, []byte(`{"message":"Token is valid"}`), http.StatusOK)
}
