package docker

import (
	"context"
	"log/slog"
	"os"

	"github.com/docker/docker/api/types/image"
	dockerClient "github.com/docker/docker/client"
	"ljos.app/ecr-change-receiver/image_watcher/aws"
)

type DockerClient struct {
	apiClient *dockerClient.Client
	awsClient *aws.AwsClient
	log       *slog.Logger
}

func NewDockerClient(awsClient *aws.AwsClient) *DockerClient {
	apiClient, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv, dockerClient.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return &DockerClient{
		apiClient: apiClient,
		awsClient: awsClient,
		log:       log,
	}
}
func (d *DockerClient) PullImage(refString string) bool {
	authStr, err := d.awsClient.GetAuthStr()
	if err != nil {
		d.log.Error("PullImage - Failed to get auth string:", err)
		return false
	}
	opts := &image.PullOptions{
		RegistryAuth: authStr,
	}

	res, err := d.apiClient.ImagePull(context.Background(), refString, *opts)
	if err != nil {
		d.log.Error("PullImage - Failed to pull image:", err)
		return false
	}
	defer res.Close()
	d.log.Info("PullImage - Image pulled successfully", "image", refString)
	return true
}
