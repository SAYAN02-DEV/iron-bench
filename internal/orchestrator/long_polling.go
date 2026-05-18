package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	appconfig "github.com/SAYAN02-DEV/iron-bench/internal/config"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func main() {
	// Setup

	cfg, err := appconfig.Load()
	if err != nil {
		log.Println(".env not found; falling back to environment variables")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}
	accessKey := os.Getenv("AWS_KEY")
	if accessKey == "" {
		accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	secretKey := os.Getenv("AWS_SECRET")
	if secretKey == "" {
		secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	endpointURL := os.Getenv("AWS_URL")

	loadOpts := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(region),
	}
	if accessKey != "" && secretKey != "" {
		loadOpts = append(loadOpts, awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	}
	if endpointURL != "" {
		loadOpts = append(loadOpts, awscfg.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpointURL, HostnameImmutable: true}, nil
			}),
		))
	}

	awsCfg, err := awscfg.LoadDefaultConfig(context.TODO(), loadOpts...)
	if err != nil {
		log.Fatal("Failed to load AWS config:", err)
	}

	client := sqs.NewFromConfig(awsCfg)
	buffer := []types.Message{}

	if cfg.SQSURL == "" {
		log.Fatal("SQS_URL is not set")
	}

	waitSeconds, err := strconv.ParseInt(cfg.LongPollingSeconds, 10, 64)
	if err != nil {
		log.Fatal("invalid LONG_POLLING_SECONDS:", err)
	}
	if waitSeconds < 0 || waitSeconds > 20 {
		log.Fatal("LONG_POLLING_SECONDS must be between 0 and 20")
	}

	maxMessagesParsed, err := strconv.ParseInt(cfg.SQSMaxMessages, 10, 32)
	if err != nil {
		log.Fatal("invalid SQS_MAX_MESSAGES:", err)
	}
	if maxMessagesParsed < 1 || maxMessagesParsed > 10 {
		log.Fatal("SQS_MAX_MESSAGES must be between 1 and 10")
	}
	maxMessages := int(maxMessagesParsed)

	log.Println("Consumer started, waiting for messages...")

	for {
		// Long poll — blocks here up to 20s
		resp, err := client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			QueueUrl:            &cfg.SQSURL,
			MaxNumberOfMessages: int32(maxMessages),
			WaitTimeSeconds:     int32(waitSeconds),
		})
		if err != nil {
			log.Println("Error receiving messages:", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Accumulate into buffer
		buffer = append(buffer, resp.Messages...)
		fmt.Printf("Buffer: %d/%d\n", len(buffer), maxMessages)

		// Process when we hit maxMessages
		if len(buffer) >= maxMessages {
			batch := buffer[:maxMessages]
			buffer = buffer[maxMessages:] // keep remainder

			processBatch(client, cfg.SQSURL, batch)
		}
	}
}

func processBatch(client *sqs.Client, queueURL string, batch []types.Message) {
	fmt.Printf("Processing batch of %d messages\n", len(batch))

	// Your processing logic here
	for _, msg := range batch {
		fmt.Println("Message:", *msg.Body)
	}

	// Delete after successful processing
	entries := make([]types.DeleteMessageBatchRequestEntry, len(batch))
	for i, msg := range batch {
		id := fmt.Sprintf("%d", i)
		entries[i] = types.DeleteMessageBatchRequestEntry{
			Id:            &id,
			ReceiptHandle: msg.ReceiptHandle,
		}
	}

	_, err := client.DeleteMessageBatch(context.TODO(), &sqs.DeleteMessageBatchInput{
		QueueUrl: &queueURL,
		Entries:  entries,
	})
	if err != nil {
		log.Println("Error deleting messages:", err)
	} else {
		fmt.Println("Batch deleted from queue")
	}
}
