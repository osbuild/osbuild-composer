package osbuildexecutor

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func handleProgress(osbuildStatus *osbuild.StatusScanner, logger logrus.FieldLogger, job worker.Job) error {
	if osbuildStatus == nil {
		return fmt.Errorf("status scanner is required to handle osbuild progress")
	}

	var lastUpdated time.Time
	for {
		st, err := osbuildStatus.Status()
		if err != nil {
			return fmt.Errorf(`error parsing osbuild status, please report a bug: %w`, err)
		}
		if st == nil {
			break
		}

		progress := logrus.Fields{}
		if st.Progress != nil {
			progress["progress-done"] = st.Progress.Done
			progress["progress-total"] = st.Progress.Total
			if st.Progress.SubProgress != nil {
				progress["subprogress-done"] = st.Progress.SubProgress.Done
				progress["subprogress-total"] = st.Progress.SubProgress.Total
			}
		}
		if st.Message != "" {
			logger.WithFields(progress).Infof("OSBuild status: %s", st.Message)
			if job == nil || time.Since(lastUpdated) < MinTimeBetweenUpdates {
				continue
			}
			lastUpdated = time.Now()
			partial := worker.JobResult{
				Progress: &worker.JobProgress{
					Message: st.Message,
				},
			}
			if st.Progress != nil {
				partial.Progress.Done = st.Progress.Done
				partial.Progress.Total = st.Progress.Total
				// more than 1 level of subprogress is not expected, just
				// pipelines and stages.
				if st.Progress.SubProgress != nil {
					partial.Progress.SubProgress = &worker.JobProgress{
						Message: st.Progress.SubProgress.Message,
						Done:    st.Progress.SubProgress.Done,
						Total:   st.Progress.SubProgress.Total,
					}
				}
			}
			err := job.Update(partial)
			if err != nil {
				logger.Errorf("Unable to update job: %s", err.Error())
			}
		}
		if st.Trace != "" {
			logger.Debugf("%s", st.Trace)
		}
	}
	return nil
}
