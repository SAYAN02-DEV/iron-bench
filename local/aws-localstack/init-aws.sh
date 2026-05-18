#!/bin/bash
echo "Creating AWS resources..."

aws --endpoint-url=http://localhost:4566 sqs create-queue \
  --queue-name my-queue \
  --region us-east-1

aws --endpoint-url=http://localhost:4566 s3 mb s3://my-bucket \
  --region us-east-1

echo "Done!"