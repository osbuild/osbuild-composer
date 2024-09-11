#!/usr/bin/python3

""" Used by AppSRE indirectly when building AMIs.
Builds osbuild-composer & osbuild rpms for one or more architectures. """

import argparse
import json
import os
import pathlib
import subprocess
import sys
import uuid
import boto3

arch_info = {}
arch_info["x86_64"] = {
    "ImageId": "ami-0282d8d07a1c0beff",
    "InstanceType": "m7a.large"
}
arch_info["aarch64"] = {
    "ImageId": "ami-053edae5f55c18809",
    "InstanceType": "m7g.large"
}


class fg:  # pylint: disable=too-few-public-methods
    """Set of constants to print colored output in the terminal"""
    BOLD = '\033[1m'  # bold
    OK = '\033[32m'  # green
    INFO = '\033[33m'  # yellow
    ERROR = '\033[31m'  # red
    RESET = '\033[0m'  # reset


def msg_error(body):
    print(f"{fg.ERROR}{fg.BOLD}Error:{fg.RESET} {body}")


def msg_info(body):
    print(f"{fg.INFO}{fg.BOLD}Info:{fg.RESET} {body}")


def msg_ok(body):
    print(f"{fg.OK}{fg.BOLD}OK:{fg.RESET} {body}")


def run_command(argv):
    subprocess.run(argv, check=True)


def create_cleanup_function(name, f, *args):
    def fun():
        msg_info("Cleaning up: " + name)
        try:
            f(*args)
        except Exception as ex:
            msg_error("during cleanup: " + str(ex))

    return fun


def stage(name, params, fun, *args):
    msg_info(name + ','.join(params))

    try:
        ret = fun(*args)
    except Exception as e:
        msg_error(f"{name} {','.join(params)} failed: {e}")
        raise

    msg_ok(f"{name} {','.join(params)}")
    return ret


def create_keypair(cleanup_actions):
    ec2 = boto3.client('ec2')
    keyname = f'rpm-builder-{uuid.uuid4()}'
    response = ec2.create_key_pair(KeyName=keyname)
    cleanup_actions += [
        create_cleanup_function(
            f"keypair {keyname}",
            lambda k: ec2.delete_key_pair(
                KeyName=k), keyname)]
    return keyname, response['KeyMaterial']


def create_ec2_instances(cleanup_actions, args, keypair):
    ec2 = boto3.resource('ec2')

    instances = []
    for a in args.arch:
        tags = [
            {
                "ResourceType": "instance",
                "Tags": [
                    {
                        "Key": "name",
                        "Value": f"rpm-builder-{uuid.uuid4()}"
                    },
                    {
                        "Key": "commit",
                        "Value": f"{args.commit}"
                    },

                ]
            }
        ]

        img = ec2.describe_images(ImageIds=[arch_info[a]["ImageId"]])
        instance = ec2.create_instances(
            ImageId=arch_info[a]["ImageId"],
            MinCount=1,
            MaxCount=1,
            InstanceType=arch_info[a]["InstanceType"],
            BlockDeviceMappings=[
                {
                    "DeviceName": img['Images'][0]['RootDeviceName'],
                    "Ebs": {
                        "VolumeSize": 20,
                        "DeleteOnTermination": True,
                        "VolumeType": "gp2",
                    },
                },
            ],
            KeyName=keypair,
            TagSpecifications=tags
        )
        instances += instance

    for i in instances:
        cleanup_actions += [
            create_cleanup_function(
                f"instance {i.id}",
                lambda x: x.terminate(),
                i)]
        i.wait_until_running()
        i.reload()

    return instances


def setup_ansible(args, instances):
    with open(os.path.join(args.base_dir, "tools", "appsre-ansible", "inventory"), 'w') as f:
        f.write("[rpmbuilder]\n")
        for i in instances:
            f.write(f"{i.public_ip_address}\n")


def run_ansible(args, key_material):
    os.umask(0)
    keypath = os.path.join(args.base_dir, "keypair.pem")

    with open(os.open(keypath, os.O_CREAT | os.O_WRONLY, 0o600), 'w') as f:
        f.write(key_material)

    with open(args.base_dir / "Schutzfile") as f:
        osbuild_commit = json.load(
            f)["centos-stream-9"]["dependencies"]["osbuild"]["commit"]

    return run_command(["ansible-playbook",
                        "--ssh-extra-args", "-o ControlPersist=no -o StrictHostKeyChecking=no -o ServerAliveInterval=5",
                        "-i", f"{args.base_dir}/tools/appsre-ansible/inventory",
                        "--key-file", keypath,
                        "-e", f"COMPOSER_COMMIT={args.commit}",
                        "-e", f"OSBUILD_COMMIT={osbuild_commit}",
                        "-e", f"RH_ACTIVATION_KEY={os.environ['RH_ACTIVATION_KEY']}",
                        "-e", f"RH_ORG_ID={os.environ['RH_ORG_ID']}",
                        f"{args.base_dir}/tools/appsre-ansible/rpmbuild.yml"])


def stage_generate_rpms(cleanup_actions, args):
    keyname, key_material = stage(
        "Create keypair", (), create_keypair, cleanup_actions)
    instances = stage("Create EC2 instances", (),
                      create_ec2_instances, cleanup_actions, args, keyname)
    stage("Setup ansible", (), setup_ansible, args, instances)
    stage("Run Ansible playbook", (), run_ansible, args, key_material)


def check_env():
    required_envvars = ["RH_ORG_ID", "RH_ACTIVATION_KEY"]
    if not all(i in os.environ for i in required_envvars):
        msg_error(
            f"At least one of the required environment variables is missing: {required_envvars}")
        sys.exit(1)


def check_params():
    parser = argparse.ArgumentParser()

    parser.add_argument('--base-dir', type=pathlib.Path, required=True,
                        help='Base directory for creating files')

    parser.add_argument('--commit', type=str, required=True,
                        help='Commit SHA')

    parser.add_argument('arch', type=str, nargs='+', choices=arch_info.keys(),
                        help='Architectures to build images for')

    args = parser.parse_args()

    return args


def print_params(p):
    msg_info(f"Building {p.arch} RPMs with base-dir {p.base_dir}")


if __name__ == "__main__":

    cleanup_actions = []

    check_env()
    args = check_params()
    print_params(args)

    try:
        stage_generate_rpms(cleanup_actions, args)
    finally:
        for c in cleanup_actions:
            c()
