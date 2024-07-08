package image_watcher

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"ljos.app/ecr-change-receiver/aws"
	"ljos.app/ecr-change-receiver/image_watcher/docker"
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
	// apiClient     *client.Client
	awsClient    *aws.AwsClient
	dockerClient *docker.DockerClient
	mutex        sync.Mutex
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
	i.awsClient.Close()
	i.dockerClient.Close()
}

func (i *ImageWatcher) RemoveImage(image string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	delete(i.watchedImages, image)
}

func NewImageWatcher(region string, awsClient *aws.AwsClient) *ImageWatcher {
	dockerClient := docker.NewDockerClient(awsClient)
	return &ImageWatcher{
		region:       region,
		awsClient:    awsClient,
		dockerClient: dockerClient,
	}
}

func (iw *ImageWatcher) Start() {
	slog.Info("Starting image watcher")
	iw.watchedImages = make(map[string]WatchedImage)
	config := newConfig()
	containers, ok := iw.dockerClient.ListContainer()
	if !ok {
		panic("could not list containers")
	}
	iw.initializeWatcherImages(config, containers)
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
func (i *ImageWatcher) pullAndStartImage(watchedImage Image, image string, imageTag string) {
	slog.Info("UpdatedImage(container not started)", "image", image, "image-tag", imageTag)
	// pull new image and start containerID
	//
	refString := fmt.Sprintf("%s%s:%s", watchedImage.RepositoryUri, watchedImage.RepositoryName, imageTag)
	ok := i.dockerClient.PullImage(refString)
	if !ok {
		slog.Error("Failed to pull image")
		return
	}
	// start container
	resp, ok := i.dockerClient.CreateContainer(refString)
	if !ok {
		slog.Error("Failed to create container")
		return
	}

	watchedImage.containerID = resp
	ok = i.dockerClient.StartContainer(resp)
	if !ok {
		return
	}
	slog.Info("UpdatedImage(container started)", "image", image, "image-tag", imageTag, "container-id", watchedImage.containerID)
	return
}
func (i *ImageWatcher) UpdateImage(image string, imageTag string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	slog.Info("UpdatedImage(start)", "image", image, "image-tag", imageTag)
	watchedImages, ok := i.watchedImages[image]
	if !ok {
		slog.Info("UpdateImage", "watchedImages", i.watchedImages)
		return
	}
	for prefix, watchedImage := range watchedImages.images {
		if strings.HasPrefix(imageTag, prefix) {
			watchedImage.PreviousImageTag = watchedImage.ImageTag
			watchedImage.ImageTag = imageTag
			watchedImage.StartTime = time.Now()
			if watchedImage.containerID == "" {
				i.pullAndStartImage(watchedImage, image, imageTag)
				return
			}

			slog.Info("UpdatedImage(container already started)", "image", image, "image-tag", imageTag, "container-id", watchedImage.containerID)
			watchedImages.images[prefix] = watchedImage
			i.watchedImages[image] = watchedImages
			slog.Info("UpdatedImage(done)", "image", image, "image-tag", imageTag)
			// pull new image and restart container
			ok := i.dockerClient.StartContainer(watchedImage.containerID)
			if !ok {
				return
			}
			ok = i.dockerClient.RemoveContainer(watchedImage.containerID)
			if !ok {
				return
			}
			i.pullAndStartImage(watchedImage, image, imageTag)
			slog.Info("UpdatedImage(done)", "image", image, "image-tag", imageTag)
			return
		}
	}
	slog.Info("UpdatedImage(done)", "No image found for image", image)
}
