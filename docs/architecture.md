# Architecture

## Introduction

[osbuild-composer](https://github.com/osbuild/osbuild-composer)'s role is to be the spider in a web that allows a service like architecture
around [osbuild](https://github.com/osbuild/osbuild) at the lower level. At a glance osbuild-composer provides an API for other tools to talk to and to distribute any
requests to workers which execute tasks as needed.

## Common

### Jobqueue

If an API request results in a task that needs to be executed by a worker then a job is created in the jobqueue. osbuild-composer uses PostgreSQL as the backend for its jobqueue.

### Worker

Workers are separate processes that look at the jobqueue and take any jobs that they can perform. Jobs are differentiated on architecture and/or operating system.

#### Jobsite

When a worker has accepted a job it will setup a temporary area to do this work in. This area is called the jobsite. The area takes the form of a temporary network in which a few processes are ran that are optionally ran in separate virtual machines.

The jobsite goes through several distinct phases which happen in lockstep between the `manager` and the `builder`, these phases are:

- Claim -- The manager ensures the builder has started up and is available.
- Provision -- The manager sends an osbuild manifest to the builder.
- Populate -- The manager sends over any resources needed for the builder.
- Build -- The manager sends over arguments for the build and the builder starts building.
- Progress -- While the build is ongoing information about the status of the build is continuously available.
- Export -- The manager retrieves any artifacts produced by the builder.

During each phase only expected data can be exchanged, any violation of the flow leads to the premature exit and cleanup of the jobsite.

##### Manager

The manager is contained within the jobsite and is the passthrough between the worker and the builder it takes in a manifest from the worker and then sends that off to the builder.

##### Builder

The builder is a HTTP API that wraps [osbuild](https://github.com/osbuild/osbuild). It takes HTTP requests from the manager and translates them into setting up the necessary environment after which it executes osbuild. Once osbuild has finished it provides any result(s) and log(s) back to the manager so it can be relayed back up the stack.

As a separate service the builder can be ran in several separation levels from the manager and host. The requirement is that they can speak to eachother over a network. The builder is generally ran as a separate virtual machine in the hosted version and is often ran as a separate uncontained process when ran on premise.