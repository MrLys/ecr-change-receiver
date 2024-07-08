package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

type AwsClient struct {
	client *ecr.Client
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

func NewAwsClient(client *ecr.Client) *AwsClient {
	return &AwsClient{
		client: client,
	}
}

func (a *AwsClient) Close() {
	// nothing to close atm
}
