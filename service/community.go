package service

import (
	"ceddit/models"
	"ceddit/repository/mysql"
)

func GetCommunityList() ([]*models.Community, error) {
	return mysql.GetCommunitys()
}

func GetCommunity(id int64) (*models.Community, error) {
	return mysql.GetCommunityByID(id)
}
