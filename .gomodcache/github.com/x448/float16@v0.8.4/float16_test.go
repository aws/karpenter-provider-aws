// Copyright 2019 Montgomery Edwards⁴⁴⁸ and Faye Amacker

package float16_test

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"testing"

	"github.com/x448/float16"
)

// wantF32toF16bits is a tiny subset of expected values
var wantF32toF16bits = []struct {
	in  float32
	out uint16
}{
	// generated to provide 100% code coverage plus additional tests for rounding, etc.
	{in: math.Float32frombits(0x00000000), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x00000001), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x00001fff), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x00002000), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x00003fff), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x00004000), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x007fffff), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x00800000), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x33000000), out: 0x0000}, // in f32=0.000000, out f16=0
	{in: math.Float32frombits(0x33000001), out: 0x0001}, // in f32=0.000000, out f16=0.000000059604645
	{in: math.Float32frombits(0x33000002), out: 0x0001}, // in f32=0.000000, out f16=0.000000059604645
	{in: math.Float32frombits(0x387fc000), out: 0x03ff}, // in f32=0.000061, out f16=0.00006097555 // exp32=-15 (underflows binary16 exp) but round-trips
	{in: math.Float32frombits(0x387fffff), out: 0x0400}, // in f32=0.000061, out f16=0.000061035156
	{in: math.Float32frombits(0x38800000), out: 0x0400}, // in f32=0.000061, out f16=0.000061035156
	{in: math.Float32frombits(0x38801fff), out: 0x0401}, // in f32=0.000061, out f16=0.00006109476
	{in: math.Float32frombits(0x38802000), out: 0x0401}, // in f32=0.000061, out f16=0.00006109476
	{in: math.Float32frombits(0x38803fff), out: 0x0402}, // in f32=0.000061, out f16=0.000061154366
	{in: math.Float32frombits(0x38804000), out: 0x0402}, // in f32=0.000061, out f16=0.000061154366
	{in: math.Float32frombits(0x33bfffff), out: 0x0001}, // in f32=0.000000, out f16=0.000000059604645
	{in: math.Float32frombits(0x33c00000), out: 0x0002}, // in f32=0.000000, out f16=0.00000011920929
	{in: math.Float32frombits(0x33c00001), out: 0x0002}, // in f32=0.000000, out f16=0.00000011920929
	{in: math.Float32frombits(0x477fffff), out: 0x7c00}, // in f32=65535.996094, out f16=+Inf
	{in: math.Float32frombits(0x47800000), out: 0x7c00}, // in f32=65536.000000, out f16=+Inf
	{in: math.Float32frombits(0x7f7fffff), out: 0x7c00}, // in f32=340282346638528859811704183484516925440.000000, out f16=+Inf
	{in: math.Float32frombits(0x7f800000), out: 0x7c00}, // in f32=+Inf, out f16=+Inf
	{in: math.Float32frombits(0x7f801fff), out: 0x7e00}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0x7f802000), out: 0x7e01}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0x7f803fff), out: 0x7e01}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0x7f804000), out: 0x7e02}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0x7fffffff), out: 0x7fff}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0x80000000), out: 0x8000}, // in f32=-0.000000, out f16=-0
	{in: math.Float32frombits(0x80001fff), out: 0x8000}, // in f32=-0.000000, out f16=-0
	{in: math.Float32frombits(0x80002000), out: 0x8000}, // in f32=-0.000000, out f16=-0
	{in: math.Float32frombits(0x80003fff), out: 0x8000}, // in f32=-0.000000, out f16=-0
	{in: math.Float32frombits(0x80004000), out: 0x8000}, // in f32=-0.000000, out f16=-0
	{in: math.Float32frombits(0x807fffff), out: 0x8000}, // in f32=-0.000000, out f16=-0
	{in: math.Float32frombits(0x80800000), out: 0x8000}, // in f32=-0.000000, out f16=-0
	{in: math.Float32frombits(0xb87fc000), out: 0x83ff}, // in f32=-0.000061, out f16=-0.00006097555 // exp32=-15 (underflows binary16 exp) but round-trips
	{in: math.Float32frombits(0xb87fffff), out: 0x8400}, // in f32=-0.000061, out f16=-0.000061035156
	{in: math.Float32frombits(0xb8800000), out: 0x8400}, // in f32=-0.000061, out f16=-0.000061035156
	{in: math.Float32frombits(0xb8801fff), out: 0x8401}, // in f32=-0.000061, out f16=-0.00006109476
	{in: math.Float32frombits(0xb8802000), out: 0x8401}, // in f32=-0.000061, out f16=-0.00006109476
	{in: math.Float32frombits(0xb8803fff), out: 0x8402}, // in f32=-0.000061, out f16=-0.000061154366
	{in: math.Float32frombits(0xb8804000), out: 0x8402}, // in f32=-0.000061, out f16=-0.000061154366
	{in: math.Float32frombits(0xc77fffff), out: 0xfc00}, // in f32=-65535.996094, out f16=-Inf
	{in: math.Float32frombits(0xc7800000), out: 0xfc00}, // in f32=-65536.000000, out f16=-Inf
	{in: math.Float32frombits(0xff7fffff), out: 0xfc00}, // in f32=-340282346638528859811704183484516925440.000000, out f16=-Inf
	{in: math.Float32frombits(0xff800000), out: 0xfc00}, // in f32=-Inf, out f16=-Inf
	{in: math.Float32frombits(0xff801fff), out: 0xfe00}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0xff802000), out: 0xfe01}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0xff803fff), out: 0xfe01}, // in f32=NaN, out f16=NaN
	{in: math.Float32frombits(0xff804000), out: 0xfe02}, // in f32=NaN, out f16=NaN
	// additional tests
	{in: math.Float32frombits(0xc77ff000), out: 0xfc00}, // in f32=-65520.000000, out f16=-Inf
	{in: math.Float32frombits(0xc77fef00), out: 0xfbff}, // in f32=-65519.000000, out f16=-65504
	{in: math.Float32frombits(0xc77fee00), out: 0xfbff}, // in f32=-65518.000000, out f16=-65504
	{in: math.Float32frombits(0xc5802000), out: 0xec01}, // in f32=-4100.000000, out f16=-4100
	{in: math.Float32frombits(0xc5801800), out: 0xec01}, // in f32=-4099.000000, out f16=-4100
	{in: math.Float32frombits(0xc5801000), out: 0xec00}, // in f32=-4098.000000, out f16=-4096
	{in: math.Float32frombits(0xc5800800), out: 0xec00}, // in f32=-4097.000000, out f16=-4096
	{in: math.Float32frombits(0xc5800000), out: 0xec00}, // in f32=-4096.000000, out f16=-4096
	{in: math.Float32frombits(0xc57ff000), out: 0xec00}, // in f32=-4095.000000, out f16=-4096
	{in: math.Float32frombits(0xc57fe000), out: 0xebff}, // in f32=-4094.000000, out f16=-4094
	{in: math.Float32frombits(0xc57fd000), out: 0xebfe}, // in f32=-4093.000000, out f16=-4092
	{in: math.Float32frombits(0xc5002000), out: 0xe801}, // in f32=-2050.000000, out f16=-2050
	{in: math.Float32frombits(0xc5001000), out: 0xe800}, // in f32=-2049.000000, out f16=-2048
	{in: math.Float32frombits(0xc5000829), out: 0xe800}, // in f32=-2048.510010, out f16=-2048
	{in: math.Float32frombits(0xc5000800), out: 0xe800}, // in f32=-2048.500000, out f16=-2048
	{in: math.Float32frombits(0xc50007d7), out: 0xe800}, // in f32=-2048.489990, out f16=-2048
	{in: math.Float32frombits(0xc5000000), out: 0xe800}, // in f32=-2048.000000, out f16=-2048
	{in: math.Float32frombits(0xc4fff052), out: 0xe800}, // in f32=-2047.510010, out f16=-2048
	{in: math.Float32frombits(0xc4fff000), out: 0xe800}, // in f32=-2047.500000, out f16=-2048
	{in: math.Float32frombits(0xc4ffefae), out: 0xe7ff}, // in f32=-2047.489990, out f16=-2047
	{in: math.Float32frombits(0xc4ffe000), out: 0xe7ff}, // in f32=-2047.000000, out f16=-2047
	{in: math.Float32frombits(0xc4ffc000), out: 0xe7fe}, // in f32=-2046.000000, out f16=-2046
	{in: math.Float32frombits(0xc4ffa000), out: 0xe7fd}, // in f32=-2045.000000, out f16=-2045
	{in: math.Float32frombits(0xbf800000), out: 0xbc00}, // in f32=-1.000000, out f16=-1
	{in: math.Float32frombits(0xbf028f5c), out: 0xb814}, // in f32=-0.510000, out f16=-0.5097656
	{in: math.Float32frombits(0xbf000000), out: 0xb800}, // in f32=-0.500000, out f16=-0.5
	{in: math.Float32frombits(0xbefae148), out: 0xb7d7}, // in f32=-0.490000, out f16=-0.48999023
	{in: math.Float32frombits(0x3efae148), out: 0x37d7}, // in f32=0.490000, out f16=0.48999023
	{in: math.Float32frombits(0x3f000000), out: 0x3800}, // in f32=0.500000, out f16=0.5
	{in: math.Float32frombits(0x3f028f5c), out: 0x3814}, // in f32=0.510000, out f16=0.5097656
	{in: math.Float32frombits(0x3f800000), out: 0x3c00}, // in f32=1.000000, out f16=1
	{in: math.Float32frombits(0x3fbeb852), out: 0x3df6}, // in f32=1.490000, out f16=1.4902344
	{in: math.Float32frombits(0x3fc00000), out: 0x3e00}, // in f32=1.500000, out f16=1.5
	{in: math.Float32frombits(0x3fc147ae), out: 0x3e0a}, // in f32=1.510000, out f16=1.5097656
	{in: math.Float32frombits(0x3fcf1bbd), out: 0x3e79}, // in f32=1.618034, out f16=1.6181641
	{in: math.Float32frombits(0x401f5c29), out: 0x40fb}, // in f32=2.490000, out f16=2.4902344
	{in: math.Float32frombits(0x40200000), out: 0x4100}, // in f32=2.500000, out f16=2.5
	{in: math.Float32frombits(0x4020a3d7), out: 0x4105}, // in f32=2.510000, out f16=2.5097656
	{in: math.Float32frombits(0x402df854), out: 0x4170}, // in f32=2.718282, out f16=2.71875
	{in: math.Float32frombits(0x40490fdb), out: 0x4248}, // in f32=3.141593, out f16=3.140625
	{in: math.Float32frombits(0x40b00000), out: 0x4580}, // in f32=5.500000, out f16=5.5
	{in: math.Float32frombits(0x44ffa000), out: 0x67fd}, // in f32=2045.000000, out f16=2045
	{in: math.Float32frombits(0x44ffc000), out: 0x67fe}, // in f32=2046.000000, out f16=2046
	{in: math.Float32frombits(0x44ffe000), out: 0x67ff}, // in f32=2047.000000, out f16=2047
	{in: math.Float32frombits(0x44ffefae), out: 0x67ff}, // in f32=2047.489990, out f16=2047
	{in: math.Float32frombits(0x44fff000), out: 0x6800}, // in f32=2047.500000, out f16=2048
	{in: math.Float32frombits(0x44fff052), out: 0x6800}, // in f32=2047.510010, out f16=2048
	{in: math.Float32frombits(0x45000000), out: 0x6800}, // in f32=2048.000000, out f16=2048
	{in: math.Float32frombits(0x450007d7), out: 0x6800}, // in f32=2048.489990, out f16=2048
	{in: math.Float32frombits(0x45000800), out: 0x6800}, // in f32=2048.500000, out f16=2048
	{in: math.Float32frombits(0x45000829), out: 0x6800}, // in f32=2048.510010, out f16=2048
	{in: math.Float32frombits(0x45001000), out: 0x6800}, // in f32=2049.000000, out f16=2048
	{in: math.Float32frombits(0x450017d7), out: 0x6801}, // in f32=2049.489990, out f16=2050
	{in: math.Float32frombits(0x45001800), out: 0x6801}, // in f32=2049.500000, out f16=2050
	{in: math.Float32frombits(0x45001829), out: 0x6801}, // in f32=2049.510010, out f16=2050
	{in: math.Float32frombits(0x45002000), out: 0x6801}, // in f32=2050.000000, out f16=2050
	{in: math.Float32frombits(0x45003000), out: 0x6802}, // in f32=2051.000000, out f16=2052
	{in: math.Float32frombits(0x457fd000), out: 0x6bfe}, // in f32=4093.000000, out f16=4092
	{in: math.Float32frombits(0x457fe000), out: 0x6bff}, // in f32=4094.000000, out f16=4094
	{in: math.Float32frombits(0x457ff000), out: 0x6c00}, // in f32=4095.000000, out f16=4096
	{in: math.Float32frombits(0x45800000), out: 0x6c00}, // in f32=4096.000000, out f16=4096
	{in: math.Float32frombits(0x45800800), out: 0x6c00}, // in f32=4097.000000, out f16=4096
	{in: math.Float32frombits(0x45801000), out: 0x6c00}, // in f32=4098.000000, out f16=4096
	{in: math.Float32frombits(0x45801800), out: 0x6c01}, // in f32=4099.000000, out f16=4100
	{in: math.Float32frombits(0x45802000), out: 0x6c01}, // in f32=4100.000000, out f16=4100
	{in: math.Float32frombits(0x45ad9c00), out: 0x6d6d}, // in f32=5555.500000, out f16=5556
	{in: math.Float32frombits(0x45ffe800), out: 0x6fff}, // in f32=8189.000000, out f16=8188
	{in: math.Float32frombits(0x45fff000), out: 0x7000}, // in f32=8190.000000, out f16=8192
	{in: math.Float32frombits(0x45fff800), out: 0x7000}, // in f32=8191.000000, out f16=8192
	{in: math.Float32frombits(0x46000000), out: 0x7000}, // in f32=8192.000000, out f16=8192
	{in: math.Float32frombits(0x46000400), out: 0x7000}, // in f32=8193.000000, out f16=8192
	{in: math.Float32frombits(0x46000800), out: 0x7000}, // in f32=8194.000000, out f16=8192
	{in: math.Float32frombits(0x46000c00), out: 0x7000}, // in f32=8195.000000, out f16=8192
	{in: math.Float32frombits(0x46001000), out: 0x7000}, // in f32=8196.000000, out f16=8192
	{in: math.Float32frombits(0x46001400), out: 0x7001}, // in f32=8197.000000, out f16=8200
	{in: math.Float32frombits(0x46001800), out: 0x7001}, // in f32=8198.000000, out f16=8200
	{in: math.Float32frombits(0x46001c00), out: 0x7001}, // in f32=8199.000000, out f16=8200
	{in: math.Float32frombits(0x46002000), out: 0x7001}, // in f32=8200.000000, out f16=8200
	{in: math.Float32frombits(0x46002400), out: 0x7001}, // in f32=8201.000000, out f16=8200
	{in: math.Float32frombits(0x46002800), out: 0x7001}, // in f32=8202.000000, out f16=8200
	{in: math.Float32frombits(0x46002c00), out: 0x7001}, // in f32=8203.000000, out f16=8200
	{in: math.Float32frombits(0x46003000), out: 0x7002}, // in f32=8204.000000, out f16=8208
	{in: math.Float32frombits(0x467fec00), out: 0x73ff}, // in f32=16379.000000, out f16=16376
	{in: math.Float32frombits(0x467ff000), out: 0x7400}, // in f32=16380.000000, out f16=16384
	{in: math.Float32frombits(0x467ff400), out: 0x7400}, // in f32=16381.000000, out f16=16384
	{in: math.Float32frombits(0x467ff800), out: 0x7400}, // in f32=16382.000000, out f16=16384
	{in: math.Float32frombits(0x467ffc00), out: 0x7400}, // in f32=16383.000000, out f16=16384
	{in: math.Float32frombits(0x46800000), out: 0x7400}, // in f32=16384.000000, out f16=16384
	{in: math.Float32frombits(0x46800200), out: 0x7400}, // in f32=16385.000000, out f16=16384
	{in: math.Float32frombits(0x46800400), out: 0x7400}, // in f32=16386.000000, out f16=16384
	{in: math.Float32frombits(0x46800600), out: 0x7400}, // in f32=16387.000000, out f16=16384
	{in: math.Float32frombits(0x46800800), out: 0x7400}, // in f32=16388.000000, out f16=16384
	{in: math.Float32frombits(0x46800a00), out: 0x7400}, // in f32=16389.000000, out f16=16384
	{in: math.Float32frombits(0x46800c00), out: 0x7400}, // in f32=16390.000000, out f16=16384
	{in: math.Float32frombits(0x46800e00), out: 0x7400}, // in f32=16391.000000, out f16=16384
	{in: math.Float32frombits(0x46801000), out: 0x7400}, // in f32=16392.000000, out f16=16384
	{in: math.Float32frombits(0x46801200), out: 0x7401}, // in f32=16393.000000, out f16=16400
	{in: math.Float32frombits(0x46801400), out: 0x7401}, // in f32=16394.000000, out f16=16400
	{in: math.Float32frombits(0x46801600), out: 0x7401}, // in f32=16395.000000, out f16=16400
	{in: math.Float32frombits(0x46801800), out: 0x7401}, // in f32=16396.000000, out f16=16400
	{in: math.Float32frombits(0x46801a00), out: 0x7401}, // in f32=16397.000000, out f16=16400
	{in: math.Float32frombits(0x46801c00), out: 0x7401}, // in f32=16398.000000, out f16=16400
	{in: math.Float32frombits(0x46801e00), out: 0x7401}, // in f32=16399.000000, out f16=16400
	{in: math.Float32frombits(0x46802000), out: 0x7401}, // in f32=16400.000000, out f16=16400
	{in: math.Float32frombits(0x46802200), out: 0x7401}, // in f32=16401.000000, out f16=16400
	{in: math.Float32frombits(0x46802400), out: 0x7401}, // in f32=16402.000000, out f16=16400
	{in: math.Float32frombits(0x46802600), out: 0x7401}, // in f32=16403.000000, out f16=16400
	{in: math.Float32frombits(0x46802800), out: 0x7401}, // in f32=16404.000000, out f16=16400
	{in: math.Float32frombits(0x46802a00), out: 0x7401}, // in f32=16405.000000, out f16=16400
	{in: math.Float32frombits(0x46802c00), out: 0x7401}, // in f32=16406.000000, out f16=16400
	{in: math.Float32frombits(0x46802e00), out: 0x7401}, // in f32=16407.000000, out f16=16400
	{in: math.Float32frombits(0x46803000), out: 0x7402}, // in f32=16408.000000, out f16=16416
	{in: math.Float32frombits(0x46ffee00), out: 0x77ff}, // in f32=32759.000000, out f16=32752
	{in: math.Float32frombits(0x46fff000), out: 0x7800}, // in f32=32760.000000, out f16=32768
	{in: math.Float32frombits(0x46fff200), out: 0x7800}, // in f32=32761.000000, out f16=32768
	{in: math.Float32frombits(0x46fff400), out: 0x7800}, // in f32=32762.000000, out f16=32768
	{in: math.Float32frombits(0x46fff600), out: 0x7800}, // in f32=32763.000000, out f16=32768
	{in: math.Float32frombits(0x46fff800), out: 0x7800}, // in f32=32764.000000, out f16=32768
	{in: math.Float32frombits(0x46fffa00), out: 0x7800}, // in f32=32765.000000, out f16=32768
	{in: math.Float32frombits(0x46fffc00), out: 0x7800}, // in f32=32766.000000, out f16=32768
	{in: math.Float32frombits(0x46fffe00), out: 0x7800}, // in f32=32767.000000, out f16=32768
	{in: math.Float32frombits(0x47000000), out: 0x7800}, // in f32=32768.000000, out f16=32768
	{in: math.Float32frombits(0x47000100), out: 0x7800}, // in f32=32769.000000, out f16=32768
	{in: math.Float32frombits(0x47000200), out: 0x7800}, // in f32=32770.000000, out f16=32768
	{in: math.Float32frombits(0x47000300), out: 0x7800}, // in f32=32771.000000, out f16=32768
	{in: math.Float32frombits(0x47000400), out: 0x7800}, // in f32=32772.000000, out f16=32768
	{in: math.Float32frombits(0x47000500), out: 0x7800}, // in f32=32773.000000, out f16=32768
	{in: math.Float32frombits(0x47000600), out: 0x7800}, // in f32=32774.000000, out f16=32768
	{in: math.Float32frombits(0x47000700), out: 0x7800}, // in f32=32775.000000, out f16=32768
	{in: math.Float32frombits(0x47000800), out: 0x7800}, // in f32=32776.000000, out f16=32768
	{in: math.Float32frombits(0x47000900), out: 0x7800}, // in f32=32777.000000, out f16=32768
	{in: math.Float32frombits(0x47000a00), out: 0x7800}, // in f32=32778.000000, out f16=32768
	{in: math.Float32frombits(0x47000b00), out: 0x7800}, // in f32=32779.000000, out f16=32768
	{in: math.Float32frombits(0x47000c00), out: 0x7800}, // in f32=32780.000000, out f16=32768
	{in: math.Float32frombits(0x47000d00), out: 0x7800}, // in f32=32781.000000, out f16=32768
	{in: math.Float32frombits(0x47000e00), out: 0x7800}, // in f32=32782.000000, out f16=32768
	{in: math.Float32frombits(0x47000f00), out: 0x7800}, // in f32=32783.000000, out f16=32768
	{in: math.Float32frombits(0x47001000), out: 0x7800}, // in f32=32784.000000, out f16=32768
	{in: math.Float32frombits(0x47001100), out: 0x7801}, // in f32=32785.000000, out f16=32800
	{in: math.Float32frombits(0x47001200), out: 0x7801}, // in f32=32786.000000, out f16=32800
	{in: math.Float32frombits(0x47001300), out: 0x7801}, // in f32=32787.000000, out f16=32800
	{in: math.Float32frombits(0x47001400), out: 0x7801}, // in f32=32788.000000, out f16=32800
	{in: math.Float32frombits(0x47001500), out: 0x7801}, // in f32=32789.000000, out f16=32800
	{in: math.Float32frombits(0x47001600), out: 0x7801}, // in f32=32790.000000, out f16=32800
	{in: math.Float32frombits(0x47001700), out: 0x7801}, // in f32=32791.000000, out f16=32800
	{in: math.Float32frombits(0x47001800), out: 0x7801}, // in f32=32792.000000, out f16=32800
	{in: math.Float32frombits(0x47001900), out: 0x7801}, // in f32=32793.000000, out f16=32800
	{in: math.Float32frombits(0x47001a00), out: 0x7801}, // in f32=32794.000000, out f16=32800
	{in: math.Float32frombits(0x47001b00), out: 0x7801}, // in f32=32795.000000, out f16=32800
	{in: math.Float32frombits(0x47001c00), out: 0x7801}, // in f32=32796.000000, out f16=32800
	{in: math.Float32frombits(0x47001d00), out: 0x7801}, // in f32=32797.000000, out f16=32800
	{in: math.Float32frombits(0x47001e00), out: 0x7801}, // in f32=32798.000000, out f16=32800
	{in: math.Float32frombits(0x47001f00), out: 0x7801}, // in f32=32799.000000, out f16=32800
	{in: math.Float32frombits(0x47002000), out: 0x7801}, // in f32=32800.000000, out f16=32800
	{in: math.Float32frombits(0x47002100), out: 0x7801}, // in f32=32801.000000, out f16=32800
	{in: math.Float32frombits(0x47002200), out: 0x7801}, // in f32=32802.000000, out f16=32800
	{in: math.Float32frombits(0x47002300), out: 0x7801}, // in f32=32803.000000, out f16=32800
	{in: math.Float32frombits(0x47002400), out: 0x7801}, // in f32=32804.000000, out f16=32800
	{in: math.Float32frombits(0x47002500), out: 0x7801}, // in f32=32805.000000, out f16=32800
	{in: math.Float32frombits(0x47002600), out: 0x7801}, // in f32=32806.000000, out f16=32800
	{in: math.Float32frombits(0x47002700), out: 0x7801}, // in f32=32807.000000, out f16=32800
	{in: math.Float32frombits(0x47002800), out: 0x7801}, // in f32=32808.000000, out f16=32800
	{in: math.Float32frombits(0x47002900), out: 0x7801}, // in f32=32809.000000, out f16=32800
	{in: math.Float32frombits(0x47002a00), out: 0x7801}, // in f32=32810.000000, out f16=32800
	{in: math.Float32frombits(0x47002b00), out: 0x7801}, // in f32=32811.000000, out f16=32800
	{in: math.Float32frombits(0x47002c00), out: 0x7801}, // in f32=32812.000000, out f16=32800
	{in: math.Float32frombits(0x47002d00), out: 0x7801}, // in f32=32813.000000, out f16=32800
	{in: math.Float32frombits(0x47002e00), out: 0x7801}, // in f32=32814.000000, out f16=32800
	{in: math.Float32frombits(0x47002f00), out: 0x7801}, // in f32=32815.000000, out f16=32800
	{in: math.Float32frombits(0x47003000), out: 0x7802}, // in f32=32816.000000, out f16=32832
	{in: math.Float32frombits(0x477fe500), out: 0x7bff}, // in f32=65509.000000, out f16=65504
	{in: math.Float32frombits(0x477fe100), out: 0x7bff}, // in f32=65505.000000, out f16=65504
	{in: math.Float32frombits(0x477fee00), out: 0x7bff}, // in f32=65518.000000, out f16=65504
	{in: math.Float32frombits(0x477fef00), out: 0x7bff}, // in f32=65519.000000, out f16=65504
	{in: math.Float32frombits(0x477feffd), out: 0x7bff}, // in f32=65519.988281, out f16=65504
	{in: math.Float32frombits(0x477ff000), out: 0x7c00}, // in f32=65520.000000, out f16=+Inf
}

