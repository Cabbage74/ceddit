package service

import (
	"ceddit/models"
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

func LogIn(p *models.ParamLogin) error {
	exist, err := mysql.CheckUserExist(p.Username)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("Invalid username")
	}

	u := models.User{
		Username: p.Username,
		Password: encryptPassword(p.Password),
	}

	ok, err := mysql.CheckPassword(&u)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("Wrong password")
	}
	return nil
}

func encryptPassword(oPassword string) string {
	h := md5.New()
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum([]byte(oPassword)))
}
