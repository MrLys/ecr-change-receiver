package image_watcher

import (
	"os"

	"gopkg.in/yaml.v3"
)

type WatchedImageConfig struct {
	// RepositoryName is the name of the ECR repository.
	RepositoryName string `yaml:"repositoryName"`
	// repositoryUri is the URI of the ECR repository.
	RepositoryUri string `yaml:"repositoryUri"`
	// ImageTagPrefix prefix
	ImageTagPrefix string `yaml:"imageTagPrefix"`
}
type Config struct {
	// Port is the port on which the server listens for incoming requests.
	// SecretName is the name of the secret in AWS Secrets Manager that contains the current key.
	SecretName    string               `yaml:"secretName"`
	WatchedImages []WatchedImageConfig `yaml:"watchedImages"`
}

func newConfig() *Config {
	c := &Config{}
	data, err := os.ReadFile("./conf/conf.yml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, c)
	if err != nil {
		panic(err)
	}
	return c
}