func TestPrecisionFromfloat32(t *testing.T) {
	for i, v := range wantF32toF16bits {
		f16 := float16.Fromfloat32(v.in)
		u16 := uint16(f16)

		if u16 != v.out {
			t.Errorf("i=%d, in f32bits=0x%08x, wanted=0x%04x, got=0x%04x.", i, math.Float32bits(v.in), v.out, u16)
		}

		checkPrecision(t, v.in, f16, uint64(i))
	}

	f32 := float32(5.5) // value that doesn't drop any bits in the significand, is within normal exponent range
	pre := float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionExact {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionExact (%d), got=%d.", math.Float32bits(f32), float16.PrecisionExact, pre)
	}

	f32 = math.Float32frombits(0x38000000) // subnormal value with coef = 0 that can round-trip float32->float16->float32
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionUnknown {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionUnknown (%d), got=%d.", math.Float32bits(f32), float16.PrecisionUnknown, pre)
	}

	f32 = math.Float32frombits(0x387fc000) // subnormal value with coef !=0 that can round-trip float32->float16->float32
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionUnknown {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionUnknown (%d), got=%d.", math.Float32bits(f32), float16.PrecisionUnknown, pre)
	}

	f32 = math.Float32frombits(0x33c00000) // subnormal value with no dropped bits that cannot round-trip float32->float16->float32
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionUnknown {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionUnknown (%d), got=%d.", math.Float32bits(f32), float16.PrecisionUnknown, pre)
	}

	f32 = math.Float32frombits(0x38000001) // subnormal value with dropped non-zero bits > 0
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionInexact {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionInexact (%d), got=%d.", math.Float32bits(f32), float16.PrecisionInexact, pre)
	}

	f32 = float32(math.Pi) // value that cannot "preserve value" because it drops bits in the significand
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionInexact {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionInexact (%d), got=%d.", math.Float32bits(f32), float16.PrecisionInexact, pre)
	}

	f32 = math.Float32frombits(0x1) // value that will underflow
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionUnderflow {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionUnderflow (%d), got=%d.", math.Float32bits(f32), float16.PrecisionUnderflow, pre)
	}

	f32 = math.Float32frombits(0x33000000) // value that will underflow
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionUnderflow {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionUnderflow (%d), got=%d.", math.Float32bits(f32), float16.PrecisionUnderflow, pre)
	}

	f32 = math.Float32frombits(0x47800000) // value that will overflow
	pre = float16.PrecisionFromfloat32(f32)
	if pre != float16.PrecisionOverflow {
		t.Errorf("f32bits=0x%08x, wanted=PrecisionOverflow (%d), got=%d.", math.Float32bits(f32), float16.PrecisionOverflow, pre)
	}

}

