package main_test

import (
	"testing"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-service-maintenance"
	"github.com/osbuild/osbuild-composer/internal/common"
)

func TestFilterReservations(t *testing.T) {
	reservations := []ec2types.Reservation{
		{
			Instances: []ec2types.Instance{
				{
					LaunchTime: common.ToPtr(time.Now().Add(-time.Hour * 24)),
					InstanceId: common.ToPtr("not filtered"),
				},
			},
		},
		{
			Instances: []ec2types.Instance{
				{
					LaunchTime: common.ToPtr(time.Now().Add(-time.Minute * 121)),
					InstanceId: common.ToPtr("not filtered"),
				},
			},
		},
		{
			Instances: []ec2types.Instance{
				{
					LaunchTime: common.ToPtr(time.Now().Add(-time.Minute * 119)),
					InstanceId: common.ToPtr("filtered"),
				},
			},
		},
		{
			Instances: []ec2types.Instance{
				{
					LaunchTime: common.ToPtr(time.Now()),
					InstanceId: common.ToPtr("filtered"),
				},
			},
		},
	}

	instanceIDs := main.FilterReservations(reservations)
	require.Equal(t, []string{"not filtered", "not filtered"}, instanceIDs)
}
