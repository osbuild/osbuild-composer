package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToQueueName(t *testing.T) {
	cases := []struct {
		Name      string
		JobType   string
		JobOwner  string
		QueueName string
	}{
		{
			Name:      "only a job type",
			JobType:   "osbuild:x86_64",
			JobOwner:  "",
			QueueName: "osbuild:x86_64",
		},
		{
			Name:      "job type and job owner",
			JobType:   "osbuild:x86_64",
			JobOwner:  "ostrich",
			QueueName: "osbuild:x86_64?owner=ostrich",
		},
		{
			Name:      "job type and weird job owner",
			JobType:   "osbuild:x86_64",
			JobOwner:  "bird?kingfisher",
			QueueName: "osbuild:x86_64?owner=bird%3Fkingfisher",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			require.Equal(t, c.QueueName, toQueueName(c.JobType, c.JobOwner))
		})
	}
}

func TestFromQueueName(t *testing.T) {
	cases := []struct {
		Name          string
		QueueName     string
		JobType       string
		JobOwner      string
		expectedError bool
	}{
		{
			Name:          "only a job type",
			QueueName:     "osbuild:x86_64",
			JobType:       "osbuild:x86_64",
			JobOwner:      "",
			expectedError: false,
		},
		{
			Name:          "job type and job owner",
			QueueName:     "osbuild:x86_64?owner=ostrich",
			JobType:       "osbuild:x86_64",
			JobOwner:      "ostrich",
			expectedError: false,
		},
		{
			Name:          "job type and weird job owner",
			QueueName:     "osbuild:x86_64?owner=bird%3Fkingfisher",
			JobType:       "osbuild:x86_64",
			JobOwner:      "bird?kingfisher",
			expectedError: false,
		},
		{
			Name:          "non existing parameter",
			QueueName:     "osbuild:x86_64?bird=emu",
			JobType:       "",
			JobOwner:      "",
			expectedError: true,
		},
		{
			Name:          "non unescapable parameter value",
			QueueName:     "osbuild:x86_64?owner=bird%3vulture",
			JobType:       "",
			JobOwner:      "",
			expectedError: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actualJobType, actualJobOwner, err := fromQueueName(c.QueueName)
			if c.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, c.JobType, actualJobType)
			require.Equal(t, c.JobOwner, actualJobOwner)
		})
	}
}

// Test that jobRestrictions == fromQueueName(toQueueName(jobRestrictions))
// holds true.
func TestToQueueNameAndBack(t *testing.T) {
	cases := []struct {
		Name     string
		JobType  string
		JobOwner string
	}{
		{
			Name:     "only a job type",
			JobType:  "osbuild:x86_64",
			JobOwner: "",
		},
		{
			Name:     "job type and job owner",
			JobType:  "osbuild:x86_64",
			JobOwner: "ostrich",
		},
		{
			Name:     "job type and weird job owner",
			JobType:  "osbuild:x86_64",
			JobOwner: "bird?kingfisher",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			queueName := toQueueName(c.JobType, c.JobOwner)
			actualJobType, actualJobOwner, err := fromQueueName(queueName)
			require.NoError(t, err)
			require.Equal(t, c.JobType, actualJobType)
			require.Equal(t, c.JobOwner, actualJobOwner)
		})
	}
}
