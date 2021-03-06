package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/store"
)

type ComposerClient struct {
	client *http.Client
}

func NewClient() *ComposerClient {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", "/run/osbuild-composer/job.socket")
			},
		},
	}
	return &ComposerClient{client}
}

func (c *ComposerClient) AddJob() (*jobqueue.Job, error) {
	type request struct {
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(request{})
	response, err := c.client.Post("http://localhost/job-queue/v1/jobs", "application/json", &b)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return nil, errors.New("couldn't create job")
	}

	job := &jobqueue.Job{}
	err = json.NewDecoder(response.Body).Decode(job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (c *ComposerClient) UpdateJob(job *jobqueue.Job, status string, image *store.Image) error {
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(&jobqueue.JobStatus{status, image})
	req, err := http.NewRequest("PATCH", "http://localhost/job-queue/v1/jobs/"+job.ID.String(), &b)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errors.New("error setting job status")
	}

	return nil
}

func handleJob(client *ComposerClient, distro distro.Distro) {
	fmt.Println("Waiting for a new job...")
	job, err := client.AddJob()
	if err != nil {
		panic(err)
	}

	client.UpdateJob(job, "RUNNING", nil)

	fmt.Printf("Running job %s\n", job.ID.String())
	image, err, errs := job.Run(distro)
	if err != nil {
		client.UpdateJob(job, "FAILED", nil)
		return
	}

	for _, err := range errs {
		if err != nil {
			client.UpdateJob(job, "FAILED", nil)
			return
		}
	}

	client.UpdateJob(job, "FINISHED", image)
}

func main() {
	distro, err := distro.FromHost()
	if err != nil {
		panic(err)
	}

	client := NewClient()
	for {
		handleJob(client, distro)
	}
}
