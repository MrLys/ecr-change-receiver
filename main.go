package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type MyEvent struct {
	Time   string `json:"time"`
	Detail struct {
		Result         string `json:"result"`
		RepositoryName string `json:"repository-name"`
		ImageTag       string `json:"image-tag"`
	} `json:"detail"`
}
type Secrets struct {
	PrevKey    string
	CurrentKey string
	mutex      sync.Mutex
}
type WatchedImage struct {
	RepositoryName   string
	RepositoryUri    string
	ImageTag         string
	StartTime        time.Time
	PreviousImageTag string
	mutex            sync.Mutex
}
type RateLimit struct {
	remateAddr string
	limit      int
	start      time.Time
}

var currentSecrets = Secrets{}

var watchedImages = map[string]WatchedImage{}
var rateLimits sync.Map

func (s *Secrets) authorizeRequest(r *http.Request) bool {
	bearer := r.Header.Get("Bearer")
	if bearer == "" {
		return false
	}
	bearer = bearer[7:] // Remove "Bearer " prefix
	if bearer != s.PrevKey && bearer != s.CurrentKey {
		return false
	}
	return true
}

func (s *Secrets) getCurrentKeyFromSecretManager() {
	// Get the current key from AWS Secret Manager
	s.CurrentKey = "current_key"
}

func generateKey() string {
	key := make([]byte, 32)
	_, err := rand.Reader.Read(key)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func (s *Secrets) uploadCurrentKeyToSecretManager() {
	// Upload key to AWS Secret Manager
}
func (s *Secrets) rotateKey() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.PrevKey = s.CurrentKey
	s.CurrentKey = generateKey()
	s.uploadCurrentKeyToSecretManager()
}

func listContainers(images []string) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Containers(%d):\n", len(containers))
	fmt.Println("-----------")
	for _, ctr := range containers {
		if ctr.Image == "" {
			continue
		}
		for _, image := range images {
			if ctr.Image == image {
				// we assume these are not initialized here, so no need to lock
				watchedImages[image] = WatchedImage{
					RepositoryName:   image,
					RepositoryUri:    image,
					ImageTag:         ctr.Image,
					StartTime:        time.Now(),
					PreviousImageTag: "",
					mutex:            sync.Mutex{},
				}
			}
		}
		fmt.Printf("%s %s %s (status: %s)\n", ctr.ID, ctr.Image, ctr.Names, ctr.Status)
	}
}
func manageSecrets() {
	currentSecrets.getCurrentKeyFromSecretManager()
	currentSecrets.rotateKey()
	// Rotate the key every 6 hours
	ticker := time.NewTicker(6 * time.Hour)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				currentSecrets.rotateKey()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func rateLimitsExceeded(remateAddr string) bool {
	limit, ok := rateLimits.Load(remateAddr)
	if limit == nil || !ok {
		rateLimit := RateLimit{remateAddr, 1, time.Now()}
		rateLimits.Store(remateAddr, rateLimit)
		return false
	}
	rateLimit := limit.(RateLimit)
	// reset if more than 1 minute has passed
	if time.Since(rateLimit.start) > 1*time.Minute {
		rateLimit.start = time.Now()
		rateLimit.limit = 1
		rateLimits.Store(remateAddr, rateLimit)
	} else if rateLimit.limit > 30 {
		// rate limit exceeded
		return true
	} else {
		// increment the limit
		rateLimit.limit++
		// update time to nearest minute
		rateLimit.start = time.Now().Truncate(time.Minute)
		rateLimits.Store(remateAddr, rateLimit)
	}

	return false
}

func main() {
	config := Config{}
	config.init()
	images := make([]string, len(config.WatchedImages))
	for i, image := range config.WatchedImages {
		images[i] = image.RepositoryName
	}

	listContainers(images)
	manageSecrets()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		// Parse the request body
		if rateLimitsExceeded(r.RemoteAddr) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		defer r.Body.Close()

		var event MyEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}
		if !currentSecrets.authorizeRequest(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handleWebhook(event)

		w.WriteHeader(http.StatusOK)
	})
	http.ListenAndServe(":8080", nil)
}

func handleWebhook(event MyEvent) {
	// Handle the webhook event
}
