package service

import (
	"ceddit/models"
	"ceddit/repository/mysql"

	"go.uber.org/zap"
)

func CreatePost(p *models.Post) error {
	return mysql.CreatePost(p)
}

func GetPost(id int64) (*models.PostDetail, error) {
	var data models.PostDetail

	post, err := mysql.GetPostByID(id)
	if err != nil {
		return nil, err
	}

	user, err := mysql.GetUserByID(post.AuthorID)
	if err != nil {
		return nil, err
	}

	community, err := mysql.GetCommunityByID(post.CommunityID)
	if err != nil {
		return nil, err
	}


	data.AuthorName = user.Username
	data.CommunityName = community.CommunityName
	data.Post = post
	return &data, nil
}

func GetPostList(page, size int64) ([]*models.PostDetail, error) {
	var data []*models.PostDetail

	posts, err := mysql.GetPostList(page, size)
	if err != nil {
		return nil, err
	}
	
	for _, post := range posts {
		var cur models.PostDetail

		user, err := mysql.GetUserByID(post.AuthorID)
		if err != nil {
			zap.L().Error("mysql.GetUserByID() failed", zap.Error(err))
			continue
		}

		community, err := mysql.GetCommunityByID(post.CommunityID)
		if err != nil {
			zap.L().Error("mysql.GetCommunityByID() failed", zap.Error(err))	
			continue
		}
		
		cur.AuthorName = user.Username
		cur.CommunityName = community.CommunityName
		cur.Post = post
		data = append(data, &cur)
	}

	return data, nil
}
