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

	// The results of these jobs take up the most space and can contain sensitive data. Delete
	// them after a while.
	jobsByType, err := jobs.JobsUptoByType([]string{"manifest-id-only", "depsolve"}, cutoff)
	if err != nil {
		return fmt.Errorf("Error querying jobs: %v", err)
	}

	for k, v := range jobsByType {
		logrus.Infof("Deleting results from %d %s jobs", len(v), k)
		if dryRun {
			logrus.Info("Dry run, skipping deletion of jobs")
			continue
		}
		rows, err := jobs.DeleteJobResult(v)
		if err != nil {
			logrus.Errorf("Error deleting results for jobs: %v, %d rows affected", rows, err)
			continue
		}
		logrus.Infof("Deleted results from %d jobs out of %d job ids", rows, len(v))
	}

	return nil
}
