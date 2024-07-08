package secrets

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type Secrets struct {
	prevKey        string
	prevKeyExpirey time.Time
	currentKey     string
}

type SecretService struct {
	secretName string
	secrets    Secrets
	awsClient  *secretsmanager.Client
	mutex      sync.Mutex
}
type awsSecret struct {
	EcrWebhookSecret string `json:"ecr-webhook-secret"`
}

func (ss *SecretService) Close() {
}

func NewSecretManager(smc *secretsmanager.Client, region, secretName string) (*SecretService, error) {
	slog.Info("SecretService created")
	return &SecretService{
		awsClient:  smc,
		secretName: secretName,
	}, nil
}

func (ss *SecretService) Validate(secret string) bool {
	slog.Info("validating secret")
	if secret == ss.secrets.currentKey {
		return true
	}
	if secret == ss.secrets.prevKey && time.Since(ss.secrets.prevKeyExpirey) < 0 {
		return true
	}
	return false
}

func (ss *SecretService) getCurrentKeyFromSecretManager() {
	// Get the current key from AWS Secret Manager
	slog.Info("secretService:", "secretName", ss.secretName)
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(ss.secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := ss.awsClient.GetSecretValue(context.TODO(), input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		slog.Error("failed to get client from aws", "error", err)
		return
	}

	// Decrypts secret using the associated KMS key.
	var secretString string = *result.SecretString

	var secret awsSecret
	err = json.Unmarshal([]byte(secretString), &secret)
	if err != nil {
		slog.Error("Error Unmarshalling secret")
		return
	}
	ss.secrets.currentKey = secret.EcrWebhookSecret
	slog.Info("currentKey updated successfully")
}

func (ss *SecretService) uploadCurrentKeyToSecretManager() error {
	awsSecret := awsSecret{EcrWebhookSecret: ss.secrets.currentKey}
	secretBytes, err := json.Marshal(&awsSecret)
	if err != nil {
		return err
	}
	secretString := string(secretBytes)

	input := &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(ss.secretName),
		SecretString: aws.String(secretString),
	}

	_, err = ss.awsClient.UpdateSecret(context.TODO(), input)
	return err
}

func (ss *SecretService) rotateKey() {
	slog.Info("rotating keys")
	ss.mutex.Lock()
	defer ss.mutex.Unlock()
	oldPrev := &ss.secrets.prevKey
	ss.secrets.prevKey = ss.secrets.currentKey
	// allow caller systems cache to clear before we expire the old key
	ss.secrets.prevKeyExpirey = time.Now().Add(15 * time.Minute)
	ss.secrets.currentKey = generateKey()
	err := ss.uploadCurrentKeyToSecretManager()
	if err != nil {
		// something went wrong in updating secrets
		// revert change and log
		ss.secrets.currentKey = ss.secrets.prevKey
		ss.secrets.prevKey = *oldPrev
		slog.Error("error during uploadCurrentKeyToSecretManager", err)
	}
}

func (sm *SecretService) Start() {
	slog.Info("managing secrets")
	sm.getCurrentKeyFromSecretManager()
	// sm.rotateKey()
	// Rotate the key every 24 hours
	ticker := time.NewTicker(24 * time.Hour)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				slog.Info("pretending to rotate keys")
				// sm.rotateKey()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func generateKey() string {
	key := make([]byte, 32)
	_, err := rand.Reader.Read(key)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(key)
}
