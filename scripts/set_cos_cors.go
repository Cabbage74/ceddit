//go:build ignore
// +build ignore

// Set COS CORS rules via API for Lighthouse bucket.
// Run: cd /home/ubuntu/ceddit && go run scripts/set_cos_cors.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/viper"
	"github.com/tencentyun/cos-go-sdk-v5"
)

func main() {
	viper.SetConfigFile("config.yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Sprintf("read config: %v", err))
	}

	u, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com",
		viper.GetString("cos.bucket"),
		viper.GetString("cos.region"),
	))
	if err != nil {
		panic(fmt.Sprintf("parse COS URL: %v", err))
	}

	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  viper.GetString("cos.secret_id"),
			SecretKey: viper.GetString("cos.secret_key"),
		},
	})

	opt := &cos.BucketPutCORSOptions{
		Rules: []cos.BucketCORSRule{
			{
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"PUT", "GET", "HEAD"},
				AllowedHeaders: []string{"Content-Type", "Authorization", "Content-Length"},
				ExposeHeaders:  []string{"ETag", "Content-Length", "x-cos-request-id"},
				MaxAgeSeconds:  600,
			},
		},
	}

	_, err = client.Bucket.PutCORS(context.Background(), opt)
	if err != nil {
		panic(fmt.Sprintf("put CORS: %v", err))
	}

	fmt.Println("CORS 规则设置成功!")

	// Verify
	result, _, err := client.Bucket.GetCORS(context.Background())
	if err != nil {
		fmt.Printf("读取 CORS 验证失败: %v\n", err)
		return
	}
	fmt.Println("当前 CORS 规则:")
	for _, rule := range result.Rules {
		fmt.Printf("  Origins:  %v\n", rule.AllowedOrigins)
		fmt.Printf("  Methods:  %v\n", rule.AllowedMethods)
		fmt.Printf("  Headers:  %v\n", rule.AllowedHeaders)
		fmt.Printf("  Expose:   %v\n", rule.ExposeHeaders)
		fmt.Println()
	}
}