func TestFromNaN32ps(t *testing.T) {
	for i, v := range wantF32toF16bits {
		f16 := float16.Fromfloat32(v.in)
		u16 := uint16(f16)

		if u16 != v.out {
			t.Errorf("i=%d, in f32bits=0x%08x, wanted=0x%04x, got=0x%04x.", i, math.Float32bits(v.in), v.out, u16)
		}

		checkFromNaN32ps(t, v.in, f16)
	}

	// since checkFromNaN32ps rejects non-NaN input, try one here
	nan, err := float16.FromNaN32ps(float32(math.Pi))
	if err != float16.ErrInvalidNaNValue {
		t.Errorf("FromNaN32ps: in float32(math.Pi) wanted err float16.ErrInvalidNaNValue, got err = %q", err)
	}
	if err.Error() != "float16: invalid NaN value, expected IEEE 754 NaN" {
		t.Errorf("unexpected string value returned by err.Error() for ErrInvalidNaNValue: %s", err.Error())
	}
	if uint16(nan) != 0x7c01 { // signalling NaN
		t.Errorf("FromNaN32ps: in float32(math.Pi) wanted nan = 0x7c01, got nan = 0x%04x", uint16(nan))
	}

}

// Test a small subset of possible conversions from float32 to Float16.
// TestSomeFromFloat32 runs in under 1 second while TestAllFromFloat32 takes about 45 seconds.
func TestSomeFromFloat32(t *testing.T) {

	for i, v := range wantF32toF16bits {
		f16 := float16.Fromfloat32(v.in)
		u16 := uint16(f16)

		if u16 != v.out {
			t.Errorf("i=%d, in f32bits=0x%08x, wanted=0x%04x, got=0x%04x.", i, math.Float32bits(v.in), v.out, u16)
		}
	}
}

