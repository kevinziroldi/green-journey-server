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
		getUserById(w, r)
	case "POST":
		addUser(w, r)
	default:
		log.Println("HandleUsers received an unsupported method")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
	}
}

func getUserById(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")

	// check id present
	if idStr == "" {
		log.Println("User id is missing")
		http.Error(w, "User id is required", http.StatusBadRequest)
		return
	}
	// check id format
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		log.Println("User id is not valid")
		http.Error(w, "The provided id is not valid", http.StatusBadRequest)
		return
	}

	userDAO := db.NewUserDAO(db.GetDB())

	user, err := userDAO.GetUserById(id)
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

	// check non-empty strings
	if user.FirstName == "" ||
		user.LastName == "" ||
		user.BirthDate == "" ||
		user.Gender == "" ||
		user.FirebaseUID == "" ||
		user.StreetName == "" ||
		user.City == "" {
		log.Println("Missing required fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}
	// check birthdate format and value
	birthDate, err := time.Parse("2006-01-02", user.BirthDate)
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
	// check valid zip code and house number
	if user.ZipCode <= 0 {
		log.Println("Invalid data: ", err)
		http.Error(w, "Invalid zip code", http.StatusBadRequest)
		return
	}
	if user.HouseNumber <= 0 {
		log.Println("Invalid data: ", err)
		http.Error(w, "Invalid house number", http.StatusBadRequest)
		return
	}
	// check gender
	if user.Gender != "male" && user.Gender != "female" && user.Gender != "other" {
		log.Println("Invalid data: ", err)
		http.Error(w, "Invalid gender value", http.StatusBadRequest)
		return
	}

	// insert user
	userDAO := db.NewUserDAO(db.GetDB())
	err = userDAO.AddUser(user)
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
	case "PATCH":
		modifyUser(w, r)
	case "DELETE":
		deleteUser(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
	}
}

func modifyUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PATCH" {
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

	// get the user
	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserById(userID)
	if err != nil {
		log.Println("User not found: ", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// get body content
	var updateData map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&updateData)
	if err != nil {
		log.Println("Error while decoding JSON: ", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			log.Println("Error closing request body:", err)
		}
	}()

	// update fields in the request body
	firstName, isString := updateData["first_name"].(string)
	if isString && firstName != "" {
		user.FirstName = firstName
	}
	lastName, isString := updateData["last_name"].(string)
	if isString && lastName != "" {
		user.LastName = lastName
	}
	birthDate, isString := updateData["birth_date"].(string)
	if isString && birthDate != "" {
		parsedDate, err := time.Parse("2006-01-02", birthDate)
		if err != nil {
			log.Println("Invalid data: ", err)
			http.Error(w, "Invalid birth date format", http.StatusBadRequest)
			return
		}
		user.BirthDate = parsedDate.Format("2006-01-02")
	}
	gender, isString := updateData["gender"].(string)
	if isString && gender != "" {
		if gender != "male" && gender != "female" && gender != "other" {
			log.Println("Invalid data")
			http.Error(w, "Invalid gender type", http.StatusBadRequest)
			return
		}
		user.Gender = gender
	}
	zipCode, isNumber := updateData["zip_code"].(float64)
	if isNumber && zipCode >= 0 {
		user.ZipCode = int(zipCode)
	}
	streetName, isString := updateData["street_name"].(string)
	if isString && streetName != "" {
		user.StreetName = streetName
	}
	houseNumber, isNumber := updateData["house_number"].(float64)
	if isNumber && houseNumber >= 0 {
		user.HouseNumber = int(houseNumber)
	}
	city, isString := updateData["city"].(string)
	if isString && city != "" {
		user.City = city
	}

	// update user in db
	err = userDAO.UpdateUser(user)
	if err != nil {
		log.Println("Error while interacting with db: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

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
