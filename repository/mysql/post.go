package mysql

import (
	"ceddit/models"
	"strings"

	"github.com/jmoiron/sqlx"
)

func CreatePost(p *models.Post) error {
	sqlStr := "insert into post (post_id, title, content, author_id, community_id) values (?, ?, ?, ?, ?)"
	_, err := db.Exec(sqlStr, p.PostID, p.Title, p.Content, p.AuthorID, p.CommunityID)
	return err
}

func GetPostByID(id int64) (*models.Post, error) {
	sqlStr := "select post_id, title, content, author_id, community_id, create_time from post where post_id = ?"
	var data models.Post
	err := db.Get(&data, sqlStr, id)
	return &data, err
}

func GetPostList(page, size int64) ([]*models.Post, error) {
	sqlStr := "select post_id, title, content, author_id, community_id, create_time from post limit ?,?"
	var data []*models.Post
	err := db.Select(&data, sqlStr, (page-1)*size, size)
	return data, err
}

func GetPostListByIDs(ids []string) ([]*models.Post, error) {
	sqlStr := "select post_id, title, content, author_id, community_id, create_time from post where post_id in (?) order by FIND_IN_SET(post_id, ?)"
	query, args, err := sqlx.In(sqlStr, ids, strings.Join(ids, ","))
	if err != nil {
		return nil, err
	}
	query = db.Rebind(query)
	var data []*models.Post
	err = db.Select(&data, query, args...)
	return data, err
}
