package docker

import (
	"testing"

	"ljos.app/ecr-change-receiver/image_watcher/aws"
)

func TestPull(t *testing.T) {
	awsClient := aws.NewAwsClient(aws.CreateEcrClient())
	d := NewDockerClient(awsClient)
	d.PullImage("dummy")
}
