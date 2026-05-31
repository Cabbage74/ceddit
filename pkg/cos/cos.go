package cos

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/viper"
	"github.com/tencentyun/cos-go-sdk-v5"
)

var client *cos.Client

func Init() error {
	u, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com",
		viper.GetString("cos.bucket"),
		viper.GetString("cos.region"),
	))
	if err != nil {
		return fmt.Errorf("parse COS URL: %w", err)
	}

	b := &cos.BaseURL{BucketURL: u}
	client = cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  viper.GetString("cos.secret_id"),
			SecretKey: viper.GetString("cos.secret_key"),
		},
	})

	return nil
}

func GeneratePresignedPutURL(objectKey, contentType string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("COS client not initialized")
	}

	putURL, err := client.Object.GetPresignedURL(
		context.Background(),
		http.MethodPut,
		objectKey,
		viper.GetString("cos.secret_id"),
		viper.GetString("cos.secret_key"),
		10*time.Minute,
		&cos.PresignedURLOptions{
			Query:  &url.Values{},
			Header: &http.Header{"Content-Type": []string{contentType}},
		},
	)
	if err != nil {
		return "", fmt.Errorf("generate presigned URL: %w", err)
	}

	return putURL.String(), nil
}

func HeadObject(objectKey string) (etag string, size int64, err error) {
	if client == nil {
		return "", 0, fmt.Errorf("COS client not initialized")
	}

	resp, err := client.Object.Head(context.Background(), objectKey, nil)
	if err != nil {
		return "", 0, fmt.Errorf("HEAD object: %w", err)
	}

	etag = resp.Header.Get("ETag")
	size = resp.ContentLength
	return etag, size, nil
}

func GetObject(objectKey string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("COS client not initialized")
	}

	resp, err := client.Object.Get(context.Background(), objectKey, nil)
	if err != nil {
		return nil, fmt.Errorf("GET object: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func DeleteObject(objectKey string) error {
	if client == nil {
		return fmt.Errorf("COS client not initialized")
	}

	_, err := client.Object.Delete(context.Background(), objectKey)
	return err
}
