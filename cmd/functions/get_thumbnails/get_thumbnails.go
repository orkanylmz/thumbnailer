package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"log"
	"os"
	"time"
)

type Request = events.APIGatewayProxyRequest
type Response = events.APIGatewayProxyResponse

type GetThumbnailsResponse struct {
	Url string `json:"url"`
}

func handler(ctx context.Context, r Request) (Response, error) {
	fileName := r.PathParameters["name"]

	log.Printf("Filename: %s\n", fileName)

	var signedUrls []GetThumbnailsResponse
	keys, err := listS3Objects(fileName)

	if err != nil {
		return Response{StatusCode: 400, Body: err.Error()}, nil
	}

	for _, k := range keys {
		su, err := generateSignedUrlForFile(k)
		if err == nil {
			signedUrls = append(signedUrls, GetThumbnailsResponse{Url: su})
		}
	}

	b, err := json.Marshal(signedUrls)

	if err != nil {
		return Response{StatusCode: 400, Body: err.Error()}, nil
	}

	return Response{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json", "Access-Control-Allow-Origin": "*"}}, nil
}

func listS3Objects(prefix string) ([]string, error) {
	bucketName := os.Getenv("OUTPUT_BUCKET_NAME")
	region := os.Getenv("REGION")

	conf := aws.Config{Region: &region}
	sess, err := session.NewSession(&conf)

	if err != nil {
		return nil, err
	}

	svc := s3.New(sess, &conf)

	inp := s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(fmt.Sprintf("%s/", prefix)),
	}

	output, err := svc.ListObjects(&inp)
	if err != nil {
		return nil, err
	}

	var keys []string

	for _, o := range output.Contents {
		keys = append(keys, *o.Key)
	}

	return keys, nil
}

func generateSignedUrlForFile(fileName string) (string, error) {
	bucketName := os.Getenv("OUTPUT_BUCKET_NAME")
	region := os.Getenv("REGION")

	conf := aws.Config{Region: &region}
	sess, err := session.NewSession(&conf)
	if err != nil {
		return "", err
	}
	svc := s3.New(sess)

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})

	url, err := req.Presign(time.Minute * 60)
	if err != nil {
		return "", err
	}

	return url, nil
}

func main() {
	lambda.Start(handler)
}
