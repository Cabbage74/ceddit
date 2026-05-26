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

func GetUserByName(userName string) (*models.User, error) {
	sqlStr := `select user_id, username, password from user where username = ?`
	var user models.User
	if err := db.Get(&user, sqlStr, userName); err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByID(userID int64) (*models.User, error) {
	sqlStr := `select user_id, username, password from user where user_id = ?`
	var user models.User
	if err := db.Get(&user, sqlStr, userID); err != nil {
		return nil, err
	}
	return &user, nil
}


