package main

import (
	"log/slog"
	"os"

	"ljos.app/ecr-change-receiver/web"
)

func main() {
	secretName := os.Getenv("AWS_ECR_WEBHOOK_SECRET_NAME")
	accessKey := os.Getenv("AWS_ECR_WEBHOOK_ACCESS_KEY")
	accessSecret := os.Getenv("AWS_ECR_WEBHOOK_ACCESS_SECRET")
	region := os.Getenv("AWS_ECR_WEBHOOK_REGION")
	webServer := web.NewWeb(accessKey, accessSecret, region, secretName)
	defer webServer.Close()
	slog.Info("Starting web server")
	webServer.Start()
}
