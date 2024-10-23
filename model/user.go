package model

type User struct {
	UserID      int    `gorm:"column:id_user;primaryKey;autoIncrement"`
	FirstName   string `gorm:"column:first_name;type:text;not null"`
	LastName    string `gorm:"column:last_name;type:text;not null"`
	BirthDate   string `gorm:"column:birth_date;type:date"`
	Gender      string `gorm:"column:gender;type:text"`
	FirebaseUID string `gorm:"column:firebase_uid;type:text;not null"`
	ZipCode     int    `gorm:"column:zip_code;type:integer"`
	StreetName  string `gorm:"column:street_name;type:text"`
	HouseNumber int    `gorm:"column:house_number;type:integer"`
	City        string `gorm:"column:city;type:text"`
}

func (User) TableName() string {
	return "user"
}