// Test all possible 4294967296 float32 input values and results for
// Fromfloat32(), FromNaN32ps(), and PrecisionFromfloat32().
func TestAllFromFloat32(t *testing.T) {

	if testing.Short() {
		t.Skip("skipping TestAllFromFloat32 in short mode.")
	}

	fmt.Printf("WARNING: TestAllFromFloat32 should take about 1-2 minutes to run on amd64, other platforms may take longer...\n")

	//const wantBlake2b = "3f310bc5608a087462d361644fe66feeb4c68145f6f18eb6f1439cd7914888b6df9e30ae5350dce0635162cc6a2f23b31b3e4353ca132a3c552bdbd58baa54e6"
	const wantSHA512 = "08670429a475164d6c4a080969e35231c77ef7069b430b5f38af22e013796b7818bbe8f5942a6ddf26de0e1dfc67d02243f483d85729ebc3762fc2948a5ca1f8"

	const batchSize uint32 = 16384
	results := make([]uint16, batchSize)
	buf := new(bytes.Buffer)
	h := sha512.New()

	for i := uint64(0); i < uint64(0xFFFFFFFF); i += uint64(batchSize) {
		// fill results
		for j := uint32(0); j < batchSize; j++ {
			inF32 := math.Float32frombits(uint32(i) + uint32(j))
			f16 := float16.Fromfloat32(inF32)
			results[j] = uint16(f16)
			checkPrecision(t, inF32, f16, i)
			checkFromNaN32ps(t, inF32, f16)
		}

		// convert results to []byte
		err := binary.Write(buf, binary.LittleEndian, results)
		if err != nil {
			panic(err)
		}

		// update hash with []byte of results
		h.Write(buf.Bytes())

		buf.Reset()
	}

	// display hash digest in hex
	digest := h.Sum(nil)
	gotSHA512hex := hex.EncodeToString(digest)
	if gotSHA512hex != wantSHA512 {
		t.Errorf("gotSHA512hex = %s", gotSHA512hex)
	}
}

