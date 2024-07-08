package image_watcher

import (
	"testing"

	"github.com/docker/docker/api/types"
)

type expected struct {
	image string
	tag   string
}

func TestImageWatcherInitialize(t *testing.T) {
	iw := &ImageWatcher{}
	iw.watchedImages = make(map[string]WatchedImage)
	config := &Config{}
	watchedImages := make([]WatchedImageConfig, 3)
	config.WatchedImages = watchedImages

	watchedImages[0] = WatchedImageConfig{
		RepositoryName: "/image1",
		RepositoryUri:  "123456789013.dkr.ecr.us-west-2.amazonaws.com",
		ImageTagPrefix: "ljos-dev",
	}
	watchedImages[1] = WatchedImageConfig{
		RepositoryName: "/image1",
		RepositoryUri:  "123456789013.dkr.ecr.us-west-2.amazonaws.com",
		ImageTagPrefix: "staging",
	}
	watchedImages[2] = WatchedImageConfig{
		RepositoryName: "/some-other-image",
		RepositoryUri:  "987654321023.dkr.ecr.eu-north-1.amazonaws.com",
		ImageTagPrefix: "master",
	}
	containers := make([]types.Container, 2)
	containers[0] = types.Container{
		ID:         "aa1234o",
		Names:      []string{"image1-staging"},
		Image:      "123456789013.dkr.ecr.us-west-2.amazonaws.com/image1:staging-1.1.0",
		ImageID:    "",
		Command:    "",
		Created:    0,
		Ports:      []types.Port{},
		SizeRw:     0,
		SizeRootFs: 0,
		Labels:     map[string]string{},
		State:      "",
		Status:     "Exited (1) 27 hours ago",
		HostConfig: struct {
			NetworkMode string            "json:\",omitempty\""
			Annotations map[string]string "json:\",omitempty\""
		}{},
		NetworkSettings: &types.SummaryNetworkSettings{},
		Mounts:          []types.MountPoint{},
	}
	containers[1] = types.Container{
		ID:         "ee11e4efc4204e52da89ebe149e98cbced44efd34cb0865040329d745f83b8c6",
		Names:      []string{"image1-staging"},
		Image:      "123456789013.dkr.ecr.us-west-2.amazonaws.com/image1:staging-1.1.0",
		ImageID:    "",
		Command:    "",
		Created:    0,
		Ports:      []types.Port{},
		SizeRw:     0,
		SizeRootFs: 0,
		Labels:     map[string]string{},
		State:      "",
		Status:     "UP 27 hours ago",
		HostConfig: struct {
			NetworkMode string            "json:\",omitempty\""
			Annotations map[string]string "json:\",omitempty\""
		}{},
		NetworkSettings: &types.SummaryNetworkSettings{},
		Mounts:          []types.MountPoint{},
	}
	testCases := []expected{
		{"/image1:ljos-dev", ""},
		{"/image1:staging", "staging-1.1.0"},
		{"/some-other-image:master", ""},
	}
	iw.initializeWatcherImages(config, containers)
	if len(iw.watchedImages) != 3 {
		t.Errorf("Expected 3 watched image, got %d", len(iw.watchedImages))
	}
	for _, tc := range testCases {
		if _, ok := iw.watchedImages[tc.image]; !ok {
			t.Errorf("Expected to find /image1:ljos-dev")
		}
	}
}
