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
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/go-cmd/cmd"
	"github.com/google/uuid"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Request = events.APIGatewayProxyRequest
type Response = events.APIGatewayProxyResponse

type ThumbnailerRequest struct {
	Filename string `json:"filename"`
	Seconds  int    `json:"seconds"`
}

func handler(ctx context.Context, r Request) (Response, error) {
	log.Printf("BODY: %v\n", r)

	var thumbnailerReq ThumbnailerRequest
	err := json.Unmarshal([]byte(r.Body), &thumbnailerReq)
	if err != nil {
		return Response{StatusCode: 400, Body: err.Error()}, nil
	}

	log.Printf("Filename: %s\n", thumbnailerReq.Filename)

	signedUrl, err := generateSignedUrlForFile(thumbnailerReq.Filename)

	targetFolder := fmt.Sprintf("/tmp/%s", thumbnailerReq.Filename)

	createTempFolderCmd := cmd.NewCmd("mkdir", "-p", targetFolder)

	s := 5

	if thumbnailerReq.Seconds != 0 {
		s = thumbnailerReq.Seconds
	}

	<-createTempFolderCmd.Start()

	ffmpegCmd := cmd.NewCmd("ffmpeg", "-i", signedUrl, "-vf", fmt.Sprintf("select='not(mod(t,%d))',setpts=N/FRAME_RATE/TB", s), fmt.Sprintf("%s/output_%%04d.jpg", targetFolder))

	// Run and wait for Cmd to return Status
	<-ffmpegCmd.Start()
	log.Printf("Uploading To S3...\n")
	uploadDirToS3(ctx, thumbnailerReq.Filename)

	return Response{StatusCode: 200, Body: "uploaded thumbnails", Headers: map[string]string{"Content-Type": "application/json", "Access-Control-Allow-Origin": "*"}}, nil
}

func uploadDirToS3(ctx context.Context, root string) {
	var files []string

	_ = filepath.WalkDir(fmt.Sprintf("/tmp/%s", root), func(path string, d fs.DirEntry, err error) error {
		files = append(files, path)
		return nil
	})

	var wg sync.WaitGroup
	wg.Add(len(files[1:]))

	for _, pathOfFile := range files[1:] {

		pathOfFile := pathOfFile
		go func(path string) {
			err := uploadToS3(ctx, root, path)
			if err != nil {
				log.Printf("uploadToS3 Err: %v\n", err)
			}
			defer wg.Done()
		}(pathOfFile)
	}
	wg.Wait()

}

func uploadToS3(ctx context.Context, parentName, pathOfFile string) error {

	file, err := os.Open(pathOfFile)
	if err != nil {
		return err
	}

	defer file.Close()

	bucketName := os.Getenv("OUTPUT_BUCKET_NAME")
	region := os.Getenv("REGION")

	conf := aws.Config{Region: &region}
	sess, err := session.NewSession(&conf)

	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s/%s", parentName, fmt.Sprintf("%s.jpg", uuid.New().String()))

	uploader := s3manager.NewUploader(sess)
	uploadParams := &s3manager.UploadInput{
		Bucket: &bucketName,
		Key:    &fileName,
		Body:   file,
	}

	_, err = uploader.UploadWithContext(ctx, uploadParams)

	if err != nil {
		return err
	}

	return nil
}

func generateSignedUrlForFile(fileName string) (string, error) {
	bucketName := os.Getenv("BUCKET_NAME")
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