// Test all 65536 conversions from float16 to float32.
// TestAllToFloat32 runs in under 1 second.
func TestAllToFloat32(t *testing.T) {
	//const wantBlake2b = "078d8e3fac9480de1493f22c8f9bfc1eb2051537c536f00f621557d70eed1af057a487c3e252f6d593769f5288d5ab66d8e9cd1adba359838802944bdb731f4d"
	const wantSHA512 = "1a4ccec9fd7b6e83310c6b4958a25778cd95f8d4f88b19950e4b8d6932a955f7fbd96b1c9bd9b2a79c3a9d34d653f55e671f8f86e6a5a876660cd38479001aa6"
	const batchSize uint32 = 16384
	results := make([]float32, batchSize)
	buf := new(bytes.Buffer)
	h := sha512.New()

	for i := uint64(0); i < uint64(0xFFFF); i += uint64(batchSize) {
		// fill results
		for j := uint32(0); j < batchSize; j++ {
			inU16 := uint16((uint16(i) + uint16(j)))
			f16 := float16.Float16(inU16)
			results[j] = f16.Float32()
		}

		// convert results to []byte
		err := binary.Write(buf, binary.LittleEndian, results)
		if err != nil {
			panic(err)
		}

		// update hash with []byte of results
		h.Write(buf.Bytes())

		buf.Reset()
	}

	// display hash digest in hex
	digest := h.Sum(nil)
	gotSHA512hex := hex.EncodeToString(digest)
	if gotSHA512hex != wantSHA512 {
		t.Errorf("Float16toFloat32: gotSHA512hex = %s", gotSHA512hex)
	}

}

