package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"s3-service/saladclient"
)

func main() {
	baseURL := os.Getenv("S3_SERVICE_URL")
	token := os.Getenv("S3_SERVICE_TOKEN")

	if baseURL == "" || token == "" {
		log.Fatal("S3_SERVICE_URL and S3_SERVICE_TOKEN environment variables are required")
	}

	client := saladclient.NewClient(baseURL, token)
	ctx := context.Background()

	bucketName := "my-bucket"
	objectKey := "example/file.txt"

	// 1. Upload an object
	fmt.Println("1. Uploading object...")
	data := []byte("Hello, s3-service!")

	uploadResp, err := client.UploadObjectWithData(
		ctx,
		bucketName,
		objectKey,
		"text/plain",
		data,
		map[string]string{
			"source": "example",
		},
	)
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
	fmt.Printf("   Uploaded: %s\n", uploadResp.ObjectKey)
	fmt.Printf("   ETag: %s\n", uploadResp.ETag)
	fmt.Printf("   Size: %d bytes\n", uploadResp.Size)

	// 2. Get a presigned download URL
	fmt.Println("\n2. Generating presigned download URL...")
	downloadPresign, err := client.PresignDownloadURL(ctx, bucketName, objectKey)
	if err != nil {
		log.Fatalf("Presign download failed: %v", err)
	}
	fmt.Printf("   Download URL: %s\n", downloadPresign.URL)
	fmt.Printf("   Method: %s\n", downloadPresign.Method)
	fmt.Printf("   Expires at: %s\n", downloadPresign.ExpiresAt)

	// 3. Get a presigned upload URL
	fmt.Println("\n3. Generating presigned upload URL...")
	uploadPresign, err := client.PresignUploadURL(ctx, bucketName, "example/file2.txt")
	if err != nil {
		log.Fatalf("Presign upload failed: %v", err)
	}
	fmt.Printf("   Upload URL: %s\n", uploadPresign.URL)
	fmt.Printf("   Method: %s\n", uploadPresign.Method)
	fmt.Printf("   Expires at: %s\n", uploadPresign.ExpiresAt)

	// 4. Delete the object
	fmt.Println("\n4. Deleting object...")
	deleteResp, err := client.DeleteObject(ctx, bucketName, objectKey)
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	fmt.Printf("   Deleted: %v\n", deleteResp.Deleted)

	fmt.Println("\n✅ Upload/download operations completed!")
}
