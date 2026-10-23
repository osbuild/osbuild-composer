package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/sync/semaphore"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

type ChildToParentAssociation struct {
	Child  string
	Parent string
}

func AWSCleanup(maxConcurrentRequests int, dryRun bool, accessKeyID, accessKey string, cutoff time.Time) error {
	const region = "us-east-1"
	var a *awscloud.AWS
	var err error

	ctx := context.Background()

	if accessKeyID != "" && accessKey != "" {
		a, err = awscloud.New(region, accessKeyID, accessKey, "")
		if err != nil {
			return err
		}
	} else {
		log.Printf("One of AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY is missing, trying default credentials…")
		a, err = awscloud.NewDefault(region)
		if err != nil {
			return err
		}
	}

	regions, err := a.Regions()
	if err != nil {
		return err
	}

	for _, region := range regions {
		a, err := awscloud.New(region, accessKeyID, accessKey, "")
		if err != nil {
			log.Printf("Unable to create new aws session for region %s: %v", region, err)
			continue
		}

		var wg sync.WaitGroup
		sem := semaphore.NewWeighted(int64(maxConcurrentRequests))
		images, err := a.DescribeImagesByTag("Name", "composer-api-*")
		if err != nil {
			log.Printf("Unable to describe images for region %s: %v", region, err)
			continue
		}

		for index, image := range images {
			// TODO are these actual concerns?
			if image.ImageId == nil {
				log.Printf("ImageId is nil %v", image)
				continue
			}
			if image.CreationDate == nil {
				log.Printf("Image %v has nil creationdate", *image.ImageId)
				continue
			}

			created, err := time.Parse(time.RFC3339, *image.CreationDate)
			if err != nil {
				log.Printf("Unable to parse date %s for image %s", *image.CreationDate, *image.ImageId)
				continue
			}

			if !created.Before(cutoff) {
				continue
			}

			if dryRun {
				log.Printf("Dry run, aws image %s in region %s, with creation date %s would be removed", *image.ImageId, region, *image.CreationDate)
				continue
			}

			if err = sem.Acquire(ctx, 1); err != nil {
				log.Printf("Error acquiring semaphore: %v", err)
				continue
			}
			wg.Add(1)

			go func(i int) {
				defer sem.Release(1)
				defer wg.Done()

				err := a.DeleteEC2Image(aws.ToString(images[i].ImageId))
				if err != nil {
					log.Printf("Cleanup for image %s in region %s failed: %v", *images[i].ImageId, region, err)
				}
			}(index)
		}
		wg.Wait()
	}

	var errs []error

	err = terminateOrphanedSecureInstances(a, dryRun)
	if err != nil {
		log.Printf("Error in terminating secure instances: %v, continuing other cleanup.", err)
		errs = append(errs, err)
	}

	err = searchSGAndCleanup(ctx, a, dryRun)
	if err != nil {
		log.Printf("Error in cleaning up security groups: %v", err)
		errs = append(errs, err)
	}

	err = searchLTAndCleanup(ctx, a, dryRun)
	if err != nil {
		log.Printf("Error in cleaning up launch templates: %v", err)
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func terminateOrphanedSecureInstances(a *awscloud.AWS, dryRun bool) error {
	// Terminate leftover secure instances
	reservations, err := a.DescribeInstancesByTag("parent", "i-*")
	if err != nil {
		return fmt.Errorf("Unable to describe instances by tag %w", err)
	}

	instanceData := getChildParentAssociations(reservations)

	var instanceIDs []string
	for _, data := range instanceData {
		parent, err := a.DescribeInstancesByInstanceID(data.Parent)
		if err != nil {
			log.Printf("Error getting info of %s (parent of %s): %v", data.Parent, data.Child, err)
			continue
		}

		if !checkValidParent(data.Child, parent) {
			instanceIDs = append(instanceIDs, data.Child)
		}
	}

	instanceIDs = filterOnTooOld(instanceIDs, reservations)
	log.Printf("Cleaning up executor instances: %v", instanceIDs)
	if !dryRun {
		if len(instanceIDs) > 0 {
			_, err = a.TerminateInstancesEC2(instanceIDs, 0)
			if err != nil {
				return fmt.Errorf("Unable to terminate secure instances: %w", err)
			}
		}
	} else {
		log.Print("Dry run, didn't actually terminate any instances")
	}
	return nil
}

func filterOnTooOld(instanceIDs []string, reservations []ec2types.Reservation) []string {
	for _, res := range reservations {
		for _, i := range res.Instances {
			if i.LaunchTime.Before(time.Now().Add(-time.Hour * 2)) {
				log.Printf("Instance %s is too old", *i.InstanceId)
				if !slices.Contains(instanceIDs, *i.InstanceId) {
					instanceIDs = append(instanceIDs, *i.InstanceId)
				}
			}
		}
	}
	return instanceIDs
}

func getChildParentAssociations(reservations []ec2types.Reservation) []ChildToParentAssociation {
	var ChildToParentIDs []ChildToParentAssociation

	for _, res := range reservations {
		for _, i := range res.Instances {
			for _, t := range i.Tags {
				if *t.Key == "parent" {
					ChildToParentIDs = append(ChildToParentIDs, ChildToParentAssociation{
						Child:  *i.InstanceId,
						Parent: *t.Value,
					})
				}
			}
		}
	}
	return ChildToParentIDs
}

func checkValidParent(childId string, parent []ec2types.Reservation) bool {
	if len(parent) == 0 {
		log.Printf("Instance %s has no parent, removing it", childId)
		return false
	}
	if len(parent) != 1 {
		log.Printf("Instance %s has %d parents. That should never happen, not changing anything here.", childId, len(parent))
		return true
	}
	if len(parent[0].Instances) == 0 {
		log.Printf("Instance %s has no parent instance, removing it", childId)
		return false
	}
	if len(parent[0].Instances) != 1 {
		log.Printf("Instance %s has %d parent instances. That should never happen, not changing anything here.", childId, len(parent[0].Instances))
		return true
	}

	parentState := parent[0].Instances[0].State.Name
	if parentState != ec2types.InstanceStateNameTerminated {
		return true
	}
	log.Printf("Instance %s has a parent (%s) in state %s, so we'll terminate %s.", childId, *parent[0].Instances[0].InstanceId, parentState, childId)
	return false
}

func searchSGAndCleanup(ctx context.Context, a *awscloud.AWS, dryRun bool) error {
	securityGroups, err := a.DescribeSecurityGroupsByPrefix(ctx, "SG for i-")
	if err != nil {
		return err
	}

	for _, sg := range securityGroups {
		if sg.GroupId == nil || sg.GroupName == nil {
			log.Printf(
				"Security Group needs to have a GroupId (%v) and a GroupName (%v).",
				sg.GroupId,
				sg.GroupName)
			continue
		}
		reservations, err := a.DescribeInstancesBySecurityGroupID(*sg.GroupId)
		if err != nil {
			log.Printf("Failed to describe security group %s: %v", *sg.GroupId, err)
			continue
		}

		// If no instance is running/pending, delete the SG
		if allTerminated(reservations) {
			log.Printf("Deleting security group: %s (%s)", *sg.GroupName, *sg.GroupId)
			if !dryRun {
				err := a.DeleteSecurityGroupById(ctx, sg.GroupId)

				if err != nil {
					log.Printf("Failed to delete security group %s: %v", *sg.GroupId, err)
				}
			}
		} else {
			log.Printf("Security group %s has non terminated instances associated with it.", *sg.GroupId)
		}
	}
	return nil
}

// allTerminated returns true if any instance of the reservations is not terminated
// then it's considered "in use"
func allTerminated(reservations []ec2types.Reservation) bool {
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			if instance.State != nil && (instance.State.Name != ec2types.InstanceStateNameTerminated) {
				return false
			}
		}
	}
	return true
}

