package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"s3-service/saladclient"
)

func main() {
	// Get environment variables
	baseURL := os.Getenv("S3_SERVICE_URL")
	token := os.Getenv("S3_SERVICE_TOKEN")

	if baseURL == "" || token == "" {
		log.Fatal("S3_SERVICE_URL and S3_SERVICE_TOKEN environment variables are required")
	}

	// Create client
	client := saladclient.NewClient(baseURL, token)
	ctx := context.Background()

	// 1. Check health
	fmt.Println("1. Checking service health...")
	health, err := client.Health(ctx)
	if err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Printf("   Status: %s\n", health.Status)

	// 2. Verify authentication
	fmt.Println("\n2. Verifying authentication...")
	authCheck, err := client.AuthCheck(ctx)
	if err != nil {
		log.Fatalf("Auth check failed: %v", err)
	}
	fmt.Printf("   Subject: %s\n", authCheck.Subject)
	fmt.Printf("   Project ID: %s\n", authCheck.ProjectID)
	fmt.Printf("   App ID: %s\n", authCheck.AppID)
	fmt.Printf("   Role: %s\n", authCheck.Role)
	fmt.Printf("   Type: %s\n", authCheck.PrincipalType)

	// 3. List bucket connections
	fmt.Println("\n3. Listing bucket connections...")
	buckets, err := client.ListBucketConnections(ctx)
	if err != nil {
		log.Fatalf("List buckets failed: %v", err)
	}
	fmt.Printf("   Found %d bucket connection(s)\n", len(buckets.Buckets))
	for _, b := range buckets.Buckets {
		fmt.Printf("   - %s (%s)\n", b.BucketName, b.Region)
	}

	// 4. List images
	fmt.Println("\n4. Listing images...")
	images, err := client.ListImages(ctx)
	if err != nil {
		log.Fatalf("List images failed: %v", err)
	}
	fmt.Printf("   Found %d image(s)\n", len(images.Images))
	for _, img := range images.Images {
		fmt.Printf("   - %s (%d bytes)\n", img.ObjectKey, img.SizeBytes)
	}

	fmt.Println("\n✅ All checks passed!")
}
