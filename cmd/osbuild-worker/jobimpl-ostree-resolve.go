package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type OSTreeResolveJobImpl struct {
	RepositoryMTLSConfig *RepositoryMTLSConfig
}

func (job *OSTreeResolveJobImpl) CompareBaseURL(baseURLStr string) (bool, error) {
	if job.RepositoryMTLSConfig == nil || job.RepositoryMTLSConfig.BaseURL == nil {
		return false, nil
	}

	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return false, err
	}

	if baseURL.Scheme != job.RepositoryMTLSConfig.BaseURL.Scheme {
		return false, nil
	}
	if baseURL.Host != job.RepositoryMTLSConfig.BaseURL.Host {
		return false, nil
	}
	if !strings.HasPrefix(baseURL.Path, job.RepositoryMTLSConfig.BaseURL.Path) {
		return false, nil
	}

	return true, nil
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
		reqParams := ostree.SourceSpec{}
		reqParams.URL = s.URL
		reqParams.Ref = s.Ref
		if match, err := impl.CompareBaseURL(s.URL); match && err == nil {
			if impl.RepositoryMTLSConfig.Proxy != nil {
				reqParams.Proxy = impl.RepositoryMTLSConfig.Proxy.String()
			}
			reqParams.MTLS = &ostree.MTLS{
				CA:         impl.RepositoryMTLSConfig.CA,
				ClientCert: impl.RepositoryMTLSConfig.MTLSClientCert,
				ClientKey:  impl.RepositoryMTLSConfig.MTLSClientKey,
			}
		} else if err != nil {
			logWithId.Errorf("Error comparing base URL: %v", err)
			result.JobError = clienterrors.New(
				clienterrors.ErrorInvalidRepositoryURL,
				"Repository URL is malformed",
				err.Error(),
			)
			break
		} else {
			mURL := ""
			if impl.RepositoryMTLSConfig != nil && impl.RepositoryMTLSConfig.BaseURL != nil {
				mURL = impl.RepositoryMTLSConfig.BaseURL.String()
			}
			logWithId.Warnf("Repository URL '%s' does not match '%s', MTLS: %t", s.URL, mURL, impl.RepositoryMTLSConfig != nil)
		}
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
