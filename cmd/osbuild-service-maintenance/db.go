package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/dbjobqueue"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func DBCleanup(dbURL string, dryRun bool, cutoff time.Time) error {
	jobs, err := dbjobqueue.New(dbURL)
	if err != nil {
		return err
	}

	// The results of these jobs take up the most space and can contain sensitive data. Delete
	// them after a while.
	jobsByType, err := jobs.JobsUptoByType([]string{worker.JobTypeManifestIDOnly, worker.JobTypeDepsolve}, cutoff)
	if err != nil {
		return fmt.Errorf("Error querying jobs: %v", err)
	}

	err = jobs.LogVacuumStats()
	if err != nil {
		logrus.Errorf("Error running vacuum stats: %v", err)
	}

	for k, v := range jobsByType {
		logrus.Infof("Deleting results from %d %s jobs", len(v), k)
		if dryRun {
			logrus.Info("Dry run, skipping deletion of jobs")
			continue
		}

		// Delete results in chunks to avoid starving the rds instance
		for i := 0; i < len(v); i += 100 {
			max := i + 100
			if max > len(v) {
				max = len(v)
			}

			rows, err := jobs.DeleteJobResult(v[i:max])
			if err != nil {
				logrus.Errorf("Error deleting results for jobs: %v, %d rows affected", rows, err)
				continue
			}
			logrus.Infof("Deleted results from %d jobs out of %d job ids", rows, len(v))
			err = jobs.VacuumAnalyze()
			if err != nil {
				logrus.Errorf("Error running vacuum analyze: %v", err)
			}
		}
	}

	err = jobs.LogVacuumStats()
	if err != nil {
		logrus.Errorf("Error running vacuum stats: %v", err)
	}

	return nil
}
