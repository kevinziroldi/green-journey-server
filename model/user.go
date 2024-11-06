package model

type User struct {
	UserID      int    `gorm:"column:id_user;primaryKey;autoIncrement" json:"user_id"`
	FirstName   string `gorm:"column:first_name;type:text;not null" json:"first_name"`
	LastName    string `gorm:"column:last_name;type:text;not null" json:"last_name"`
	BirthDate   string `gorm:"column:birth_date;type:date" json:"birth_date"`
	Gender      string `gorm:"column:gender;type:text" json:"gender"`
	FirebaseUID string `gorm:"column:firebase_uid;type:text;not null" json:"firebase_uid"`
	ZipCode     int    `gorm:"column:zip_code;type:integer" json:"zip_code"`
	StreetName  string `gorm:"column:street_name;type:text" json:"street_name"`
	HouseNumber int    `gorm:"column:house_number;type:integer" json:"house_number"`
	City        string `gorm:"column:city;type:text" json:"city"`
}

func (User) TableName() string {
	return "user"
}
