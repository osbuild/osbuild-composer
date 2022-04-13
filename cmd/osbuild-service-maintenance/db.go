package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/dbjobqueue"
)

func DBCleanup(dbURL string, dryRun bool, cutoff time.Time) error {
	jobs, err := dbjobqueue.New(dbURL)
	if err != nil {
		return err
	}

	// Query 'root' jobs
	jobsByType, err := jobs.JobsUptoByType([]string{"depsolve", "koji-init"}, cutoff)
	if err != nil {
		return fmt.Errorf("Error querying jobs: %v", err)
	}

	for k, v := range jobsByType {
		logrus.Infof("Deleting %d %s jobs and their dependants", len(v), k)
		if dryRun {
			logrus.Info("Dry run, skipping deletion of jobs")
			continue
		}

		for _, jobId := range v {
			err = jobs.DeleteJobIncludingDependants(jobId)
			if err != nil {
				return fmt.Errorf("Error deleting job: %v", jobId)
			}
		}
	}
	return nil
}
