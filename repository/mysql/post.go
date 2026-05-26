package mysql

import (
	"ceddit/models"
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
	err := db.Select(&data, sqlStr, (page - 1) * size, size)
	return data, err
}
