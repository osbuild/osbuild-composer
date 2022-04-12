package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/dbjobqueue"
)

func DBCleanup(dbURL string, dryRun bool, cutoff time.Time) error {
	archs := []string{"x86_64"}
	jobType := "osbuild"

	jobs, err := dbjobqueue.New(dbURL)
	if err != nil {
		return err
	}

	var jobTypes []string
	for _, a := range archs {
		jobTypes = append(jobTypes, fmt.Sprintf("%s:%s", jobType, a))
	}

	jobsByType, err := jobs.JobsUptoByType(jobTypes, cutoff)
	if err != nil {
		return fmt.Errorf("Error querying jobs: %v", err)
	}

	for k, v := range jobsByType {
		logrus.Infof("Deleting jobs and their dependencies of type %v", k)
		if dryRun {
			logrus.Infof("Dry run, skipping deletion of jobs: %v", v)
			continue
		}

		for _, jobId := range v {
			err = jobs.DeleteJobIncludingDependencies(jobId)
			if err != nil {
				return fmt.Errorf("Error deleting job: %v", jobId)
			}
		}
	}
	return nil
}
