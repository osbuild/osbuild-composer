package main

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"os"

	"github.com/osbuild/images/pkg/upload/oci"
	"github.com/spf13/cobra"
)

var (
	tenancy         string
	region          string
	userID          string
	privateKeyFile  string
	fingerprint     string
	bucketName      string
	bucketNamespace string
	fileName        string
	objectName      string
	compartment     string
)

var uploadCmd = &cobra.Command{
	Example:      "This tool uses the $HOME/.oci/config file to create the OCI client and can be\noverridden using CLI flags.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		uploader, err := uploaderFromConfig()
		if err != nil {
			return err
		}

		file, err := os.Open(fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		err = uploader.Upload(objectName, bucketName, bucketNamespace, file)
		if err != nil {
			return fmt.Errorf("failed to upload the image: %v", err)
		}

		imageID, err := uploader.CreateImage(objectName, bucketName, bucketNamespace, compartment, fileName)
		if err != nil {
			return fmt.Errorf("failed to create the image from storage object: %v", err)
		}

		fmt.Printf("Image %s was uploaded and created successfully\n", imageID)
		return nil
	},
}

func main() {
	i, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	uploadCmd.Flags().StringVarP(&tenancy, "tenancy", "t", "", "target tenancy")
	uploadCmd.Flags().StringVarP(&region, "region", "r", "", "target region")
	uploadCmd.Flags().StringVarP(&userID, "user-id", "u", "", "user OCI ID")
	uploadCmd.Flags().StringVarP(&bucketName, "bucket-name", "b", "", "target OCI bucket name")
	uploadCmd.Flags().StringVarP(&bucketNamespace, "bucket-namespace", "", "", "target OCI bucket namespace")
	uploadCmd.Flags().StringVarP(&fileName, "filename", "f", "", "image file to upload")
	uploadCmd.Flags().StringVarP(&objectName, "object-name", "o", fmt.Sprintf("osbuild-upload-%v", i), "the target name of the uploaded object in the bucket")
	uploadCmd.Flags().StringVarP(&privateKeyFile, "private-key", "p", "", "private key for authenticating OCI API requests")
	uploadCmd.Flags().StringVarP(&fingerprint, "fingerprint", "", "", "the private key's fingerprint")
	uploadCmd.Flags().StringVarP(&compartment, "compartment-id", "c", "", "the compartment ID of the target image")
	_ = uploadCmd.MarkFlagRequired("bucket-name")
	_ = uploadCmd.MarkFlagRequired("bucket-namespace")
	_ = uploadCmd.MarkFlagRequired("compartment-id")
	_ = uploadCmd.MarkFlagRequired("filename")
	if err := uploadCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func uploaderFromConfig() (oci.Uploader, error) {
	if privateKeyFile != "" {
		if tenancy == "" || region == "" || userID == "" || fingerprint == "" {
			return nil, fmt.Errorf("when suppling a private key the following args are mandatory as well:" +
				" fingerprint, tenancy, region, and user-id")
		}
		pk, err := os.ReadFile(privateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file %w", err)
		}
		uploader, err := oci.NewClient(
			&oci.ClientParams{
				Tenancy:     tenancy,
				User:        userID,
				Region:      region,
				PrivateKey:  string(pk),
				Fingerprint: fingerprint,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to create an OCI client %w", err)
		}
		return uploader, nil
	}

	fmt.Printf("Creating an uploader from default config\n")
	uploader, err := oci.NewClient(nil)
	if err != nil {
		return nil, err
	}
	return uploader, nil
}
