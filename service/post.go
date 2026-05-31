package service

import (
	"ceddit/models"
	"ceddit/pkg/cos"
	"ceddit/pkg/deepseek"
	"ceddit/pkg/snowflake"
	"ceddit/repository/mysql"
	"ceddit/repository/redis"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"
)

func CreateDraft(authorID, postID int64) error {
	return mysql.CreateDraft(postID, authorID)
}

func GetPresignedURL(authorID int64, p *models.ParamPresign) (*models.RespPresign, error) {
	post, err := mysql.GetPostByID(p.PostID)
	if err != nil {
		return nil, err
	}
	if post.AuthorID != authorID {
		return nil, errors.New("not the owner of this post")
	}

	var objectKey string
	switch p.Scene {
	case "post_content":
		objectKey = fmt.Sprintf("posts/%d/content%s", p.PostID, p.Ext)
	case "post_image":
		objectKey = fmt.Sprintf("posts/%d/images/%d%s", p.PostID, snowflake.GenID(), p.Ext)
	default:
		return nil, errors.New("unknown scene: " + p.Scene)
	}

	putURL, err := cos.GeneratePresignedPutURL(objectKey, p.ContentType)
	if err != nil {
		return nil, err
	}

	return &models.RespPresign{
		ObjectKey: objectKey,
		PutURL:    putURL,
		ExpiresIn: 600,
	}, nil
}

func ConfirmContent(postID, authorID int64, p *models.ParamContentConfirm) error {
	post, err := mysql.GetPostByID(postID)
	if err != nil {
		return err
	}
	if post.AuthorID != authorID {
		return errors.New("not the owner of this post")
	}

	etag, size, err := cos.HeadObject(p.ObjectKey)
	if err != nil {
		return fmt.Errorf("HEAD object failed: %w", err)
	}

	if etag != p.ETag {
		return fmt.Errorf("ETag mismatch: expected %s, got %s", p.ETag, etag)
	}
	if size != p.Size {
		return fmt.Errorf("size mismatch: expected %d, got %d", p.Size, size)
	}

	data, err := cos.GetObject(p.ObjectKey)
	if err != nil {
		return fmt.Errorf("GET object failed: %w", err)
	}

	hash := sha256.Sum256(data)
	gotSHA256 := hex.EncodeToString(hash[:])
	if gotSHA256 != p.SHA256 {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", p.SHA256, gotSHA256)
	}

	return mysql.UpdateContentConfirm(postID, p.ObjectKey, etag, size, p.SHA256)
}

func PatchPost(postID, authorID int64, p *models.ParamPatchPost) error {
	post, err := mysql.GetPostByID(postID)
	if err != nil {
		return err
	}
	if post.AuthorID != authorID {
		return errors.New("not the owner of this post")
	}

	return mysql.PatchPost(postID, p.Title)
}

func PublishPost(postID, authorID int64) error {
	post, err := mysql.GetPostByID(postID)
	if err != nil {
		return err
	}
	if post.AuthorID != authorID {
		return errors.New("not the owner of this post")
	}
	if post.Title == nil || *post.Title == "" {
		return errors.New("title is required before publishing")
	}
	if post.ContentObjectKey == nil || *post.ContentObjectKey == "" {
		return errors.New("content must be confirmed before publishing")
	}

	var description string
	data, err := cos.GetObject(*post.ContentObjectKey)
	if err != nil {
		return fmt.Errorf("fetch content from COS failed: %w", err)
	}
	summary, err := deepseek.GenerateSummary(context.Background(), string(data))
	if err != nil {
		zap.L().Warn("DeepSeek summary generation failed, leaving empty", zap.Error(err))
		description = ""
	} else {
		description = summary
	}

	if err := mysql.PublishPost(postID, description); err != nil {
		return err
	}

	return redis.AddPostToTimeline(postID)
}

func GetPost(id int64) (*models.PostDetail, error) {
	post, err := mysql.GetPostByID(id)
	if err != nil {
		return nil, err
	}

	user, err := mysql.GetUserByID(post.AuthorID)
	if err != nil {
		return nil, err
	}

	voteData := redis.GetPostVote([]string{strconv.FormatInt(post.PostID, 10)})
	var voteNum int64
	if len(voteData) > 0 {
		voteNum = voteData[0]
	}

	var content string
	if post.ContentObjectKey != nil && *post.ContentObjectKey != "" {
		data, err := cos.GetObject(*post.ContentObjectKey)
		if err != nil {
			zap.L().Warn("fetch post content from COS failed", zap.Error(err))
		} else {
			content = string(data)
		}
	}

	return &models.PostDetail{
		AuthorName: user.Username,
		VoteNum:    voteNum,
		Content:    content,
		Post:       post,
	}, nil
}

func GetEasyPostList(page, size int64) ([]*models.PostDetail, error) {
	var data []*models.PostDetail

	posts, err := mysql.GetPublishedPosts(page, size)
	if err != nil {
		return nil, err
	}

	for _, post := range posts {
		user, err := mysql.GetUserByID(post.AuthorID)
		if err != nil {
			zap.L().Error("mysql.GetUserByID() failed", zap.Error(err))
			continue
		}

		data = append(data, &models.PostDetail{
			AuthorName: user.Username,
			Post:       post,
		})
	}

	return data, nil
}

func GetPostList(p *models.ParamPostList) ([]*models.PostDetail, error) {
	var data []*models.PostDetail

	ids, err := redis.GetPostIDInOrder(p)
	if err != nil {
		return nil, err
	}

	posts, err := mysql.GetPublishedPostsByIDs(ids)
	if err != nil {
		return nil, err
	}

	voteData := redis.GetPostVote(ids)

	for idx, post := range posts {
		user, err := mysql.GetUserByID(post.AuthorID)
		if err != nil {
			zap.L().Error("mysql.GetUserByID() failed", zap.Error(err))
			continue
		}

		var voteNum int64
		if idx < len(voteData) {
			voteNum = voteData[idx]
		}

		data = append(data, &models.PostDetail{
			AuthorName: user.Username,
			VoteNum:    voteNum,
			Post:       post,
		})
	}

	return data, nil
}
