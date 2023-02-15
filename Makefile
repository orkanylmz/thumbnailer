MY_AWS_PROFILE = "default"

GOOS=linux
GOARCH=amd64
CGO_ENABLED=0

# this spins up a local serverless stack and runs it
run:
	AWS_PROFILE=${MY_AWS_PROFILE} sam local start-api

build-run:
	make build && make run

# the first step is to build the binaries
build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 sam build

# this packages up your deployment assets and uploads them
# to S3
package:
	AWS_PROFILE=${MY_AWS_PROFILE} sam package \
		--s3-bucket thumbnailer-resources \
		--output-template-file ./.aws-sam/packaged.yaml

# this will trigger the actual deploy (updating Lambda and such)
deploy:
	AWS_PROFILE=${MY_AWS_PROFILE} make build && make package && sam deploy --no-confirm-changeset \
		--no-fail-on-empty-changeset \
		--s3-bucket thumbnailer-resources \
		--stack-name thumbnailer-api \
		--capabilities CAPABILITY_IAM