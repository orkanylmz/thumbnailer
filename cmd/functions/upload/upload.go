package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	session2 "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"mime/multipart"
	"net/http"
	"time"

	"io"
	"os"
)

type Request = events.APIGatewayProxyRequest
type Response = events.APIGatewayProxyResponse

type UploadResponse struct {
	FileName string `json:"filename"`
}

func handler(ctx context.Context, req Request) (Response, error) {

	resp := events.APIGatewayProxyResponse{Headers: make(map[string]string)}
	resp.Headers["Access-Control-Allow-Origin"] = "*"

	file, err := parseMultipartFile("file", req)

	if err != nil {
		return Response{StatusCode: 500, Body: err.Error()}, nil
	}

	defer file.Close()

	fileName := fmt.Sprintf("%d.mp4", time.Now().Unix())

	err = uploadToS3(ctx, file, fileName)

	r := UploadResponse{FileName: fileName}

	b, err := json.Marshal(r)

	if err != nil {
		return Response{StatusCode: 400, Body: err.Error()}, nil
	}

	return Response{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json", "Access-Control-Allow-Origin": "*"},
	}, nil
}

func main() {
	lambda.Start(handler)
}

func parseMultipartFile(fieldName string, req Request) (multipart.File, error) {
	r := http.Request{}
	r.Header = make(map[string][]string)
	for k, v := range req.Headers {
		if k == "content-type" || k == "Content-Type" {
			r.Header.Set(k, v)
		}
	}

	// NOTE: API Gateway is set up with */* as binary media type, so all APIGatewayProxyRequests will be base64 encoded
	body, err := base64.StdEncoding.DecodeString(req.Body)

	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		return nil, err
	}

	file, _, err := r.FormFile(fieldName)

	if err != nil {
		return nil, err
	}

	return file, nil
}

func uploadToS3(ctx context.Context, file io.Reader, filename string) error {
	bucketName := os.Getenv("BUCKET_NAME")
	region := os.Getenv("REGION")

	conf := aws.Config{Region: &region}
	session, err := session2.NewSession(&conf)

	if err != nil {
		return err
	}
	uploader := s3manager.NewUploader(session)
	uploadParams := &s3manager.UploadInput{
		Bucket: &bucketName,
		Key:    &filename,
		Body:   file,
	}

	result, err := uploader.UploadWithContext(ctx, uploadParams)

	if err != nil {
		return err
	}

	fmt.Println(result.UploadID)

	return nil
}
