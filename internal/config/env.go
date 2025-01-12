package config

import (
	"errors"
	"github.com/joho/godotenv"
	"log"
	"os"
)

type Env struct {
	MisskeyToken        string
	MisskeyHostURL      string
	AWSRegion           string
	AWSSecretNameForRDS string
}

// LoadEnv loads environment variables from a .env file and populates the Env struct,
// checking for required values.
func LoadEnv() (*Env, error) {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v\n", err)
	}

	// Fetch and validate required environment variables
	misskeyToken := os.Getenv("MISSKEY_TOKEN")
	if misskeyToken == "" {
		return nil, errors.New("MISSKEY_TOKEN is required but not set")
	}

	misskeyHostURL := os.Getenv("MISSKEY_HOST_URL")
	if misskeyHostURL == "" {
		return nil, errors.New("MISSKEY_HOST_URL is required but not set")
	}

	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		return nil, errors.New("AWS_REGION is required but not set")
	}

	awsSecretNameForRDS := os.Getenv("AWS_SECRET_NAME_FOR_RDS")
	if awsSecretNameForRDS == "" {
		return nil, errors.New("AWS_SECRET_NAME_FOR_RDS is required but not set")
	}

	// Create and populate the Env struct
	env := &Env{
		MisskeyToken:        misskeyToken,
		MisskeyHostURL:      misskeyHostURL,
		AWSRegion:           awsRegion,
		AWSSecretNameForRDS: awsSecretNameForRDS,
	}

	return env, nil
}
