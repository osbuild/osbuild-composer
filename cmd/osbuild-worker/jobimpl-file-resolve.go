package main

import (
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/remotefile"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type FileResolveJobImpl struct{}

func (impl *FileResolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())

	var err error
	result := worker.FileResolveJobResult{
		Success: false,
		Results: []worker.FileResolveJobResultItem{},
	}

	defer func() {
		logWithId := logrus.WithField("jobId", job.Id().String())
		if result.JobError != nil {
			logWithId.Errorf("file content resolve job failed: %s", result.JobError.Reason)
			if result.JobError.Details != nil {
				logWithId.Errorf("failure details: %v", result.JobError.Details)
			}
		}

		if len(result.Results) == 0 {
			logWithId.Infof("Resolving file contents failed: %v", err)
			result.JobError = clienterrors.New(
				clienterrors.ErrorRemoteFileResolution,
				"Error resolving file contents",
				"All remote file contents returned empty",
			)
		}

		err := job.Update(result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.FileResolveJob
	err = job.Args(&args)
	if err != nil {
		return err
	}

	logWithId.Infof("Resolving file contents (%d)", len(args.URLs))

	resolver := remotefile.NewResolver()
	for _, url := range args.URLs {
		resolver.Add(url)
	}

	resultItems := resolver.Finish()

	for idx := range resultItems {
		result.Results = append(result.Results, worker.FileResolveJobResultItem{
			URL:             resultItems[idx].URL,
			Content:         resultItems[idx].Content,
			ResolutionError: resultItems[idx].ResolutionError,
		})
	}

	resolutionErrors := result.ResolutionErrors()
	if len(resolutionErrors) == 0 {
		result.Success = true
	} else {
		result.JobError = clienterrors.New(
			clienterrors.ErrorRemoteFileResolution,
			"at least one file resolution failed",
			resolutionErrors,
		)
	}

	return nil
}
