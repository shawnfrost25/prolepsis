package oauthGithub

import (
	"io"
	"net/http"
)

func MockGithubWebHook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		print(err)
		return
	}
	println(string(body))
	w.WriteHeader(http.StatusOK)
}
