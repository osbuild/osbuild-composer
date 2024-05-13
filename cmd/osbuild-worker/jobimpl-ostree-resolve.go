package main

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type OSTreeResolveJobImpl struct {
}

func setError(err error, result *worker.OSTreeResolveJobResult) {
	switch err.(type) {
	case ostree.RefError:
		result.JobError = clienterrors.New(
			clienterrors.ErrorOSTreeRefInvalid,
			"Invalid OSTree ref",
			err.Error(),
		)
	case ostree.ResolveRefError:
		result.JobError = clienterrors.New(
			clienterrors.ErrorOSTreeRefResolution,
			"Error resolving OSTree ref",
			err.Error(),
		)
	default:
		result.JobError = clienterrors.New(
			clienterrors.ErrorOSTreeParamsInvalid,
			"Invalid OSTree parameters or parameter combination",
			err.Error(),
		)
	}
}

func (impl *OSTreeResolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())
	var args worker.OSTreeResolveJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	result := worker.OSTreeResolveJobResult{
		Specs: make([]worker.OSTreeResolveResultSpec, len(args.Specs)),
	}

	logWithId.Infof("Resolving (%d) ostree commits", len(args.Specs))

	for i, s := range args.Specs {
		reqParams := ostree.SourceSpec(s)
		commitSpec, err := ostree.Resolve(reqParams)
		if err != nil {
			logWithId.Infof("Resolving ostree params failed: %v", err)
			setError(err, &result)
			break
		}

		result.Specs[i] = worker.OSTreeResolveResultSpec{
			URL:      commitSpec.URL,
			Ref:      commitSpec.Ref,
			Checksum: commitSpec.Checksum,
			Secrets:  commitSpec.Secrets,
			RHSM:     s.RHSM,
		}
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}
