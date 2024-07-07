package image_watcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"

	//"github.com/aws/aws-sdk-go/aws"
	//"github.com/aws/aws-sdk-go/aws/session"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
)

type PasswordPartAwsCredentials struct {
	Payload    string `json:"payload"`
	DataKey    string `json:"dataKey"`
	Version    string `json:"version"`
	Type       string `json:"type"`
	Expiration int    `json:"expiration"`
}
type ImageWatcher struct {
	accessId      string
	accessSecret  string
	region        string
	watchedImages map[string]WatchedImage
	apiClient     *client.Client
	awsClient     *ecr.Client
	mutex         sync.Mutex
}
type Image struct {
	RepositoryName   string
	RepositoryUri    string
	ImageTag         string
	StartTime        time.Time
	PreviousImageTag string
	containerID      string
}
type WatchedImage struct {
	images map[string]Image
}

func (i *ImageWatcher) Close() {
	// close the docker client
	i.apiClient.Close()
}
func (i *ImageWatcher) RemoveImage(image string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	delete(i.watchedImages, image)
}

func NewImageWatcher(region string) *ImageWatcher {
	return &ImageWatcher{
		region: region,
	}
}
func (iw *ImageWatcher) Start() {
	slog.Info("Starting image watcher")
	iw.watchedImages = make(map[string]WatchedImage)
	config := Config{}
	config.init()
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	iw.apiClient = apiClient

	containers, err := iw.apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		panic(err)
	}
	iw.initializeWatcherImages(&config, containers)
}

func (iw *ImageWatcher) initializeWatcherImages(config *Config, containers []types.Container) {

	for _, image := range config.WatchedImages {
		key := image.RepositoryName
		existingImages, ok := iw.watchedImages[key]
		var wi WatchedImage
		if ok {
			wi = existingImages
		} else {
			wi = WatchedImage{
				images: make(map[string]Image),
			}
		}
		im := Image{
			RepositoryName:   image.RepositoryName,
			RepositoryUri:    image.RepositoryUri,
			ImageTag:         "",
			StartTime:        time.Now(),
			PreviousImageTag: "",
		}

		for _, ctr := range containers {
			firstPart := fmt.Sprintf("%s%s:%s", image.RepositoryUri, image.RepositoryName, image.ImageTagPrefix)
			parts := strings.Split(ctr.Image, firstPart)
			if len(parts) != 2 {
				fmt.Printf("skipping container %s %s\n", firstPart, parts)
				continue
			}
			imageTag := strings.Split(ctr.Image, ":")[1]
			im.ImageTag = imageTag
			im.containerID = ctr.ID
			slog.Info("Found container", "container-id", ctr.ID, "image-tag", imageTag)
		}
		wi.images[image.ImageTagPrefix] = im
		iw.watchedImages[key] = wi
	}

}

