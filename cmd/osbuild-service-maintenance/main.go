package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetReportCaller(true)

	// 14 days
	cutoff := time.Now().Add(-(time.Hour * 24 * 14))
	logrus.Infof("Cutoff date: %v", cutoff)

	conf := Config{
		MaxConcurrentRequests: 20,
		EnableDBMaintenance:   false,
		EnableGCPMaintenance:  false,
		EnableAWSMaintenance:  false,
	}
	err := LoadConfigFromEnv(&conf)
	if err != nil {
		logrus.Fatal(err)
	}

	if conf.DryRun {
		logrus.Info("Dry run, no state will be changed")
	}

	if conf.MaxConcurrentRequests == 0 {
		logrus.Fatal("Max concurrent requests is 0")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !conf.EnableAWSMaintenance {
			logrus.Info("AWS maintenance not enabled, skipping")
			return
		}

		logrus.Info("Cleaning up AWS")
		err := AWSCleanup(conf.MaxConcurrentRequests, conf.DryRun, conf.AWSAccessKeyID, conf.AWSSecretAccessKey, cutoff)
		if err != nil {
			logrus.Errorf("AWS cleanup failed: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if !conf.EnableGCPMaintenance {
			logrus.Info("GCP maintenance not enabled, skipping")
			return
		}

		logrus.Info("Cleaning up GCP")
		var gcpConf GCPCredentialsConfig
		err := LoadConfigFromEnv(&gcpConf)
		if err != nil {
			logrus.Error("Unable to load GCP config from environment")
			return
		}

		if !gcpConf.valid() {
			logrus.Error("GCP credentials invalid, fields missing")
			return
		}

		creds, err := json.Marshal(&gcpConf)
		if err != nil {
			logrus.Errorf("Unable to marshal gcp conf: %v", err)
			return
		}

		err = GCPCleanup(creds, conf.MaxConcurrentRequests, conf.DryRun, cutoff)
		if err != nil {
			logrus.Errorf("GCP Cleanup failed: %v", err)
		}
	}()

	wg.Wait()
	logrus.Info("🦀🦀🦀 cloud cleanup done 🦀🦀🦀")

	if !conf.EnableDBMaintenance {
		logrus.Info("🦀🦀🦀 DB maintenance not enabled, skipping  🦀🦀🦀")
		return
	}
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		conf.PGUser,
		conf.PGPassword,
		conf.PGHost,
		conf.PGPort,
		conf.PGDatabase,
		conf.PGSSLMode,
	)
	err = DBCleanup(dbURL, conf.DryRun, cutoff)
	if err != nil {
		logrus.Fatalf("Error during DBCleanup: %v", err)
	}
	logrus.Info("🦀🦀🦀 dbqueue cleanup done 🦀🦀🦀")
}
