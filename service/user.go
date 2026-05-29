package service

import (
	"ceddit/models"
	"ceddit/pkg/auth"
	"ceddit/pkg/snowflake"
	"ceddit/repository/mysql"
	"crypto/md5"
	"encoding/hex"
	"errors"
)

const secret = "cabbage"

func SignUp(p *models.ParamSignUp) (*auth.TokenPair, string, error) {
	exist, err := mysql.CheckUserExist(p.Username)
	if err != nil {
		return nil, "", err
	}
	if exist {
		return nil, "", errors.New("Duplicate username")
	}

	u := models.User{
		UserID:   snowflake.GenID(),
		Username: p.Username,
		Password: encryptPassword(p.Password),
	}

	if err := mysql.InsertUser(&u); err != nil {
		return nil, "", err
	}

	// Auto-login: generate token pair after successful signup.
	return auth.CreateTokens(u.UserID, u.Username, p.DeviceID)
}

func LogIn(p *models.ParamLogIn) (*auth.TokenPair, string, error) {
	u, err := mysql.GetUserByName(p.Username)
	if err != nil {
		return nil, "", err
	}
	if u.Password != encryptPassword(p.Password) {
		return nil, "", errors.New("Wrong password")
	}

	return auth.CreateTokens(u.UserID, u.Username, p.DeviceID)
}

// RefreshTokens validates the old refresh token and returns a new pair.
func RefreshTokens(oldRefreshToken string) (*auth.TokenPair, string, error) {
	return auth.RefreshTokens(oldRefreshToken)
}

// RevokeUserTokens removes all refresh tokens for the given user.
func RevokeUserTokens(userID int64) error {
	return auth.RevokeAllTokens(userID)
}

func encryptPassword(oPassword string) string {
	h := md5.New()
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum([]byte(oPassword)))
}
