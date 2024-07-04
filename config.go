package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Port is the port on which the server listens for incoming requests.
	watchedImages []struct {
		// SecretName is the name of the secret in AWS Secrets Manager that contains the current key.
		SecretName string `yaml:"secretName"`
		// RepositoryName is the name of the ECR repository.
		RepositoryName string `yaml:"repositoryName"`
		// repositoryUri is the URI of the ECR repository.
		repositoryUri string `yaml:"repositoryUri"`
	} `yaml:"watchedImages"`
}

func (c *Config) init() {
	data, err := os.ReadFile("./conf/conf.yml")
	if err != nil {
		panic(err)
	}
	yaml.Unmarshal(data, c)
}
