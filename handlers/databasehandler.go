package handlers

import "net/http"

func HandleResetTestDatabase(w http.ResponseWriter, r *http.Request) {
	// TODO

	// controlla che lo stato sia test, altrimenti non fa nulla

	// chiamo db.ResetTestDatabase
}
