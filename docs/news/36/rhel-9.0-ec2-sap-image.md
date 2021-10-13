# Add support for official RHEL EC2 SAP image on RHEL-9.0

OSBuild Composer can now build the RHEL 9.0 EC2 SAP image called `ec2-sap`,
which is based on the official RHEL EC2 SAP image. The image type is not
exposed through the Weldr API, because its default package set includes the
RHUI client packages, which are not publicly available.
