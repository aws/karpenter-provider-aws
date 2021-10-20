# Custom Images

## Introduction

Karpenter follows existing AWS patterns for customizing the base image of instances. More specifically, Karpenter uses EC2 launch templates. Launch templates may specify many values related to networking, authorization, instance type, and more. The base image (AMI) is a required value. Use the AWS CLI to import virtual machine images as AMIs. 

## Base Image Requirements

### Autoconfigure

Importantly, the AMI must support automatically connecting to a cluster based on "user data", or a base64 encoded string. The syntax and purpose of the user data varies between images. Bottlerocket images use TOML to specify instance configuration using a file. Amazon Linux 2 (AL2) images accept bash commands. 

In the default configuration, Karpenter uses AL2 and passes the hostname of the kubernetes API server, and a certificate. The instance subsequently uses this information to securely join the cluster.

<<How does Karpenter permit user data to be specified at run time?>>

### Instance Type

The instance type should not be specified in the launch template. Karpenter will determine the launch template at run time. 

### Instance Profile - IAM

The launch template must include an "instance profile" -- a set of IAM roles. At lest one IAM role must include permissions to join the cluster? Node role?

### storage

-- based on the AMI?

### security groups - firewall 

This is kinda where the VPC (but not subnet) comes in. 

## Creating the Launch Template

### CLI

F'it I'm doing CFN!


# Notes

- copy/paste text block
- in line edit
- file based approach
- test or ask?
 - https://github.com/kubernetes/website/pull/28226/files