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
					InstanceId: common.ToPtr("not filtered 1"),
				},
			},
		},
		{
			Instances: []ec2types.Instance{
				{
					LaunchTime: common.ToPtr(time.Now().Add(-time.Minute * 121)),
					InstanceId: common.ToPtr("not filtered 2"),
				},
			},
		},
		{
			Instances: []ec2types.Instance{
				{
					LaunchTime: common.ToPtr(time.Now().Add(-time.Minute * 119)),
					InstanceId: common.ToPtr("filtered 1"),
				},
			},
		},
		{
			Instances: []ec2types.Instance{
				{
					LaunchTime: common.ToPtr(time.Now()),
					InstanceId: common.ToPtr("filtered 2"),
				},
			},
		},
	}

	instanceIDs := main.FilterOnTooOld([]string{}, reservations)
	require.Equal(t, []string{"not filtered 1", "not filtered 2"}, instanceIDs)
}

func TestCheckValidParent(t *testing.T) {
	testInstanceID := "TestInstance"
	tests :=
		[]struct {
			parent []ec2types.Reservation
			result bool
		}{
			// no parent
			{
				parent: []ec2types.Reservation{},
				result: false,
			},
			// many parents - "valid" to leave as is
			{
				parent: []ec2types.Reservation{
					{}, {},
				},
				result: true,
			},
			// no parent instance
			{
				parent: []ec2types.Reservation{
					{Instances: []ec2types.Instance{}},
				},
				result: false,
			},
			// many parent instances - "valid" to leave as is
			{
				parent: []ec2types.Reservation{
					{Instances: []ec2types.Instance{{}, {}}},
				},
				result: true,
			},
			// pending parent
			{
				parent: []ec2types.Reservation{
					{Instances: []ec2types.Instance{{
						InstanceId: &testInstanceID,
						State: &ec2types.InstanceState{
							Name: ec2types.InstanceStateNamePending,
						},
					}}},
				},
				result: true,
			},
			// running parent
			{
				parent: []ec2types.Reservation{
					{Instances: []ec2types.Instance{{
						InstanceId: &testInstanceID,
						State: &ec2types.InstanceState{
							Name: ec2types.InstanceStateNameRunning,
						},
					}}},
				},
				result: true,
			},
			// terminated parent - not valid instance
			{
				parent: []ec2types.Reservation{
					{Instances: []ec2types.Instance{{
						InstanceId: &testInstanceID,
						State: &ec2types.InstanceState{
							Name: ec2types.InstanceStateNameTerminated,
						},
					}}},
				},
				result: false,
			},
		}
	for _, tc := range tests {
		require.Equal(t, tc.result, main.CheckValidParent("testChildId", tc.parent))
	}
}