func (i *ImageWatcher) UpdateImage(image string, imageTag string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	slog.Info("UpdatedImage(start)", "image", image, "image-tag", imageTag)
	watchedImages, ok := i.watchedImages[image]
	if !ok {
		slog.Info("watchedImages", i.watchedImages)
		return
	}
	for prefix, watchedImage := range watchedImages.images {
		if strings.HasPrefix(imageTag, prefix) {
			watchedImage.PreviousImageTag = watchedImage.ImageTag
			watchedImage.ImageTag = imageTag
			watchedImage.StartTime = time.Now()
			if watchedImage.containerID == "" {
				slog.Info("UpdatedImage(container not started)", "image", image, "image-tag", imageTag)
				// pull new image and start container
				res, err := i.apiClient.ImagePull(context.Background(), fmt.Sprintf("%s%s:%s", watchedImage.RepositoryUri, watchedImage.RepositoryName, imageTag), dockerImage.PullOptions{})
				if err != nil {
					slog.Error("Failed to pull image", "error", err)
					return
				}
				res.Close()
				// start container
				resp, err := i.apiClient.ContainerCreate(context.Background(), &container.Config{}, &container.HostConfig{}, nil, nil, "")
				if err != nil {
					slog.Error("Failed to create container", "error", err)
					return
				}
				err = i.apiClient.ContainerStart(context.Background(), resp.ID, container.StartOptions{})
				if err != nil {
					slog.Error("Failed to start container", "error", err)
					return
				}
				watchedImage.containerID = resp.ID
				slog.Info("UpdatedImage(container started)", "image", image, "image-tag", imageTag, "container-id", resp.ID)
				return
			}
			slog.Info("UpdatedImage(container already started)", "image", image, "image-tag", imageTag, "container-id", watchedImage.containerID)
			watchedImages.images[prefix] = watchedImage
			i.watchedImages[image] = watchedImages
			slog.Info("UpdatedImage(done)", "image", image, "image-tag", imageTag)
			// pull new image and restart container
			err := i.apiClient.ContainerStop(context.Background(), watchedImage.containerID, container.StopOptions{})
			if err != nil {
				slog.Error("Failed to stop container", "error", err)
			}
			err = i.apiClient.ContainerRemove(context.Background(), watchedImage.containerID, container.RemoveOptions{})
			if err != nil {
				slog.Error("Failed to remove container", "error", err)
			}

			region := "eu-north-1"

			// Load AWS configuration
			cfg, err := config.LoadDefaultConfig(context.TODO(),
				config.WithRegion(region),
			)
			if err != nil {
				slog.Error("Unable to load SDK config, %v", err)
				return
			}

			// Create an ECR client
			client := ecr.NewFromConfig(cfg)

			// Call GetAuthorizationToken
			resp, err := client.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
			if err != nil {
				log.Fatalf("Failed to get authorization token, %v", err)
			}

			// Extract the token from the response
			if len(resp.AuthorizationData) == 0 {
				log.Fatalf("No authorization data in response")
			}

			token := *resp.AuthorizationData[0].AuthorizationToken //token from ecr.GetAuthorizationToken
			// Decode the base64 token
			decodedToken, err := base64.StdEncoding.DecodeString(token)
			if err != nil {
				log.Fatalf("Failed to decode authorization token, %v", err)
			}

			// Split the token into username and password
			parts := strings.Split(string(decodedToken), ":")
			if len(parts) != 2 {
				log.Fatalf("Invalid token format")
			}

			r, err := i.apiClient.RegistryLogin(context.Background(), registry.AuthConfig{
				Username:      parts[0],
				Password:      parts[1],
				ServerAddress: "628406611447.dkr.ecr.eu-north-1.amazonaws.com",
			})
			if err != nil {
				slog.Error("Failed to login to registry", "error", err)
				return
			}
			slog.Info("Logged in to registry", "status", r.Status)
			jsonBytes, _ := json.Marshal(map[string]string{
				"username": parts[0],
				"password": parts[1],
			})

			authStr := base64.URLEncoding.EncodeToString(jsonBytes)

			opts := &dockerImage.PullOptions{
				RegistryAuth: authStr,
				PrivilegeFunc: func(ctx context.Context) (string, error) {
					return "Basic " + string(decodedToken), nil
				},
			}
			res, err := i.apiClient.ImagePull(context.Background(), "628406611447.dkr.ecr.eu-north-1.amazonaws.com/unbrewd:0.0.2",
				// fmt.Sprintf("%s%s:%s", watchedImage.RepositoryUri, watchedImage.RepositoryName, imageTag),
				*opts)
			if err != nil {
				slog.Error("Failed to pull image", "error", err)
				return
			}
			res.Close()
			// start container
			dockerResp, err := i.apiClient.ContainerCreate(context.Background(), &container.Config{}, &container.HostConfig{}, nil, nil, "")
			if err != nil {
				slog.Error("Failed to create container", "error", err)
				return
			}
			err = i.apiClient.ContainerStart(context.Background(), dockerResp.ID, container.StartOptions{})
			if err != nil {
				slog.Error("Failed to start container", "error", err)
				return
			}
			watchedImage.containerID = dockerResp.ID
			slog.Info("UpdatedImage(container restarted)", "image", image, "image-tag", imageTag, "container-id", dockerResp.ID)
			return
		}
	}
	slog.Info("UpdatedImage(done)", "No image found for image", image)

	//
	// run docker pull
	// run docker compose down
	// run docker compose up
	// # Define variables
	//
	// container_name="your_container_name"
	// image_name="your_image_name"
	// new_tag="your_new_tag"
	//
	// # Stop the running container
	// docker stop $container_name
	//
	// # Remove the stopped container (optional)
	// docker rm $container_name
	//
	// # Pull the new image
	// docker pull $image_name:$new_tag
	//
	// # Start a new container with the updated image
	// docker run -d --name $container_name $image_name:$new_tag
	//cmd := exec.Command("/bin/sh",

}
