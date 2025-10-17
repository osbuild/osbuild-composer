package main

import (
	"errors"
	"fmt"

	smithy "github.com/aws/smithy-go"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func getAWS(awsCreds, region string) (*awscloud.AWS, error) {
	if awsCreds != "" {
		return awscloud.NewFromFile(awsCreds, region)
	}
	return awscloud.NewDefault(region)
}

type AWSEC2CopyJobImpl struct {
	AWSCreds string
}

func (impl *AWSEC2CopyJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())
	result := worker.AWSEC2CopyJobResult{}

	defer func() {
		err := job.Finish(&result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.AWSEC2CopyJob
	err := job.Args(&args)
	if err != nil {
		result.JobError = clienterrors.New(clienterrors.ErrorParsingJobArgs, fmt.Sprintf("Error parsing arguments: %v", err), nil)
		return err
	}

	aws, err := getAWS(impl.AWSCreds, args.TargetRegion)
	if err != nil {
		logWithId.Errorf("Error creating aws client: %v", err)
		result.JobError = clienterrors.New(clienterrors.ErrorInvalidConfig, "Invalid worker config", nil)
		return err
	}

	ami, err := aws.CopyImage(args.TargetName, args.Ami, args.SourceRegion)
	if err != nil {
		logWithId.Errorf("Error copying ami: %v", err)
		result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Error copying ami %s", args.Ami), nil)

		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "InvalidRegion":
				result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Invalid source region '%s'", args.SourceRegion), nil)
			case "InvalidAMIID.Malformed":
				result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Malformed source ami id '%s'", args.Ami), nil)
			case "InvalidAMIID.NotFound":
				fallthrough // CopyImage returns InvalidRequest instead of InvalidAMIID.NotFound
			case "InvalidRequest":
				result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Source ami '%s' not found", args.Ami), nil)
			}
		} else {
			result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Unknown error copying ami '%s'", args.Ami), err.Error())
		}

		return err
	}

	result.Ami = ami
	result.Region = args.TargetRegion
	return nil
}

type AWSEC2ShareJobImpl struct {
	AWSCreds string
}

func (impl *AWSEC2ShareJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())
	result := worker.AWSEC2ShareJobResult{}

	defer func() {
		err := job.Finish(&result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.AWSEC2ShareJob
	err := job.Args(&args)
	if err != nil {
		result.JobError = clienterrors.New(clienterrors.ErrorParsingJobArgs, fmt.Sprintf("Error parsing arguments: %v", err), nil)
		return err
	}

	if args.Ami == "" || args.Region == "" {
		if job.NDynamicArgs() != 1 {
			logWithId.Error("No arguments given and dynamic args empty")
			result.JobError = clienterrors.New(clienterrors.ErrorNoDynamicArgs, "An ec2 share job should have args or depend on an ec2 copy job", nil)
			return nil
		}
		var cjResult worker.AWSEC2CopyJobResult
		err = job.DynamicArgs(0, &cjResult)
		if err != nil {
			result.JobError = clienterrors.New(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args as ec2 copy job", nil)
			return err
		}
		if cjResult.JobError != nil {
			result.JobError = clienterrors.New(clienterrors.ErrorJobDependency, "AWSEC2CopyJob dependency failed", nil)
			return nil
		}

		args.Ami = cjResult.Ami
		args.Region = cjResult.Region
	}

	aws, err := getAWS(impl.AWSCreds, args.Region)
	if err != nil {
		logWithId.Errorf("Error creating aws client: %v", err)
		result.JobError = clienterrors.New(clienterrors.ErrorInvalidConfig, "Invalid worker config", nil)
		return err
	}

	err = aws.ShareImage(args.Ami, nil, args.ShareWithAccounts)
	if err != nil {
		logWithId.Errorf("Error sharing image: %v", err)
		result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Error sharing image with target %v", args.ShareWithAccounts), nil)
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "InvalidAMIID.Malformed":
				result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Malformed ami id '%s'", args.Ami), nil)
			case "InvalidAMIID.NotFound":
				result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Ami '%s' not found in region '%s'", args.Ami, args.Region), nil)
			case "InvalidAMIAttributeItemValue":
				result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Invalid user id to share ami with: %v", args.ShareWithAccounts), nil)
			}
		} else {
			result.JobError = clienterrors.New(clienterrors.ErrorSharingTarget, fmt.Sprintf("Unknown error sharing ami '%s' with %v", args.Ami, args.ShareWithAccounts), err.Error())
		}

		return err
	}

	result.Ami = args.Ami
	result.Region = args.Region
	return nil
}
