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

var currentSecrets = Secrets{}

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

func listContianers() {
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

func main() {
	config := Config{}
	config.init()

	listContianers()
	manageSecrets()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		// Parse the request body
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
