package postgres

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

type RDSAuth interface {
	getRDSCredentials() (*RDSCredentials, error)
}

type Client struct {
	Db *gorm.DB
	RDSAuth
}

type RDSCredentials struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"username"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

type AWSRDSAuth struct {
	Region     string
	SecretName string
}

func (a AWSRDSAuth) getRDSCredentials() (*RDSCredentials, error) {
	// Create a new AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(a.Region),
	})
	if err != nil {
		return nil, err
	}

	// Create a Secrets Manager client
	smClient := secretsmanager.New(sess)

	// Retrieve the secret value
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(a.SecretName),
	}

	result, err := smClient.GetSecretValue(input)
	if err != nil {
		return nil, err
	}

	// Unmarshal secret into RDSCredentials struct
	var creds RDSCredentials
	if err := json.Unmarshal([]byte(*result.SecretString), &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func (c *Client) connectToPostgres() (*gorm.DB, error) {
	creds, err := c.getRDSCredentials()
	if err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=require TimeZone=UTC",
		creds.Host, creds.User, creds.Password, creds.DBName, creds.Port,
	)

	// Initialize GORM connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to RDS Postgres: %v", err)
		return nil, err
	}
	log.Println("Connected successfully to RDS Postgres!")
	c.Db = db
	return db, nil
}
