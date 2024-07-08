package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

type AwsClient struct {
	client *ecr.Client
	token  string
	mutex  sync.Mutex
	log    *slog.Logger
}

func CreateEcrClient() *ecr.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}
	return ecr.NewFromConfig(cfg)
}

func (a *AwsClient) GetAuthStr() (string, error) {
	resp, err := a.getAuthorizationToken()
	if err != nil {
		return "", err
	}
	username, pwd, err := tokenFromAuthStr(resp)
	if err != nil {
		return "", err
	}
	jsonBytes, _ := json.Marshal(map[string]string{
		"username": username,
		"password": pwd,
	})

	return base64.URLEncoding.EncodeToString(jsonBytes), nil
}

func (a *AwsClient) getAuthorizationToken() (string, error) {
	if a.token == "" {
		return "", errors.New("No token available")
	}
	return a.token, nil
}
func (a *AwsClient) UpdateToken() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	token, err := a.retrieveToken()
	if err != nil {
		return err
	}
	a.token = token
	return nil
}

func (a *AwsClient) retrieveToken() (string, error) {
	resp, err := a.client.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", err
	}

	if len(resp.AuthorizationData) == 0 {
		return "", errors.New("no authorization data in response")
	}

	return *resp.AuthorizationData[0].AuthorizationToken, nil
}

func tokenFromAuthStr(authStr string) (string, string, error) {
	decodedToken, err := base64.StdEncoding.DecodeString(authStr)
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(string(decodedToken), ":")
	if len(parts) != 2 {
		return "", "", errors.New("invalid token format")
	}
	return parts[0], parts[1], nil
}
func (a *AwsClient) startTokenRefresh() {
	// Refresh token every 11 hours (1 hour before it expires)
	ticker := time.NewTicker(11 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := a.UpdateToken()
			if err != nil {
				a.log.Error("Failed to refresh authorization token", err.Error(), "\n")
				continue
			}
			a.log.Info("Refreshed token: ")
		}
	}
}
func NewAwsClient(client *ecr.Client) *AwsClient {
	awsClient := &AwsClient{
		client: client,
	}
	go awsClient.startTokenRefresh()
	return awsClient
}

func (a *AwsClient) Close() {
	// nothing to close atm
}
