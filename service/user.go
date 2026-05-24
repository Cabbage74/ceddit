package service

import (
	"ceddit/models"
	"ceddit/pkg/jwt"
	"ceddit/pkg/snowflake"
	"ceddit/repository/mysql"
	"crypto/md5"
	"encoding/hex"
	"errors"
)

const secret = "cabbage"

func SignUp(p *models.ParamSignUp) error {
	exist, err := mysql.CheckUserExist(p.Username)
	if err != nil {
		return err
	}
	if exist {
		return errors.New("Duplicate username")
	}

	u := models.User{
		UserID:   snowflake.GenID(),
		Username: p.Username,
		Password: encryptPassword(p.Password),
	}

	return mysql.InsertUser(&u)
}

func LogIn(p *models.ParamLogin) (string, error) {
	u := models.User{
		Username: p.Username,
	}

	err := mysql.GetUser(&u)
	if err != nil {
		return "", err
	}
	if u.Password != encryptPassword(p.Password) {
		return "", errors.New("Wrong password")
	}

	return jwt.GenToken(u.UserID, u.Username)
}

func encryptPassword(oPassword string) string {
	h := md5.New()
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum([]byte(oPassword)))
}