func TestFrombits(t *testing.T) {
	x := uint16(0x1234)
	f16 := float16.Frombits(x)
	if uint16(f16) != f16.Bits() || uint16(f16) != x {
		t.Errorf("float16.Frombits(0x7fff) returned %04x, wanted %04x", uint16(f16), x)
	}
}

func TestNaN(t *testing.T) {
	nan := float16.NaN()
	if !nan.IsNaN() {
		t.Errorf("nan.IsNaN() returned false, wanted true")
	}
}

func TestInf(t *testing.T) {
	posInf := float16.Inf(0)
	if uint16(posInf) != 0x7c00 {
		t.Errorf("float16.Inf(0) returned %04x, wanted %04x", uint16(posInf), 0x7c00)
	}

	posInf = float16.Inf(1)
	if uint16(posInf) != 0x7c00 {
		t.Errorf("float16.Inf(1) returned %04x, wanted %04x", uint16(posInf), 0x7c00)
	}

	negInf := float16.Inf(-1)
	if uint16(negInf) != 0xfc00 {
		t.Errorf("float16.Inf(-1) returned %04x, wanted %04x", uint16(negInf), 0xfc00)
	}
}

func TestBits(t *testing.T) {
	x := uint16(0x1234)
	f16 := float16.Frombits(x)
	if uint16(f16) != f16.Bits() || f16.Bits() != x {
		t.Errorf("Bits() returned %04x, wanted %04x", uint16(f16), x)
	}
}

