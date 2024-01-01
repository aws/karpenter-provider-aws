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

package instancetype

// GENERATED FILE. DO NOT EDIT DIRECTLY.
// Update hack/code/bandwidth_gen.go and re-generate to edit
// You can add instance types by adding to the --instance-types CLI flag

var (
	InstanceTypeBandwidthMegabits = map[string]int64{
		// c3.large is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// c4.4xlarge is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// hpc7g.4xlarge is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// i2.2xlarge is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// m2.4xlarge is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// m4.4xlarge is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// r3.large is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// t1.micro is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		// t2.2xlarge is not available in https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html
		"t3.nano":           32,
		"t3a.nano":          32,
		"t4g.nano":          32,
		"t3.micro":          64,
		"t3a.micro":         64,
		"t4g.micro":         64,
		"t3.small":          128,
		"t3a.small":         128,
		"t4g.small":         128,
		"t3.medium":         256,
		"t3a.medium":        256,
		"t4g.medium":        256,
		"c7a.medium":        390,
		"m7a.medium":        390,
		"m7i-flex.large":    390,
		"r7a.medium":        390,
		"a1.medium":         500,
		"c6g.medium":        500,
		"c6gd.medium":       500,
		"m6g.medium":        500,
		"m6gd.medium":       500,
		"r6g.medium":        500,
		"r6gd.medium":       500,
		"x2gd.medium":       500,
		"t3.large":          512,
		"t3a.large":         512,
		"t4g.large":         512,
		"c7g.medium":        520,
		"c7gd.medium":       520,
		"m7g.medium":        520,
		"m7gd.medium":       520,
		"r7g.medium":        520,
		"r7gd.medium":       520,
		"x1e.xlarge":        625,
		"a1.large":          750,
		"c5.large":          750,
		"c5a.large":         750,
		"c5ad.large":        750,
		"c5d.large":         750,
		"c6g.large":         750,
		"c6gd.large":        750,
		"i3.large":          750,
		"m5.large":          750,
		"m5a.large":         750,
		"m5ad.large":        750,
		"m5d.large":         750,
		"m6g.large":         750,
		"m6gd.large":        750,
		"r4.large":          750,
		"r5.large":          750,
		"r5a.large":         750,
		"r5ad.large":        750,
		"r5b.large":         750,
		"r5d.large":         750,
		"r6g.large":         750,
		"r6gd.large":        750,
		"x2gd.large":        750,
		"z1d.large":         750,
		"c6a.large":         781,
		"c6i.large":         781,
		"c6id.large":        781,
		"c7a.large":         781,
		"c7i.large":         781,
		"i4g.large":         781,
		"i4i.large":         781,
		"m6a.large":         781,
		"m6i.large":         781,
		"m6id.large":        781,
		"m7a.large":         781,
		"m7i-flex.xlarge":   781,
		"m7i.large":         781,
		"r6a.large":         781,
		"r6i.large":         781,
		"r6id.large":        781,
		"r7a.large":         781,
		"r7i.large":         781,
		"r7iz.large":        781,
		"c7g.large":         937,
		"c7gd.large":        937,
		"m7g.large":         937,
		"m7gd.large":        937,
		"r7g.large":         937,
		"r7gd.large":        937,
		"t3.xlarge":         1024,
		"t3a.xlarge":        1024,
		"t4g.xlarge":        1024,
		"a1.xlarge":         1250,
		"c5.xlarge":         1250,
		"c5a.xlarge":        1250,
		"c5ad.xlarge":       1250,
		"c5d.xlarge":        1250,
		"c6g.xlarge":        1250,
		"c6gd.xlarge":       1250,
		"g5g.xlarge":        1250,
		"i3.xlarge":         1250,
		"m5.xlarge":         1250,
		"m5a.xlarge":        1250,
		"m5ad.xlarge":       1250,
		"m5d.xlarge":        1250,
		"m6g.xlarge":        1250,
		"m6gd.xlarge":       1250,
		"r4.xlarge":         1250,
		"r5.xlarge":         1250,
		"r5a.xlarge":        1250,
		"r5ad.xlarge":       1250,
		"r5b.xlarge":        1250,
		"r5d.xlarge":        1250,
		"r6g.xlarge":        1250,
		"r6gd.xlarge":       1250,
		"x1e.2xlarge":       1250,
		"x2gd.xlarge":       1250,
		"z1d.xlarge":        1250,
		"c6a.xlarge":        1562,
		"c6i.xlarge":        1562,
		"c6id.xlarge":       1562,
		"c7a.xlarge":        1562,
		"c7i.xlarge":        1562,
		"is4gen.medium":     1562,
		"m6a.xlarge":        1562,
		"m6i.xlarge":        1562,
		"m6id.xlarge":       1562,
		"m7a.xlarge":        1562,
		"m7i-flex.2xlarge":  1562,
		"m7i.xlarge":        1562,
		"r6a.xlarge":        1562,
		"r6i.xlarge":        1562,
		"r6id.xlarge":       1562,
		"r7a.xlarge":        1562,
		"r7i.xlarge":        1562,
		"r7iz.xlarge":       1562,
		"c6gn.medium":       1600,
		"i4g.xlarge":        1875,
		"i4i.xlarge":        1875,
		"x2iedn.xlarge":     1875,
		"c7g.xlarge":        1876,
		"c7gd.xlarge":       1876,
		"m7g.xlarge":        1876,
		"m7gd.xlarge":       1876,
		"r7g.xlarge":        1876,
		"r7gd.xlarge":       1876,
		"g4ad.xlarge":       2000,
		"t3.2xlarge":        2048,
		"t3a.2xlarge":       2048,
		"t4g.2xlarge":       2048,
		"inf2.xlarge":       2083,
		"i3en.large":        2100,
		"m5dn.large":        2100,
		"m5n.large":         2100,
		"r5dn.large":        2100,
		"r5n.large":         2100,
		"a1.2xlarge":        2500,
		"c5.2xlarge":        2500,
		"c5a.2xlarge":       2500,
		"c5ad.2xlarge":      2500,
		"c5d.2xlarge":       2500,
		"c6g.2xlarge":       2500,
		"c6gd.2xlarge":      2500,
		"f1.2xlarge":        2500,
		"g5.xlarge":         2500,
		"g5g.2xlarge":       2500,
		"h1.2xlarge":        2500,
		"i3.2xlarge":        2500,
		"m5.2xlarge":        2500,
		"m5a.2xlarge":       2500,
		"m5ad.2xlarge":      2500,
		"m5d.2xlarge":       2500,
		"m6g.2xlarge":       2500,
		"m6gd.2xlarge":      2500,
		"r4.2xlarge":        2500,
		"r5.2xlarge":        2500,
		"r5a.2xlarge":       2500,
		"r5ad.2xlarge":      2500,
		"r5b.2xlarge":       2500,
		"r5d.2xlarge":       2500,
		"r6g.2xlarge":       2500,
		"r6gd.2xlarge":      2500,
		"x1e.4xlarge":       2500,
		"x2gd.2xlarge":      2500,
		"z1d.2xlarge":       2500,
		"c5n.large":         3000,
		"c6gn.large":        3000,
		"d3.xlarge":         3000,
		"m5zn.large":        3000,
		"vt1.3xlarge":       3120,
		"c6a.2xlarge":       3125,
		"c6i.2xlarge":       3125,
		"c6id.2xlarge":      3125,
		"c6in.large":        3125,
		"c7a.2xlarge":       3125,
		"c7gn.medium":       3125,
		"c7i.2xlarge":       3125,
		"im4gn.large":       3125,
		"is4gen.large":      3125,
		"m6a.2xlarge":       3125,
		"m6i.2xlarge":       3125,
		"m6id.2xlarge":      3125,
		"m6idn.large":       3125,
		"m6in.large":        3125,
		"m7a.2xlarge":       3125,
		"m7i-flex.4xlarge":  3125,
		"m7i.2xlarge":       3125,
		"r6a.2xlarge":       3125,
		"r6i.2xlarge":       3125,
		"r6id.2xlarge":      3125,
		"r6idn.large":       3125,
		"r6in.large":        3125,
		"r7a.2xlarge":       3125,
		"r7i.2xlarge":       3125,
		"r7iz.2xlarge":      3125,
		"trn1.2xlarge":      3125,
		"c7g.2xlarge":       3750,
		"c7gd.2xlarge":      3750,
		"m7g.2xlarge":       3750,
		"m7gd.2xlarge":      3750,
		"r7g.2xlarge":       3750,
		"r7gd.2xlarge":      3750,
		"m5dn.xlarge":       4100,
		"m5n.xlarge":        4100,
		"r5dn.xlarge":       4100,
		"r5n.xlarge":        4100,
		"g4ad.2xlarge":      4167,
		"i3en.xlarge":       4200,
		"i4g.2xlarge":       4687,
		"i4i.2xlarge":       4687,
		"a1.4xlarge":        5000,
		"a1.metal":          5000,
		"c5.4xlarge":        5000,
		"c5a.4xlarge":       5000,
		"c5ad.4xlarge":      5000,
		"c5d.4xlarge":       5000,
		"c5n.xlarge":        5000,
		"c6g.4xlarge":       5000,
		"c6gd.4xlarge":      5000,
		"f1.4xlarge":        5000,
		"g3.4xlarge":        5000,
		"g4dn.xlarge":       5000,
		"g5.2xlarge":        5000,
		"g5g.4xlarge":       5000,
		"h1.4xlarge":        5000,
		"i3.4xlarge":        5000,
		"inf1.2xlarge":      5000,
		"inf1.xlarge":       5000,
		"m5.4xlarge":        5000,
		"m5a.4xlarge":       5000,
		"m5ad.4xlarge":      5000,
		"m5d.4xlarge":       5000,
		"m5zn.xlarge":       5000,
		"m6g.4xlarge":       5000,
		"m6gd.4xlarge":      5000,
		"r4.4xlarge":        5000,
		"r5.4xlarge":        5000,
		"r5a.4xlarge":       5000,
		"r5ad.4xlarge":      5000,
		"r5b.4xlarge":       5000,
		"r5d.4xlarge":       5000,
		"r6g.4xlarge":       5000,
		"r6gd.4xlarge":      5000,
		"x1e.8xlarge":       5000,
		"x2gd.4xlarge":      5000,
		"x2iedn.2xlarge":    5000,
		"z1d.3xlarge":       5000,
		"d3.2xlarge":        6000,
		"d3en.xlarge":       6000,
		"c6a.4xlarge":       6250,
		"c6i.4xlarge":       6250,
		"c6id.4xlarge":      6250,
		"c6in.xlarge":       6250,
		"c7a.4xlarge":       6250,
		"c7gn.large":        6250,
		"c7i.4xlarge":       6250,
		"im4gn.xlarge":      6250,
		"is4gen.xlarge":     6250,
		"m6a.4xlarge":       6250,
		"m6i.4xlarge":       6250,
		"m6id.4xlarge":      6250,
		"m6idn.xlarge":      6250,
		"m6in.xlarge":       6250,
		"m7a.4xlarge":       6250,
		"m7i-flex.8xlarge":  6250,
		"m7i.4xlarge":       6250,
		"r6a.4xlarge":       6250,
		"r6i.4xlarge":       6250,
		"r6id.4xlarge":      6250,
		"r6idn.xlarge":      6250,
		"r6in.xlarge":       6250,
		"r7a.4xlarge":       6250,
		"r7i.4xlarge":       6250,
		"r7iz.4xlarge":      6250,
		"vt1.6xlarge":       6250,
		"c6gn.xlarge":       6300,
		"c7g.4xlarge":       7500,
		"c7gd.4xlarge":      7500,
		"m5a.8xlarge":       7500,
		"m5ad.8xlarge":      7500,
		"m7g.4xlarge":       7500,
		"m7gd.4xlarge":      7500,
		"r5a.8xlarge":       7500,
		"r5ad.8xlarge":      7500,
		"r7g.4xlarge":       7500,
		"r7gd.4xlarge":      7500,
		"m5dn.2xlarge":      8125,
		"m5n.2xlarge":       8125,
		"r5dn.2xlarge":      8125,
		"r5n.2xlarge":       8125,
		"g4ad.4xlarge":      8333,
		"i3en.2xlarge":      8400,
		"i4g.4xlarge":       9375,
		"i4i.4xlarge":       9375,
		"c3.8xlarge":        10000,
		"c4.8xlarge":        10000,
		"c5a.8xlarge":       10000,
		"c5ad.8xlarge":      10000,
		"c5n.2xlarge":       10000,
		"d2.8xlarge":        10000,
		"g3.8xlarge":        10000,
		"g4dn.2xlarge":      10000,
		"g5.4xlarge":        10000,
		"h1.8xlarge":        10000,
		"i2.8xlarge":        10000,
		"i3.8xlarge":        10000,
		"m4.10xlarge":       10000,
		"m5.8xlarge":        10000,
		"m5a.12xlarge":      10000,
		"m5ad.12xlarge":     10000,
		"m5d.8xlarge":       10000,
		"m5zn.2xlarge":      10000,
		"mac2-m2.metal":     10000,
		"mac2-m2pro.metal":  10000,
		"mac2.metal":        10000,
		"p2.8xlarge":        10000,
		"p3.8xlarge":        10000,
		"r3.8xlarge":        10000,
		"r4.8xlarge":        10000,
		"r5.8xlarge":        10000,
		"r5a.12xlarge":      10000,
		"r5ad.12xlarge":     10000,
		"r5b.8xlarge":       10000,
		"r5d.8xlarge":       10000,
		"x1.16xlarge":       10000,
		"x1e.16xlarge":      10000,
		"c5.12xlarge":       12000,
		"c5.9xlarge":        12000,
		"c5a.12xlarge":      12000,
		"c5ad.12xlarge":     12000,
		"c5d.12xlarge":      12000,
		"c5d.9xlarge":       12000,
		"c6g.8xlarge":       12000,
		"c6gd.8xlarge":      12000,
		"g5g.8xlarge":       12000,
		"m5.12xlarge":       12000,
		"m5a.16xlarge":      12000,
		"m5ad.16xlarge":     12000,
		"m5d.12xlarge":      12000,
		"m6g.8xlarge":       12000,
		"m6gd.8xlarge":      12000,
		"r5.12xlarge":       12000,
		"r5a.16xlarge":      12000,
		"r5ad.16xlarge":     12000,
		"r5b.12xlarge":      12000,
		"r5d.12xlarge":      12000,
		"r6g.8xlarge":       12000,
		"r6gd.8xlarge":      12000,
		"x2gd.8xlarge":      12000,
		"z1d.6xlarge":       12000,
		"c6a.8xlarge":       12500,
		"c6gn.2xlarge":      12500,
		"c6i.8xlarge":       12500,
		"c6id.8xlarge":      12500,
		"c6in.2xlarge":      12500,
		"c7a.8xlarge":       12500,
		"c7gn.xlarge":       12500,
		"c7i.8xlarge":       12500,
		"d3.4xlarge":        12500,
		"d3en.2xlarge":      12500,
		"i3en.3xlarge":      12500,
		"im4gn.2xlarge":     12500,
		"is4gen.2xlarge":    12500,
		"m6a.8xlarge":       12500,
		"m6i.8xlarge":       12500,
		"m6id.8xlarge":      12500,
		"m6idn.2xlarge":     12500,
		"m6in.2xlarge":      12500,
		"m7a.8xlarge":       12500,
		"m7i.8xlarge":       12500,
		"r6a.8xlarge":       12500,
		"r6i.8xlarge":       12500,
		"r6id.8xlarge":      12500,
		"r6idn.2xlarge":     12500,
		"r6in.2xlarge":      12500,
		"r7a.8xlarge":       12500,
		"r7i.8xlarge":       12500,
		"r7iz.8xlarge":      12500,
		"x2iedn.4xlarge":    12500,
		"x2iezn.2xlarge":    12500,
		"c5n.4xlarge":       15000,
		"c7g.8xlarge":       15000,
		"c7gd.8xlarge":      15000,
		"g4ad.8xlarge":      15000,
		"m5zn.3xlarge":      15000,
		"m7g.8xlarge":       15000,
		"m7gd.8xlarge":      15000,
		"r7g.8xlarge":       15000,
		"r7gd.8xlarge":      15000,
		"x2iezn.4xlarge":    15000,
		"m5dn.4xlarge":      16250,
		"m5n.4xlarge":       16250,
		"r5dn.4xlarge":      16250,
		"r5n.4xlarge":       16250,
		"inf2.8xlarge":      16667,
		"c6a.12xlarge":      18750,
		"c6i.12xlarge":      18750,
		"c6id.12xlarge":     18750,
		"c7a.12xlarge":      18750,
		"c7i.12xlarge":      18750,
		"i4g.8xlarge":       18750,
		"i4i.8xlarge":       18750,
		"m6a.12xlarge":      18750,
		"m6i.12xlarge":      18750,
		"m6id.12xlarge":     18750,
		"m7a.12xlarge":      18750,
		"m7i.12xlarge":      18750,
		"r6a.12xlarge":      18750,
		"r6i.12xlarge":      18750,
		"r6id.12xlarge":     18750,
		"r7a.12xlarge":      18750,
		"r7i.12xlarge":      18750,
		"c5a.16xlarge":      20000,
		"c5a.24xlarge":      20000,
		"c5ad.16xlarge":     20000,
		"c5ad.24xlarge":     20000,
		"c6g.12xlarge":      20000,
		"c6gd.12xlarge":     20000,
		"g4dn.4xlarge":      20000,
		"m5.16xlarge":       20000,
		"m5a.24xlarge":      20000,
		"m5ad.24xlarge":     20000,
		"m5d.16xlarge":      20000,
		"m6g.12xlarge":      20000,
		"m6gd.12xlarge":     20000,
		"r5.16xlarge":       20000,
		"r5a.24xlarge":      20000,
		"r5ad.24xlarge":     20000,
		"r5b.16xlarge":      20000,
		"r5d.16xlarge":      20000,
		"r6g.12xlarge":      20000,
		"r6gd.12xlarge":     20000,
		"x2gd.12xlarge":     20000,
		"c7g.12xlarge":      22500,
		"c7gd.12xlarge":     22500,
		"m7g.12xlarge":      22500,
		"m7gd.12xlarge":     22500,
		"r7g.12xlarge":      22500,
		"r7gd.12xlarge":     22500,
		"c5.18xlarge":       25000,
		"c5.24xlarge":       25000,
		"c5.metal":          25000,
		"c5d.18xlarge":      25000,
		"c5d.24xlarge":      25000,
		"c5d.metal":         25000,
		"c6a.16xlarge":      25000,
		"c6g.16xlarge":      25000,
		"c6g.metal":         25000,
		"c6gd.16xlarge":     25000,
		"c6gd.metal":        25000,
		"c6gn.4xlarge":      25000,
		"c6i.16xlarge":      25000,
		"c6id.16xlarge":     25000,
		"c6in.4xlarge":      25000,
		"c7a.16xlarge":      25000,
		"c7gn.2xlarge":      25000,
		"c7i.16xlarge":      25000,
		"d3.8xlarge":        25000,
		"d3en.4xlarge":      25000,
		"f1.16xlarge":       25000,
		"g3.16xlarge":       25000,
		"g4ad.16xlarge":     25000,
		"g5.16xlarge":       25000,
		"g5.8xlarge":        25000,
		"g5g.16xlarge":      25000,
		"g5g.metal":         25000,
		"h1.16xlarge":       25000,
		"i3.16xlarge":       25000,
		"i3.metal":          25000,
		"i3en.6xlarge":      25000,
		"im4gn.4xlarge":     25000,
		"inf1.6xlarge":      25000,
		"is4gen.4xlarge":    25000,
		"m4.16xlarge":       25000,
		"m5.24xlarge":       25000,
		"m5.metal":          25000,
		"m5d.24xlarge":      25000,
		"m5d.metal":         25000,
		"m5dn.8xlarge":      25000,
		"m5n.8xlarge":       25000,
		"m6a.16xlarge":      25000,
		"m6g.16xlarge":      25000,
		"m6g.metal":         25000,
		"m6gd.16xlarge":     25000,
		"m6gd.metal":        25000,
		"m6i.16xlarge":      25000,
		"m6id.16xlarge":     25000,
		"m6idn.4xlarge":     25000,
		"m6in.4xlarge":      25000,
		"m7a.16xlarge":      25000,
		"m7i.16xlarge":      25000,
		"mac1.metal":        25000,
		"p2.16xlarge":       25000,
		"p3.16xlarge":       25000,
		"r4.16xlarge":       25000,
		"r5.24xlarge":       25000,
		"r5.metal":          25000,
		"r5b.24xlarge":      25000,
		"r5b.metal":         25000,
		"r5d.24xlarge":      25000,
		"r5d.metal":         25000,
		"r5dn.8xlarge":      25000,
		"r5n.8xlarge":       25000,
		"r6a.16xlarge":      25000,
		"r6g.16xlarge":      25000,
		"r6g.metal":         25000,
		"r6gd.16xlarge":     25000,
		"r6gd.metal":        25000,
		"r6i.16xlarge":      25000,
		"r6id.16xlarge":     25000,
		"r6idn.4xlarge":     25000,
		"r6in.4xlarge":      25000,
		"r7a.16xlarge":      25000,
		"r7i.16xlarge":      25000,
		"r7iz.12xlarge":     25000,
		"r7iz.16xlarge":     25000,
		"r7iz.metal-16xl":   25000,
		"vt1.24xlarge":      25000,
		"x1.32xlarge":       25000,
		"x1e.32xlarge":      25000,
		"x2gd.16xlarge":     25000,
		"x2gd.metal":        25000,
		"x2iedn.8xlarge":    25000,
		"z1d.12xlarge":      25000,
		"z1d.metal":         25000,
		"i4i.12xlarge":      28120,
		"c7g.16xlarge":      30000,
		"c7g.metal":         30000,
		"c7gd.16xlarge":     30000,
		"m7g.16xlarge":      30000,
		"m7g.metal":         30000,
		"m7gd.16xlarge":     30000,
		"r7g.16xlarge":      30000,
		"r7g.metal":         30000,
		"r7gd.16xlarge":     30000,
		"c6a.24xlarge":      37500,
		"c6i.24xlarge":      37500,
		"c6id.24xlarge":     37500,
		"c7a.24xlarge":      37500,
		"c7i.24xlarge":      37500,
		"c7i.metal-24xl":    37500,
		"i4g.16xlarge":      37500,
		"i4i.16xlarge":      37500,
		"m6a.24xlarge":      37500,
		"m6i.24xlarge":      37500,
		"m6id.24xlarge":     37500,
		"m7a.24xlarge":      37500,
		"m7i.24xlarge":      37500,
		"m7i.metal-24xl":    37500,
		"r6a.24xlarge":      37500,
		"r6i.24xlarge":      37500,
		"r6id.24xlarge":     37500,
		"r7a.24xlarge":      37500,
		"r7i.24xlarge":      37500,
		"r7i.metal-24xl":    37500,
		"d3en.6xlarge":      40000,
		"g5.12xlarge":       40000,
		"c5n.9xlarge":       50000,
		"c6a.32xlarge":      50000,
		"c6a.48xlarge":      50000,
		"c6a.metal":         50000,
		"c6gn.8xlarge":      50000,
		"c6i.32xlarge":      50000,
		"c6i.metal":         50000,
		"c6id.32xlarge":     50000,
		"c6id.metal":        50000,
		"c6in.8xlarge":      50000,
		"c7a.32xlarge":      50000,
		"c7a.48xlarge":      50000,
		"c7a.metal-48xl":    50000,
		"c7gn.4xlarge":      50000,
		"c7i.48xlarge":      50000,
		"c7i.metal-48xl":    50000,
		"d3en.8xlarge":      50000,
		"g4dn.12xlarge":     50000,
		"g4dn.16xlarge":     50000,
		"g4dn.8xlarge":      50000,
		"g5.24xlarge":       50000,
		"i3en.12xlarge":     50000,
		"im4gn.8xlarge":     50000,
		"inf2.24xlarge":     50000,
		"is4gen.8xlarge":    50000,
		"m5dn.12xlarge":     50000,
		"m5n.12xlarge":      50000,
		"m5zn.6xlarge":      50000,
		"m6a.32xlarge":      50000,
		"m6a.48xlarge":      50000,
		"m6a.metal":         50000,
		"m6i.32xlarge":      50000,
		"m6i.metal":         50000,
		"m6id.32xlarge":     50000,
		"m6id.metal":        50000,
		"m6idn.8xlarge":     50000,
		"m6in.8xlarge":      50000,
		"m7a.32xlarge":      50000,
		"m7a.48xlarge":      50000,
		"m7a.metal-48xl":    50000,
		"m7i.48xlarge":      50000,
		"m7i.metal-48xl":    50000,
		"r5dn.12xlarge":     50000,
		"r5n.12xlarge":      50000,
		"r6a.32xlarge":      50000,
		"r6a.48xlarge":      50000,
		"r6a.metal":         50000,
		"r6i.32xlarge":      50000,
		"r6i.metal":         50000,
		"r6id.32xlarge":     50000,
		"r6id.metal":        50000,
		"r6idn.8xlarge":     50000,
		"r6in.8xlarge":      50000,
		"r7a.32xlarge":      50000,
		"r7a.48xlarge":      50000,
		"r7a.metal-48xl":    50000,
		"r7i.48xlarge":      50000,
		"r7i.metal-48xl":    50000,
		"r7iz.32xlarge":     50000,
		"r7iz.metal-32xl":   50000,
		"u-3tb1.56xlarge":   50000,
		"x2idn.16xlarge":    50000,
		"x2iedn.16xlarge":   50000,
		"x2iezn.6xlarge":    50000,
		"i4i.24xlarge":      56250,
		"c6gn.12xlarge":     75000,
		"c6in.12xlarge":     75000,
		"d3en.12xlarge":     75000,
		"i4i.32xlarge":      75000,
		"i4i.metal":         75000,
		"m5dn.16xlarge":     75000,
		"m5n.16xlarge":      75000,
		"m6idn.12xlarge":    75000,
		"m6in.12xlarge":     75000,
		"r5dn.16xlarge":     75000,
		"r5n.16xlarge":      75000,
		"r6idn.12xlarge":    75000,
		"r6in.12xlarge":     75000,
		"x2idn.24xlarge":    75000,
		"x2iedn.24xlarge":   75000,
		"x2iezn.8xlarge":    75000,
		"c5n.18xlarge":      100000,
		"c5n.metal":         100000,
		"c6gn.16xlarge":     100000,
		"c6in.16xlarge":     100000,
		"c7gn.8xlarge":      100000,
		"dl2q.24xlarge":     100000,
		"g4dn.metal":        100000,
		"g5.48xlarge":       100000,
		"i3en.24xlarge":     100000,
		"i3en.metal":        100000,
		"im4gn.16xlarge":    100000,
		"inf1.24xlarge":     100000,
		"inf2.48xlarge":     100000,
		"m5dn.24xlarge":     100000,
		"m5dn.metal":        100000,
		"m5n.24xlarge":      100000,
		"m5n.metal":         100000,
		"m5zn.12xlarge":     100000,
		"m5zn.metal":        100000,
		"m6idn.16xlarge":    100000,
		"m6in.16xlarge":     100000,
		"p3dn.24xlarge":     100000,
		"r5dn.24xlarge":     100000,
		"r5dn.metal":        100000,
		"r5n.24xlarge":      100000,
		"r5n.metal":         100000,
		"r6idn.16xlarge":    100000,
		"r6in.16xlarge":     100000,
		"u-12tb1.112xlarge": 100000,
		"u-12tb1.metal":     100000,
		"u-18tb1.112xlarge": 100000,
		"u-18tb1.metal":     100000,
		"u-24tb1.112xlarge": 100000,
		"u-24tb1.metal":     100000,
		"u-6tb1.112xlarge":  100000,
		"u-6tb1.56xlarge":   100000,
		"u-6tb1.metal":      100000,
		"u-9tb1.112xlarge":  100000,
		"u-9tb1.metal":      100000,
		"x2idn.32xlarge":    100000,
		"x2idn.metal":       100000,
		"x2iedn.32xlarge":   100000,
		"x2iedn.metal":      100000,
		"x2iezn.12xlarge":   100000,
		"x2iezn.metal":      100000,
		"c6in.24xlarge":     150000,
		"c7gn.12xlarge":     150000,
		"m6idn.24xlarge":    150000,
		"m6in.24xlarge":     150000,
		"r6idn.24xlarge":    150000,
		"r6in.24xlarge":     150000,
		"c6in.32xlarge":     200000,
		"c6in.metal":        200000,
		"c7gn.16xlarge":     200000,
		"m6idn.32xlarge":    200000,
		"m6idn.metal":       200000,
		"m6in.32xlarge":     200000,
		"m6in.metal":        200000,
		"r6idn.32xlarge":    200000,
		"r6idn.metal":       200000,
		"r6in.32xlarge":     200000,
		"r6in.metal":        200000,
		"dl1.24xlarge":      400000,
		"p4d.24xlarge":      400000,
		"p4de.24xlarge":     400000,
		"trn1.32xlarge":     800000,
		"trn1n.32xlarge":    1600000,
		"p5.48xlarge":       3200000,
	}
)
