package handlers

import (
	"encoding/json"
	"green-journey-server/db"
	"green-journey-server/model"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getUserByFirebaseUID(w, r)
	case "POST":
		addUser(w, r)
	default:
		log.Println("HandleUsers received an unsupported method")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
	}
}

func getUserByFirebaseUID(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	uid := r.URL.Query().Get("uid")

	// check id present
	if uid == "" {
		log.Println("Firebase uid is missing")
		http.Error(w, "User id is required", http.StatusBadRequest)
		return
	}

	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserByFirebaseUID(uid)
	if err != nil {
		log.Println("User not found: ", err)
		http.Error(w, "User could not be found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		log.Println(err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func addUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	var user model.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println("Error while decoding JSON: ", err)
		http.Error(w, "Wrong data provided", http.StatusBadRequest)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			log.Println("Error closing request body:", err)
		}
	}()

	// check non-empty strings (only for mandatory fields)
	if user.FirstName == nil ||
		user.LastName == nil ||
		user.FirebaseUID == nil {
		log.Println("Missing required fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}
	// check birthdate format and value
	if user.BirthDate != nil {
		birthDate, err := time.Parse("2006-01-02", *user.BirthDate)
		if err != nil {
			log.Println("Invalid data: ", err)
			http.Error(w, "Invalid birth date format", http.StatusBadRequest)
			return
		}
		if birthDate.After(time.Now()) {
			log.Println("Invalid data: ", err)
			http.Error(w, "Birth date cannot be in the future", http.StatusBadRequest)
			return
		}
	}

	if user.Gender != nil && *user.Gender != "male" && *user.Gender != "female" && *user.Gender != "other" {
		// check gender
		log.Println("Invalid data: ", err)
		http.Error(w, "Invalid gender value", http.StatusBadRequest)
		return
	}

	// insert user
	userDAO := db.NewUserDAO(db.GetDB())
	user, err = userDAO.AddUser(user)
	if err != nil {
		log.Println("Error while interacting with the database: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// send response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusCreated)
}

func HandleModifyUser(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		modifyUser(w, r)
	case "DELETE":
		deleteUser(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
	}
}

func modifyUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// extract userid from URI
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[2] == "" {
		log.Println("Invalid path")
		http.Error(w, "User ID not provided", http.StatusBadRequest)
		return
	}
	userIDStr := parts[2]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID < 0 {
		log.Println("Invalid user ID")
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// get the user from the body
	var user model.User
	err = json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println("Error while decoding JSON: ", err)
		http.Error(w, "Wrong data provided", http.StatusBadRequest)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			log.Println("Error closing request body:", err)
		}
	}()

	// check non-empty strings (only for mandatory fields)
	if user.FirstName == nil ||
		user.LastName == nil ||
		user.FirebaseUID == nil {
		log.Println("Missing required fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}
	// check birthdate format and value
	if user.BirthDate != nil {
		birthDate, err := time.Parse("2006-01-02", *user.BirthDate)
		if err != nil {
			log.Println("Invalid data: ", err)
			http.Error(w, "Invalid birth date format", http.StatusBadRequest)
			return
		}
		if birthDate.After(time.Now()) {
			log.Println("Invalid data: ", err)
			http.Error(w, "Birth date cannot be in the future", http.StatusBadRequest)
			return
		}
	}

	if user.Gender != nil && *user.Gender != "male" && *user.Gender != "female" && *user.Gender != "other" {
		// check gender
		log.Println("Invalid data: ", err)
		http.Error(w, "Invalid gender value", http.StatusBadRequest)
		return
	}

	// update user in db
	userDAO := db.NewUserDAO(db.GetDB())
	err = userDAO.UpdateUser(user)
	if err != nil {
		log.Println("Error while interacting with db: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// send user back
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// extract userid from URI
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[2] == "" {
		log.Println("Invalid path")
		http.Error(w, "User ID not provided", http.StatusBadRequest)
		return
	}
	userIDStr := parts[2]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID < 0 {
		log.Println("Invalid user ID")
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// delete user
	userDAO := db.NewUserDAO(db.GetDB())
	_, err = userDAO.GetUserById(userID)
	if err != nil {
		log.Println("User not found: ", err)
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	err = userDAO.DeleteUser(userID)
	if err != nil {
		log.Println("Error while interacting with the db: ", err)
		http.Error(w, "Error while deleting user", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
