package docker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"

	"github.com/fsouza/go-dockerclient"
)

type mockClientPuller struct {
	dockerWrap

	numCalls uint64
	err      error
}

func (c *mockClientPuller) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	time.Sleep(time.Second * 1)
	atomic.AddUint64(&c.numCalls, uint64(1))
	return c.err
}

// Lets do concurrent docker-pulls for an image with two different tags. This should results in only
// two calls to docker-pull.
func TestImagePullConcurrent1(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	var cli dockerClient
	mock := mockClientPuller{}
	cli = &mock

	puller := NewImagePuller(drivers.Config{}, cli)

	cfg := docker.AuthConfiguration{}
	img := "foo"
	repo := "zoo"
	tag1 := "1.0.0"
	tag2 := "1.0.1"

	var wg sync.WaitGroup
	wg.Add(20)

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			err := <-puller.PullImage(ctx, &cfg, img, repo, tag1)
			if err != nil {
				t.Fatalf("err received %v", err)
			}
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			err := <-puller.PullImage(ctx, &cfg, img, repo, tag2)
			if err != nil {
				t.Fatalf("err received %v", err)
			}
		}()
	}

	wg.Wait()

	// Should be two docker-pulls
	if mock.numCalls != 2 || ctx.Err() != nil {
		t.Fatalf("fail numOfPulls=%d ctx=%s", mock.numCalls, ctx.Err())
	}
}

// Lets do concurrent docker-pulls for an image but with error.
func TestImagePullConcurrent2(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	var cli dockerClient
	mock := mockClientPuller{err: errors.New("yogurt")}
	cli = &mock

	puller := NewImagePuller(drivers.Config{}, cli)

	cfg := docker.AuthConfiguration{}
	img := "foo"
	repo := "zoo"
	tag := "1.0.0"

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			err := <-puller.PullImage(ctx, &cfg, img, repo, tag)
			if err == nil || strings.Index(err.Error(), "yogurt") == -1 {
				t.Fatalf("Unknown err received %v", err)
			}
		}()
	}

	wg.Wait()

	// Should be one docker-pull
	if mock.numCalls != 1 || ctx.Err() != nil {
		t.Fatalf("fail numOfPulls=%d ctx=%s", mock.numCalls, ctx.Err())
	}
}
