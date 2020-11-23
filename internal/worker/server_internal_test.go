package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToQueueName(t *testing.T) {
	cases := []struct {
		Name      string
		JobType   string
		QueueName string
	}{
		{
			Name:      "only a job type",
			JobType:   "osbuild:x86_64",
			QueueName: "osbuild:x86_64",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			require.Equal(t, c.QueueName, toQueueName(c.JobType))
		})
	}
}

func TestFromQueueName(t *testing.T) {
	cases := []struct {
		Name      string
		QueueName string
		JobType   string
	}{
		{
			Name:      "only a job type",
			QueueName: "osbuild:x86_64",
			JobType:   "osbuild:x86_64",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actualJobType, err := fromQueueName(c.QueueName)
			require.NoError(t, err)
			require.Equal(t, c.JobType, actualJobType)
		})
	}
}

// Test that jobRestrictions == fromQueueName(toQueueName(jobRestrictions))
// holds true.
func TestToQueueNameAndBack(t *testing.T) {
	cases := []struct {
		Name    string
		JobType string
	}{
		{
			Name:    "only a job type",
			JobType: "osbuild:x86_64",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			queueName := toQueueName(c.JobType)
			actualJobType, err := fromQueueName(queueName)
			require.NoError(t, err)
			require.Equal(t, c.JobType, actualJobType)
		})
	}
}