func TestIsFinite(t *testing.T) {
	// IsFinite returns true if f is neither infinite nor NaN.

	finite := float16.Fromfloat32(float32(1.5))
	if !finite.IsFinite() {
		t.Errorf("finite.Infinite() returned false, wanted true")
	}

	posInf := float16.Inf(0)
	if posInf.IsFinite() {
		t.Errorf("posInf.Infinite() returned true, wanted false")
	}

	negInf := float16.Inf(-1)
	if negInf.IsFinite() {
		t.Errorf("negInf.Infinite() returned true, wanted false")
	}

	nan := float16.NaN()
	if nan.IsFinite() {
		t.Errorf("nan.Infinite() returned true, wanted false")
	}
}

func TestIsNaN(t *testing.T) {

	f16 := float16.Float16(0)
	if f16.IsNaN() {
		t.Errorf("Float16(0).IsNaN() returned true, wanted false")
	}

	f16 = float16.Float16(0x7e00)
	if !f16.IsNaN() {
		t.Errorf("Float16(0x7e00).IsNaN() returned false, wanted true")
	}
}

func TestIsQuietNaN(t *testing.T) {

	f16 := float16.Float16(0)
	if f16.IsQuietNaN() {
		t.Errorf("Float16(0).IsQuietNaN() returned true, wanted false")
	}

	f16 = float16.Float16(0x7e00)
	if !f16.IsQuietNaN() {
		t.Errorf("Float16(0x7e00).IsQuietNaN() returned false, wanted true")
	}

	f16 = float16.Float16(0x7e00 ^ 0x0200)
	if f16.IsQuietNaN() {
		t.Errorf("Float16(0x7e00 ^ 0x0200).IsQuietNaN() returned true, wanted false")
	}
}

func TestIsNormal(t *testing.T) {
	// IsNormal returns true if f is neither zero, infinite, subnormal, or NaN.

	zero := float16.Frombits(0)
	if zero.IsNormal() {
		t.Errorf("zero.IsNormal() returned true, wanted false")
	}

	posInf := float16.Inf(0)
	if posInf.IsNormal() {
		t.Errorf("posInf.IsNormal() returned true, wanted false")
	}

	negInf := float16.Inf(-1)
	if negInf.IsNormal() {
		t.Errorf("negInf.IsNormal() returned true, wanted false")
	}

	nan := float16.NaN()
	if nan.IsNormal() {
		t.Errorf("nan.IsNormal() returned true, wanted false")
	}

	subnormal := float16.Frombits(0x0001)
	if subnormal.IsNormal() {
		t.Errorf("subnormal.IsNormal() returned true, wanted false")
	}

	normal := float16.Fromfloat32(float32(1.5))
	if !normal.IsNormal() {
		t.Errorf("normal.IsNormal() returned false, wanted true")
	}

}

func TestSignbit(t *testing.T) {

	f16 := float16.Fromfloat32(float32(0.0))
	if f16.Signbit() {
		t.Errorf("float16.Fromfloat32(float32(0)).Signbit() returned true, wanted false")
	}

	f16 = float16.Fromfloat32(float32(2.0))
	if f16.Signbit() {
		t.Errorf("float16.Fromfloat32(float32(2)).Signbit() returned true, wanted false")
	}

	f16 = float16.Fromfloat32(float32(-2.0))
	if !f16.Signbit() {
		t.Errorf("float16.Fromfloat32(float32(-2)).Signbit() returned false, wanted true")
	}

}

func TestString(t *testing.T) {
	f16 := float16.Fromfloat32(1.5)
	s := f16.String()
	if s != "1.5" {
		t.Errorf("Float16(1.5).String() returned %s, wanted 1.5", s)
	}

	f16 = float16.Fromfloat32(3.141593)
	s = f16.String()
	if s != "3.140625" {
		t.Errorf("Float16(3.141593).String() returned %s, wanted 3.140625", s)
	}

}

func TestIsInf(t *testing.T) {

	f16 := float16.Float16(0)
	if f16.IsInf(0) {
		t.Errorf("Float16(0).IsInf(0) returned true, wanted false")
	}

	f16 = float16.Float16(0x7c00)
	if !f16.IsInf(0) {
		t.Errorf("Float16(0x7c00).IsInf(0) returned false, wanted true")
	}

	f16 = float16.Float16(0x7c00)
	if !f16.IsInf(1) {
		t.Errorf("Float16(0x7c00).IsInf(1) returned false, wanted true")
	}

	f16 = float16.Float16(0x7c00)
	if f16.IsInf(-1) {
		t.Errorf("Float16(0x7c00).IsInf(-1) returned true, wanted false")
	}

	f16 = float16.Float16(0xfc00)
	if !f16.IsInf(0) {
		t.Errorf("Float16(0xfc00).IsInf(0) returned false, wanted true")
	}

	f16 = float16.Float16(0xfc00)
	if f16.IsInf(1) {
		t.Errorf("Float16(0xfc00).IsInf(1) returned true, wanted false")
	}

	f16 = float16.Float16(0xfc00)
	if !f16.IsInf(-1) {
		t.Errorf("Float16(0xfc00).IsInf(-1) returned false, wanted true")
	}
}

func float32parts(f32 float32) (exp int32, coef uint32, dropped uint32) {
	const COEFMASK uint32 = 0x7fffff // 23 least significant bits
	const EXPSHIFT uint32 = 23
	const EXPBIAS uint32 = 127
	const EXPMASK uint32 = uint32(0xff) << EXPSHIFT
	const DROPMASK uint32 = COEFMASK >> 10
	u32 := math.Float32bits(f32)
	exp = int32(((u32 & EXPMASK) >> EXPSHIFT) - EXPBIAS)
	coef = u32 & COEFMASK
	dropped = coef & DROPMASK
	return exp, coef, dropped
}

func isNaN32(f32 float32) bool {
	exp, coef, _ := float32parts(f32)
	return (exp == 128) && (coef != 0)
}