func searchLTAndCleanup(ctx context.Context, a *awscloud.AWS, dryRun bool) error {
	launchTemplates, err := a.DescribeLaunchTemplatesByPrefix(ctx, "launch-template-for-i-")
	if err != nil {
		return err
	}

	for _, lt := range launchTemplates {
		if lt.LaunchTemplateName == nil || lt.LaunchTemplateId == nil {
			log.Printf(
				"Launch template needs to have a LaunchTemplateName (%v) and a LaunchTemplateId (%v).",
				lt.LaunchTemplateName,
				lt.LaunchTemplateId)
			continue
		}

		reservations, err := a.DescribeInstancesByLaunchTemplateID(*lt.LaunchTemplateId)
		if err != nil {
			log.Printf("Failed to describe launch template %s: %v", *lt.LaunchTemplateId, err)
			continue
		}

		if allTerminated(reservations) {
			log.Printf("Deleting launch template: %s (%s)\n", *lt.LaunchTemplateName, *lt.LaunchTemplateId)
			if !dryRun {
				err := a.DeleteLaunchTemplateById(ctx, lt.LaunchTemplateId)

				if err != nil {
					log.Printf("Failed to delete launch template %s: %v", *lt.LaunchTemplateId, err)
				}
			}
		} else {
			log.Printf("Launch template %s has non terminated instances associated with it.\n", *lt.LaunchTemplateId)
		}
	}
	return nil
}
