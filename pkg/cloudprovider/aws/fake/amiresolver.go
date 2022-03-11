/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fake

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type AMIResolver struct {
	Images map[string]ec2.Image
}

func (c *AMIResolver) GetImage(_ context.Context, imageID string) (*ec2.Image, error) {
	if value, ok := c.Images[imageID]; ok {
		return &value, nil
	}
	return nil, fmt.Errorf("image %s not found", imageID)
}

func NewDefaultAMIResolver() *AMIResolver {
	// 	/aws/service/eks/optimized-ami/1.21/amazon-linux-2-arm64/recommended/image_id -> ami-002a052abdc5fff1c
	return &AMIResolver{
		Images: map[string]ec2.Image{
			"ami-002a052abdc5fff1c": { // Amazon Linux 2
				ImageId:        aws.String("ami-002a052abdc5fff1c"),
				RootDeviceName: aws.String("/dev/xvda"),
				Architecture:   aws.String("arm64"),
				RootDeviceType: aws.String("ebs"),
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(20),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-0e5fdfcc2c13deef9"),
							Encrypted:           aws.Bool(false),
						},
					},
				},
			},
			"ami-015c52b52fe1c5990": { // Amazon Linux 2
				ImageId:        aws.String("ami-015c52b52fe1c5990"),
				RootDeviceName: aws.String("/dev/xvda"),
				Architecture:   aws.String("x86_64"),
				RootDeviceType: aws.String("ebs"),
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(20),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-015d49dada6634f48"),
							Encrypted:           aws.Bool(false),
						},
					},
				},
			},
			"ami-03a9a7e59a2817979": { // Ubuntu
				ImageId:        aws.String("ami-03a9a7e59a2817979"),
				RootDeviceName: aws.String("/dev/sda1"),
				Architecture:   aws.String("x86_64"),
				RootDeviceType: aws.String("ebs"),
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(20),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-091e4de27d21f5811"),
							Encrypted:           aws.Bool(false),
						},
					},
					{
						DeviceName:  aws.String("/dev/sdb"),
						VirtualName: aws.String("ephemeral0"),
					},
					{
						DeviceName:  aws.String("/dev/sdc"),
						VirtualName: aws.String("ephemeral1"),
					},
				},
			},
			"ami-07f241bb8b6c4db85": { // Ubuntu
				ImageId:        aws.String("ami-07f241bb8b6c4db85"),
				RootDeviceName: aws.String("/dev/sda1"),
				Architecture:   aws.String("arm64"),
				RootDeviceType: aws.String("ebs"),
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(20),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-068b5e6d9b33a34dd"),
							Encrypted:           aws.Bool(false),
						},
					},
					{
						DeviceName:  aws.String("/dev/sdb"),
						VirtualName: aws.String("ephemeral0"),
					},
					{
						DeviceName:  aws.String("/dev/sdc"),
						VirtualName: aws.String("ephemeral1"),
					},
				},
			},

			"ami-07095e4c08d56c3ec": { // Bottlerocket
				ImageId:        aws.String("ami-07095e4c08d56c3ec"),
				RootDeviceName: aws.String("/dev/xvda"),
				Architecture:   aws.String("arm64"),
				RootDeviceType: aws.String("ebs"),
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(2),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-003a2b1fd1e296236"),
							Encrypted:           aws.Bool(false),
						},
					},
					{
						DeviceName: aws.String("/dev/xvdb"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(20),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-084e7ad5bc8b1f285"),
							Encrypted:           aws.Bool(false),
						},
					},
				},
			},
			"ami-08ae42adf8bb7b8b0": { // Bottlerocket
				ImageId:        aws.String("ami-08ae42adf8bb7b8b0"),
				RootDeviceName: aws.String("/dev/xvda"),
				Architecture:   aws.String("arm64"),
				RootDeviceType: aws.String("ebs"),
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/sda1"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(2),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-003a2b1fd1e296236"),
							Encrypted:           aws.Bool(false),
						},
					},
					{
						DeviceName: aws.String("/dev/xvdb"),
						Ebs: &ec2.EbsBlockDevice{
							VolumeSize:          aws.Int64(20),
							VolumeType:          aws.String("gp2"),
							DeleteOnTermination: aws.Bool(true),
							SnapshotId:          aws.String("snap-084e7ad5bc8b1f285"),
							Encrypted:           aws.Bool(false),
						},
					},
				},
			},
		},
	}
}
