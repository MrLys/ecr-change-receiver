package web

import (
	"encoding/json"
	"log/slog"
	"net/http"

	image_watcher "ljos.app/ecr-change-receiver/image_watcher"
	"ljos.app/ecr-change-receiver/ratelimit"
	secrets "ljos.app/ecr-change-receiver/secrets"
)

type MyEvent struct {
	Time   string `json:"time"`
	Detail struct {
		Result         string `json:"result"`
		RepositoryName string `json:"repository-name"`
		ImageTag       string `json:"image-tag"`
	} `json:"detail"`
}

type Web struct {
	secretmanager *secrets.SecretService
	imageWatcher  *image_watcher.ImageWatcher
}

func (w *Web) authorizeRequest(r *http.Request) bool {
	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		slog.Info("No authorization header")
		return false
	}
	bearer = bearer[7:] // Remove "Bearer " prefix
	return true         // w.secretmanager.Validate(bearer)
}
func (w *Web) Close() {
	w.imageWatcher.Close()
	w.secretmanager.Close()
}

func NewWeb(awsAccessKeyId, awsSecretAccessKey, region, secretName string) *Web {
	web := &Web{}
	web.imageWatcher = image_watcher.NewImageWatcher(region)
	ss, err := secrets.NewSecretManager(awsAccessKeyId, awsSecretAccessKey, region, secretName)
	slog.Info("Secret manager created")
	if err != nil {
		panic(err)
		//("Failed to create secret manager: %v", err)
	}
	web.secretmanager = ss
	return web
}
func (w *Web) handleWebhook(event MyEvent) {
	slog.Info("Received event")
	// Handle the webhook event
	eventString, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal event: %v", err)
		return
	}
	event.Detail.RepositoryName = "/" + event.Detail.RepositoryName
	slog.Info("Received event ", string(eventString), "\n")
	w.imageWatcher.UpdateImage(event.Detail.RepositoryName, event.Detail.ImageTag)
	//watchedImage, ok := watchedImages.Load(event.Detail.RepositoryName)
	//if !ok {
	//	return
	//}
	//image, ok := watchedImage.(WatchedImage)
	//if !ok {
	//	return
	//}
	//image.PreviousImageTag = image.ImageTag
	//image.ImageTag = event.Detail.ImageTag
	//image.StartTime = time.Now()
	//watchedImages.Store(event.Detail.RepositoryName, image)

}

func (w *Web) Start() {
	w.imageWatcher.Start()
	w.secretmanager.Start()
	ratelimiter := ratelimit.RateLimiter{Limit: 2}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/update", func(rw http.ResponseWriter, r *http.Request) {
		slog.Info("Received request")
		// Parse the request body
		if ratelimiter.RateLimitsExceeded(r.RemoteAddr) {
			http.Error(rw, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		if !w.authorizeRequest(r) {
			http.Error(rw, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var event MyEvent
		err := json.NewDecoder(r.Body).Decode(&event)

		if err != nil {
			http.Error(rw, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		w.handleWebhook(event)

		rw.WriteHeader(http.StatusOK)
	})
	slog.Info("(web) Starting web server")
	http.ListenAndServe(":8080", nil)
}
