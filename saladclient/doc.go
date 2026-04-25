// Package saladclient provides a reusable Go SDK for consuming the s3-service API.
//
// This package exports all types and a complete HTTP client for interacting with
// s3-service from other applications and services.
//
// Example:
//
//	client := saladclient.NewClient("https://api.example.com", "jwt-token")
//	authCheck, err := client.AuthCheck(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Authenticated as: %s\n", authCheck.Subject)
package saladclient
