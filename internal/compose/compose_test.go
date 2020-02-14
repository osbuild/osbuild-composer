package compose

import (
	"github.com/osbuild/osbuild-composer/internal/common"
	"testing"
)

func TestGetState(t *testing.T) {
	cases := []struct {
		compose       Compose
		expecedStatus common.ComposeState
	}{
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBWaiting},
				},
			},
			expecedStatus: common.CWaiting,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBRunning},
				},
			},
			expecedStatus: common.CRunning,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBFailed},
				},
			},
			expecedStatus: common.CFailed,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBFinished},
				},
			},
			expecedStatus: common.CFinished,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBWaiting},
					{QueueStatus: common.IBWaiting},
				},
			},
			expecedStatus: common.CWaiting,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBWaiting},
					{QueueStatus: common.IBRunning},
				},
			},
			expecedStatus: common.CRunning,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBRunning},
					{QueueStatus: common.IBRunning},
				},
			},
			expecedStatus: common.CRunning,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBRunning},
					{QueueStatus: common.IBFailed},
				},
			},
			expecedStatus: common.CRunning,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBWaiting},
					{QueueStatus: common.IBFailed},
				},
			},
			expecedStatus: common.CRunning,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBFailed},
					{QueueStatus: common.IBFailed},
				},
			},
			expecedStatus: common.CFailed,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBFinished},
					{QueueStatus: common.IBFinished},
				},
			},
			expecedStatus: common.CFinished,
		},
		{
			compose: Compose{
				ImageBuilds: []ImageBuild{
					{QueueStatus: common.IBFinished},
					{QueueStatus: common.IBFailed},
				},
			},
			expecedStatus: common.CFailed,
		},
	}
	for n, c := range cases {
		got := c.compose.GetState()
		wanted := c.expecedStatus
		if got != wanted {
			t.Error("Compose", n, "should be in", wanted.ToString(), "state, but it is:", got.ToString())
		}
	}
}
