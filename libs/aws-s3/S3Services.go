package aws_s3

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

func requireEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		log.Fatalf("%s is not set", key)
	}
	return val
}

func getEnvInt64(key string, fallback int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		log.Fatalf("invalid %s: %v", key, err)
	}
	return parsed
}

func main() {
	_ = godotenv.Load()

	ctx := context.Background()

	region := requireEnv("AWS_REGION")
	accessKey := requireEnv("AWS_KEY")
	secretKey := requireEnv("AWS_SECRET")
	endpointURL := requireEnv("AWS_URL")
	bucket := requireEnv("AWS_BUCKET")
	key := requireEnv("AWS_OBJECT_KEY")
	postKey := requireEnv("AWS_POST_KEY")
	expires := getEnvInt64("AWS_PRESIGN_EXPIRES", 3600)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpointURL,
					HostnameImmutable: true,
				}, nil
			}),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	presignClient := s3.NewPresignClient(s3Client)

	presigner := Presigner{PresignClient: presignClient}

	//put
	putReq, err := presigner.PutObject(ctx, bucket, key, expires)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("PUT URL:", putReq.URL)

	// put url
	uploadResp, err := http.NewRequest(http.MethodPut, putReq.URL, strings.NewReader("hello localstack!"))
	if err != nil {
		log.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(uploadResp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Upload status:", resp.Status)

	//get url
	getReq, err := presigner.GetObject(ctx, bucket, key, expires)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("GET URL:", getReq.URL)

	//get url to download
	getResp, err := http.Get(getReq.URL)
	if err != nil {
		log.Fatal(err)
	}
	defer getResp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, getResp.Body); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Downloaded content:", buf.String()) // hello localstack!

	//delete
	delReq, err := presigner.DeleteObject(ctx, bucket, key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("DELETE URL:", delReq.URL)

	//post
	postReq, err := presigner.PresignPostObject(ctx, bucket, postKey, expires)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("POST URL:", postReq.URL)
	fmt.Println("POST Fields:", postReq.Values)
}
