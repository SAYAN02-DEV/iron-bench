package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	appconfig "github.com/SAYAN02-DEV/iron-bench/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func RunLongPolling() {
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

	baseURL := "http://localhost:" + cfg.OrchestratorPort
	httpClient := &http.Client{Timeout: 15 * time.Second}

	log.Println("Consumer started, waiting for messages...")

	for {
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

		buffer = append(buffer, resp.Messages...)
		fmt.Printf("Buffer: %d/%d\n", len(buffer), maxMessages)

		if len(buffer) >= maxMessages {
			batch := buffer[:maxMessages]
			buffer = buffer[maxMessages:]

			if err := sendDownloadRequest(httpClient, baseURL, batch); err != nil {
				log.Println("download request failed:", err)
				buffer = append(batch, buffer...)
				continue
			}

			processBatch(client, cfg.SQSURL, batch)
		}
	}
}

func sendDownloadRequest(client *http.Client, baseURL string, batch []types.Message) error {
	requests := make([]FileRequest, 0, len(batch))
	for _, msg := range batch {
		if msg.Body == nil || *msg.Body == "" {
			return fmt.Errorf("message body is empty")
		}
		var req FileRequest
		if err := json.Unmarshal([]byte(*msg.Body), &req); err != nil {
			return fmt.Errorf("invalid message body: %w", err)
		}
		requests = append(requests, req)
	}

	payload, err := json.Marshal(requests)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/file/download", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download API returned %s", resp.Status)
	}
	return nil
}

func processBatch(client *sqs.Client, queueURL string, batch []types.Message) {
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