func isQuietNaN32(f32 float32) bool {
	exp, coef, _ := float32parts(f32)
	return (exp == 128) && (coef != 0) && ((coef & 0x00400000) != 0)
}

func checkFromNaN32ps(t *testing.T, f32 float32, f16 float16.Float16) {

	if !isNaN32(f32) {
		return
	}

	u32 := math.Float32bits(f32)
	nan16, err := float16.FromNaN32ps(f32)

	if isQuietNaN32(f32) {
		// result should be the same
		if err != nil {
			t.Errorf("FromNaN32ps: qnan = 0x%08x (%f) wanted err = nil, got err = %q", u32, f32, err)
		}
		if uint16(nan16) != uint16(f16) {
			t.Errorf("FromNaN32ps: qnan = 0x%08x (%f) wanted nan16 = %v, got nan16 = %v", u32, f32, f16, nan16)
		}
	} else {
		// result should differ only by the signaling/quiet bit unless payload is empty
		if err != nil {
			t.Errorf("FromNaN32ps: snan = 0x%08x (%f) wanted err = nil, got err = %q", u32, f32, err)
		}

		coef := uint16(f16) & uint16(0x03ff)
		payload := uint16(f16) & uint16(0x01ff)
		diff := uint16(nan16 ^ f16)

		if payload == 0 {
			// the lowest bit needed to be set to prevent turning sNaN into infinity, so 2 bits differ
			if diff != 0x0201 {
				t.Errorf("FromNaN32ps: snan = 0x%08x (%f) wanted diff == 0x0201, got 0x%04x", u32, f32, diff)
			}
		} else {
			// only the quiet bit was restored, so 1 bit differs
			if diff != 0x0200 {
				t.Errorf("FromNaN32ps: snan = 0x%08x (%f) wanted diff == 0x0200, got 0x%04x. f16=0x%04x n16=0x%04x coef=0x%04x", u32, f32, diff, uint16(f16), uint16(nan16), coef)
			}
		}
	}
}

func checkPrecision(t *testing.T, f32 float32, f16 float16.Float16, i uint64) {
	//TODO rewrite this test when time allows

	u32 := math.Float32bits(f32)
	u16 := f16.Bits()
	f32bis := f16.Float32()
	u32bis := math.Float32bits(f32bis)
	pre := float16.PrecisionFromfloat32(f32)
	roundtripped := u32 == u32bis
	exp32, coef32, dropped32 := float32parts(f32)

	if roundtripped {
		checkRoundTrippedPrecision(t, u32, u16, u32bis, exp32, coef32, dropped32)
		return
	}

	if pre == float16.PrecisionExact {
		// this should only happen if both input and output are NaN
		if !(f16.IsNaN() && isNaN32(f32)) {
			t.Errorf("i=%d, PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionExact when roundtrip failed with non-special value", i, u32, f32, u16, u32bis, f32bis)
		}

	} else if pre == float16.PrecisionUnknown {
		if exp32 < -24 {
			t.Errorf("i=%d, PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionUnknown, wanted PrecisionUnderflow", i, u32, f32, u16, u32bis, f32bis)
		}
		if dropped32 != 0 {
			t.Errorf("i=%d, PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionUnknown, wanted PrecisionInexact", i, u32, f32, u16, u32bis, f32bis)
		}
	} else if pre == float16.PrecisionInexact {
		checkPrecisionInexact(t, u32, u16, u32bis, exp32, coef32, dropped32)
	} else if pre == float16.PrecisionUnderflow {
		if exp32 >= -14 {
			t.Errorf("i=%d, PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionUnderflow when exp32 is >= -14", i, u32, f32, u16, u32bis, f32bis)
		}
	} else if pre == float16.PrecisionOverflow {
		if exp32 <= 15 {
			t.Errorf("i=%d, PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionOverflow when exp32 is <= 15", i, u32, f32, u16, u32bis, f32bis)
		}
	}
}

func checkPrecisionInexact(t *testing.T, u32 uint32, u16 uint16, u32bis uint32, exp32 int32, coef32 uint32, dropped32 uint32) {
	f32 := math.Float32frombits(u32)
	f32bis := math.Float32frombits(u32bis)

	if exp32 < -24 {
		t.Errorf("PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionInexact, wanted PrecisionUnderflow", u32, f32, u16, u32bis, f32bis)
	}
	if exp32 > 15 {
		t.Errorf("PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionInexact, wanted PrecisionOverflow", u32, f32, u16, u32bis, f32bis)
	}
	if coef32 == 0 {
		t.Errorf("PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionInexact when coef32 is 0", u32, f32, u16, u32bis, f32bis)
	}
	if dropped32 == 0 {
		t.Errorf("PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), got PrecisionInexact when dropped32 is 0", u32, f32, u16, u32bis, f32bis)
	}
}

func checkRoundTrippedPrecision(t *testing.T, u32 uint32, u16 uint16, u32bis uint32, exp32 int32, coef32 uint32, dropped32 uint32) {
	f32 := math.Float32frombits(u32)
	f32bis := math.Float32frombits(u32bis)
	pre := float16.PrecisionFromfloat32(f32)
	f16 := float16.Frombits(u16)

	if dropped32 != 0 {
		t.Errorf("PrecisionFromfloat32 in f32bits=0x%08x (%f), out f16bits=0x%04x, back=0x%08x (%f), dropped32 != 0 with successful roundtrip", u32, f32, u16, u32bis, f32bis)
	}

	if pre != float16.PrecisionExact {
		// there are 2046 values that are subnormal and can round-trip float32->float16->float32
		if pre != float16.PrecisionUnknown {
			t.Errorf("PrecisionFromfloat32 in f32bits=0x%08x (%032b) (%f), out f16bits=0x%04x (%v), back=0x%08x (%f), got %v, wanted PrecisionExact, exp=%d, coef=%d, drpd=%d", u32, u32, f32, u16, f16, u32bis, f32bis, pre, exp32, coef32, dropped32)
		}
	}

}
