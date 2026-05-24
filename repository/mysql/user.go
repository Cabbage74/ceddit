package mysql

import (
	"ceddit/models"
)

const secret = "cabbage"

func CheckUserExist(username string) (bool, error) {
	sqlStr := `select count(user_id) from user where username = ?`
	var count int
	if err := db.Get(&count, sqlStr, username); err != nil {
		return false, err
	}
	return count > 0, nil
}

func InsertUser(user *models.User) error {
	sqlStr := `insert into user(user_id, username, password) values(?, ?, ?)`
	_, err := db.Exec(sqlStr, user.UserID, user.Username, user.Password)
	if err != nil {
		return err
	}
	return nil
}

func CheckPassword(user *models.User) (bool, error) {
	sqlStr := `select count(user_id) from user where username = ? and password = ?`
	var count int
	if err := db.Get(&count, sqlStr, user.Username, user.Password); err != nil {
		return false, err
	}
	return count == 1, nil
}
