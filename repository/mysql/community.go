package mysql

import (
	"ceddit/models"
	"database/sql"
)

func GetCommunitys() ([]*models.Community, error) {
	sqlStr := "select community_id, community_name from community"
	var data []*models.Community
	if err := db.Select(&data, sqlStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}
