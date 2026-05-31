package mysql

import (
	"ceddit/models"
	"strings"

	"github.com/jmoiron/sqlx"
)

const postColumns = `id, title, description, content_object_key,
	content_etag, content_size, content_sha256, author_id, status,
	create_time, publish_time, update_time`

func CreateDraft(postID, authorID int64) error {
	sqlStr := "INSERT INTO post (id, author_id, status) VALUES (?, ?, 'draft')"
	_, err := db.Exec(sqlStr, postID, authorID)
	return err
}

func GetPostByID(id int64) (*models.Post, error) {
	sqlStr := "SELECT " + postColumns + " FROM post WHERE id = ?"
	var data models.Post
	err := db.Get(&data, sqlStr, id)
	return &data, err
}

func UpdateContentConfirm(postID int64, objectKey, etag string, size int64, sha256 string) error {
	sqlStr := `UPDATE post SET
		content_object_key = ?, content_etag = ?, content_size = ?, content_sha256 = ?
		WHERE id = ?`
	_, err := db.Exec(sqlStr, objectKey, etag, size, sha256, postID)
	return err
}

func PatchPost(postID int64, title string) error {
	sqlStr := "UPDATE post SET title = ? WHERE id = ?"
	_, err := db.Exec(sqlStr, title, postID)
	return err
}

func PublishPost(postID int64, description string) error {
	sqlStr := `UPDATE post SET
		status = 'published', description = ?, publish_time = NOW()
		WHERE id = ?`
	_, err := db.Exec(sqlStr, description, postID)
	return err
}

func DeletePost(postID int64) error {
	sqlStr := "DELETE FROM post WHERE id = ?"
	_, err := db.Exec(sqlStr, postID)
	return err
}

func GetPublishedPosts(page, size int64) ([]*models.Post, error) {
	sqlStr := "SELECT " + postColumns + " FROM post WHERE status = 'published' ORDER BY create_time DESC LIMIT ?, ?"
	var data []*models.Post
	err := db.Select(&data, sqlStr, (page-1)*size, size)
	return data, err
}

func GetPublishedPostsByIDs(ids []string) ([]*models.Post, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	sqlStr := "SELECT " + postColumns + " FROM post WHERE id IN (?) ORDER BY FIND_IN_SET(id, ?)"
	query, args, err := sqlx.In(sqlStr, ids, strings.Join(ids, ","))
	if err != nil {
		return nil, err
	}
	query = db.Rebind(query)
	var data []*models.Post
	err = db.Select(&data, query, args...)
	return data, err
}
