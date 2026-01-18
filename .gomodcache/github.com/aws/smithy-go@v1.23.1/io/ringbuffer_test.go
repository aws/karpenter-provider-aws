package io

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestRingBuffer_Write(t *testing.T) {
	cases := map[string]struct {
		sliceCapacity         int
		input                 []byte
		expectedStart         int
		expectedEnd           int
		expectedSize          int
		expectedWrittenBuffer []byte
	}{
		"RingBuffer capacity matches Bytes written": {
			sliceCapacity:         11,
			input:                 []byte("hello world"),
			expectedStart:         0,
			expectedEnd:           11,
			expectedSize:          11,
			expectedWrittenBuffer: []byte("hello world"),
		},
		"RingBuffer capacity is lower than Bytes written": {
			sliceCapacity:         10,
			input:                 []byte("hello world"),
			expectedStart:         1,
			expectedEnd:           1,
			expectedSize:          10,
			expectedWrittenBuffer: []byte("dello worl"),
		},
		"RingBuffer capacity is more than Bytes written": {
			sliceCapacity:         12,
			input:                 []byte("hello world"),
			expectedStart:         0,
			expectedEnd:           11,
			expectedSize:          11,
			expectedWrittenBuffer: []byte("hello world"),
		},
		"No Bytes written": {
			sliceCapacity:         10,
			input:                 []byte(""),
			expectedStart:         0,
			expectedEnd:           0,
			expectedSize:          0,
			expectedWrittenBuffer: []byte(""),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			byteSlice := make([]byte, c.sliceCapacity)
			ringBuffer := NewRingBuffer(byteSlice)
			ringBuffer.Write(c.input)
			if e, a := c.expectedSize, ringBuffer.size; e != a {
				t.Errorf("expect default size to be %v , got %v", e, a)
			}
			if e, a := c.expectedStart, ringBuffer.start; e != a {
				t.Errorf("expect deafult start to point to %v , got %v", e, a)
			}
			if e, a := c.expectedEnd, ringBuffer.end; e != a {
				t.Errorf("expect default end to point to %v , got %v", e, a)
			}
			if e, a := c.expectedWrittenBuffer, ringBuffer.slice; !bytes.Contains(a, e) {
				t.Errorf("expect written bytes to be %v , got %v", string(e), string(a))
			}
		})
	}

}

func TestRingBuffer_Read(t *testing.T) {
	cases := map[string]struct {
		input                         []byte
		numberOfBytesToRead           int
		expectedStartAfterRead        int
		expectedEndAfterRead          int
		expectedSizeOfBufferAfterRead int
		expectedReadSlice             []byte
		expectedErrorAfterRead        error
	}{
		"Read capacity matches Bytes written": {
			input:                         []byte("Hello world"),
			numberOfBytesToRead:           11,
			expectedStartAfterRead:        11,
			expectedEndAfterRead:          11,
			expectedSizeOfBufferAfterRead: 0,
			expectedReadSlice:             []byte("Hello world"),
			expectedErrorAfterRead:        nil,
		},
		"Read capacity is lower than Bytes written": {
			input:                         []byte("hello world"),
			numberOfBytesToRead:           5,
			expectedStartAfterRead:        5,
			expectedEndAfterRead:          11,
			expectedSizeOfBufferAfterRead: 6,
			expectedReadSlice:             []byte("hello"),
			expectedErrorAfterRead:        nil,
		},
		"Read capacity is more than Bytes written": {
			input:                         []byte("hello world"),
			numberOfBytesToRead:           15,
			expectedStartAfterRead:        11,
			expectedEndAfterRead:          11,
			expectedSizeOfBufferAfterRead: 0,
			expectedReadSlice:             []byte("hello world"),
			expectedErrorAfterRead:        io.EOF,
		},
		"No Bytes are read": {
			input:                         []byte("hello world"),
			numberOfBytesToRead:           0,
			expectedStartAfterRead:        0,
			expectedEndAfterRead:          11,
			expectedSizeOfBufferAfterRead: 11,
			expectedReadSlice:             []byte(""),
			expectedErrorAfterRead:        nil,
		},
		"No Bytes written": {
			input:                         []byte(""),
			numberOfBytesToRead:           11,
			expectedStartAfterRead:        0,
			expectedEndAfterRead:          0,
			expectedSizeOfBufferAfterRead: 0,
			expectedReadSlice:             []byte(""),
			expectedErrorAfterRead:        io.EOF,
		},
		"RingBuffer capacity is more than Bytes Written": {
			input:                         []byte("h"),
			numberOfBytesToRead:           11,
			expectedStartAfterRead:        1,
			expectedEndAfterRead:          1,
			expectedSizeOfBufferAfterRead: 0,
			expectedReadSlice:             []byte("h"),
			expectedErrorAfterRead:        io.EOF,
		},
	}
	for name, c := range cases {
		byteSlice := make([]byte, 11)
		t.Run(name, func(t *testing.T) {
			ringBuffer := NewRingBuffer(byteSlice)
			readSlice := make([]byte, c.numberOfBytesToRead)

			ringBuffer.Write(c.input)
			_, err := ringBuffer.Read(readSlice)

			if e, a := c.expectedErrorAfterRead, err; e != a {
				t.Errorf("Expected %v, got %v", e, a)
			}
			if e, a := c.expectedReadSlice, readSlice; !bytes.Contains(a, e) {
				t.Errorf("expect read buffer to be %v, got %v", string(e), string(a))
			}
			if e, a := c.expectedSizeOfBufferAfterRead, ringBuffer.size; e != a {
				t.Errorf("expect default size to be %v , got %v", e, a)
			}
			if e, a := c.expectedStartAfterRead, ringBuffer.start; e != a {
				t.Errorf("expect default start to point to %v , got %v", e, a)
			}
			if e, a := c.expectedEndAfterRead, ringBuffer.end; e != a {
				t.Errorf("expect default end to point to %v , got %v", e, a)
			}
		})
	}
}

func TestRingBuffer_forConsecutiveReadWrites(t *testing.T) {
	cases := map[string]struct {
		input                         []string
		sliceCapacity                 int
		numberOfBytesToRead           []int
		expectedStartAfterRead        []int
		expectedEnd                   []int
		expectedSizeOfBufferAfterRead []int
		expectedReadSlice             []string
		expectedWrittenBuffer         []string
		expectedErrorAfterRead        []error
	}{
		"RingBuffer capacity matches Bytes written": {
			input:                         []string{"Hello World", "Hello Earth", "Mars,/"},
			sliceCapacity:                 11,
			numberOfBytesToRead:           []int{5, 11},
			expectedStartAfterRead:        []int{5, 6},
			expectedEnd:                   []int{11, 6},
			expectedSizeOfBufferAfterRead: []int{6, 0},
			expectedReadSlice:             []string{"Hello", "EarthMars,/"},
			expectedWrittenBuffer:         []string{"Hello World", "Hello Earth", "Mars,/Earth"},
			expectedErrorAfterRead:        []error{nil, nil},
		},
		"RingBuffer capacity is lower than Bytes written": {
			input:                         []string{"Hello World", "Hello Earth", "Mars,/"},
			sliceCapacity:                 5,
			numberOfBytesToRead:           []int{5, 5},
			expectedStartAfterRead:        []int{1, 3},
			expectedEnd:                   []int{1, 3},
			expectedSizeOfBufferAfterRead: []int{0, 0},
			expectedReadSlice:             []string{"World", "ars,/"},
			expectedWrittenBuffer:         []string{"dWorl", "thEar", "s,/ar"},
			expectedErrorAfterRead:        []error{nil, nil},
		},
		"RingBuffer capacity is more than Bytes written": {
			input:                         []string{"Hello World", "Hello Earth", "Mars,/"},
			sliceCapacity:                 15,
			numberOfBytesToRead:           []int{5, 8},
			expectedStartAfterRead:        []int{5, 6},
			expectedEnd:                   []int{11, 13},
			expectedSizeOfBufferAfterRead: []int{6, 7},
			expectedReadSlice:             []string{"Hello", "llo Eart"},
			expectedWrittenBuffer:         []string{"Hello World", "o EarthorldHell", "o EarthMars,/ll"},
			expectedErrorAfterRead:        []error{nil, nil},
		},
		"No Bytes written": {
			input:                         []string{"", "", ""},
			sliceCapacity:                 11,
			numberOfBytesToRead:           []int{5, 8},
			expectedStartAfterRead:        []int{0, 0},
			expectedEnd:                   []int{0, 0},
			expectedSizeOfBufferAfterRead: []int{0, 0},
			expectedReadSlice:             []string{"", ""},
			expectedWrittenBuffer:         []string{"", "", ""},
			expectedErrorAfterRead:        []error{io.EOF, io.EOF},
		},
	}
	for name, c := range cases {
		writeSlice := make([]byte, c.sliceCapacity)
		ringBuffer := NewRingBuffer(writeSlice)

		t.Run(name, func(t *testing.T) {
			ringBuffer.Write([]byte(c.input[0]))
			if e, a := c.expectedWrittenBuffer[0], string(ringBuffer.slice); !strings.Contains(a, e) {
				t.Errorf("Expected %v, got %v", e, a)
			}

			readSlice := make([]byte, c.numberOfBytesToRead[0])
			readCount, err := ringBuffer.Read(readSlice)

			if e, a := c.expectedErrorAfterRead[0], err; e != a {
				t.Errorf("Expected %v, got %v", e, a)
			}
			if e, a := len(c.expectedReadSlice[0]), readCount; e != a {
				t.Errorf("Expected to read %v bytes, read only %v", e, a)
			}
			if e, a := c.expectedReadSlice[0], string(readSlice); !strings.Contains(a, e) {
				t.Errorf("expect read buffer to be %v, got %v", e, a)
			}
			if e, a := c.expectedSizeOfBufferAfterRead[0], ringBuffer.size; e != a {
				t.Errorf("expect buffer size to be %v , got %v", e, a)
			}
			if e, a := c.expectedStartAfterRead[0], ringBuffer.start; e != a {
				t.Errorf("expect default start to point to %v , got %v", e, a)
			}
			if e, a := c.expectedEnd[0], ringBuffer.end; e != a {
				t.Errorf("expect default end tp point to %v , got %v", e, a)
			}

			/*
				Next cycle of read writes.
			*/
			ringBuffer.Write([]byte(c.input[1]))
			if e, a := c.expectedWrittenBuffer[1], string(ringBuffer.slice); !strings.Contains(a, e) {
				t.Errorf("Expected %v, got %v", e, a)
			}

			ringBuffer.Write([]byte(c.input[2]))
			if e, a := c.expectedWrittenBuffer[2], string(ringBuffer.slice); !strings.Contains(a, e) {
				t.Errorf("Expected %v, got %v", e, a)
			}

			readSlice = make([]byte, c.numberOfBytesToRead[1])
			readCount, err = ringBuffer.Read(readSlice)
			if e, a := c.expectedErrorAfterRead[1], err; e != a {
				t.Errorf("Expected %v, got %v", e, a)
			}
			if e, a := len(c.expectedReadSlice[1]), readCount; e != a {
				t.Errorf("Expected to read %v bytes, read only %v", e, a)
			}
			if e, a := c.expectedReadSlice[1], string(readSlice); !strings.Contains(a, e) {
				t.Errorf("expect read buffer to be %v, got %v", e, a)
			}
			if e, a := c.expectedSizeOfBufferAfterRead[1], ringBuffer.size; e != a {
				t.Errorf("expect buffer size to be %v , got %v", e, a)
			}
			if e, a := c.expectedStartAfterRead[1], ringBuffer.start; e != a {
				t.Errorf("expect default start to point to %v , got %v", e, a)
			}
			if e, a := c.expectedEnd[1], ringBuffer.end; e != a {
				t.Errorf("expect default end to point to %v , got %v", e, a)
			}
		})
	}
}

func TestRingBuffer_ExhaustiveRead(t *testing.T) {
	slice := make([]byte, 5)
	buf := NewRingBuffer(slice)
	buf.Write([]byte("Hello"))

	readSlice := make([]byte, 5)
	readCount, err := buf.Read(readSlice)
	if e, a := error(nil), err; e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
	if e, a := 5, readCount; e != a {
		t.Errorf("Expected to read %v bytes, read only %v", e, a)
	}
	if e, a := "Hello", string(readSlice); e != a {
		t.Errorf("Expected %v to be read, got %v", e, a)
	}

	readCount, err = buf.Read(readSlice)
	if e, a := io.EOF, err; e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
	if e, a := 0, readCount; e != a {
		t.Errorf("Expected to read %v bytes, read only %v", e, a)
	}
	if e, a := 0, buf.size; e != a {
		t.Errorf("Expected ring buffer size to be %v, got %v", e, a)
	}
}

func TestRingBuffer_Reset(t *testing.T) {
	byteSlice := make([]byte, 10)
	ringBuffer := NewRingBuffer(byteSlice)

	ringBuffer.Write([]byte("Hello-world"))
	if ringBuffer.size == 0 {
		t.Errorf("expected ringBuffer to not be empty")
	}

	readBuffer := make([]byte, 5)
	ringBuffer.Read(readBuffer)
	if ringBuffer.size == 0 {
		t.Errorf("expected ringBuffer to not be empty")
	}
	if e, a := "ello-", string(readBuffer); !strings.EqualFold(e, a) {
		t.Errorf("expected read string to be %s, got %s", e, a)
	}

	// reset the buffer
	ringBuffer.Reset()
	if e, a := 0, ringBuffer.size; e != a {
		t.Errorf("expect default size to be %v , got %v", e, a)
	}
	if e, a := 0, ringBuffer.start; e != a {
		t.Errorf("expect deafult start to point to %v , got %v", e, a)
	}
	if e, a := 0, ringBuffer.end; e != a {
		t.Errorf("expect default end to point to %v , got %v", e, a)
	}
	if e, a := 10, len(ringBuffer.slice); e != a {
		t.Errorf("expect ringBuffer capacity to be %v, got %v", e, a)
	}

	ringBuffer.Write([]byte("someThing new"))
	if ringBuffer.size == 0 {
		t.Errorf("expected ringBuffer to not be empty")
	}

	ringBuffer.Read(readBuffer)
	if ringBuffer.size == 0 {
		t.Errorf("expected ringBuffer to not be empty")
	}

	// Here the ringBuffer length is 10; while written string is "someThing new";
	// The initial characters are thus overwritten by the ringbuffer.
	// Thus the ring Buffer if completely read will have "eThing new".
	// Here readBuffer size is 5; thus first 5 character "eThin" is read.
	if e, a := "eThin", string(readBuffer); !strings.EqualFold(e, a) {
		t.Errorf("expected read string to be %s, got %s", e, a)
	}

	// reset the buffer
	ringBuffer.Reset()
	if e, a := 0, ringBuffer.size; e != a {
		t.Errorf("expect default size to be %v , got %v", e, a)
	}
	if e, a := 0, ringBuffer.start; e != a {
		t.Errorf("expect deafult start to point to %v , got %v", e, a)
	}
	if e, a := 0, ringBuffer.end; e != a {
		t.Errorf("expect default end to point to %v , got %v", e, a)
	}
	if e, a := 10, len(ringBuffer.slice); e != a {
		t.Errorf("expect ringBuffer capacity to be %v, got %v", e, a)
	}

	// reading reset ring buffer
	readCount, _ := ringBuffer.Read(readBuffer)
	if ringBuffer.size != 0 {
		t.Errorf("expected ringBuffer to be empty")
	}
	if e, a := 0, readCount; e != a {
		t.Errorf("expected read string to be of length %v, got %v", e, a)
	}
}

func TestRingBufferWriteRead(t *testing.T) {
	cases := []struct {
		Input      []byte
		BufferSize int
		Expected   []byte
	}{
		{
			Input: func() []byte {
				return []byte(`hello world!`)
			}(),
			BufferSize: 6,
			Expected:   []byte(`world!`),
		},
		{
			Input: func() []byte {
				return []byte(`hello world!`)
			}(),
			BufferSize: 12,
			Expected:   []byte(`hello world!`),
		},
		{
			Input: func() []byte {
				return []byte(`hello`)
			}(),
			BufferSize: 6,
			Expected:   []byte(`hello`),
		},
		{
			Input: func() []byte {
				return []byte(`hello!!`)
			}(),
			BufferSize: 6,
			Expected:   []byte(`ello!!`),
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dataReader := bytes.NewReader(tt.Input)

			ringBuffer := NewRingBuffer(make([]byte, tt.BufferSize))

			n, err := io.Copy(ringBuffer, dataReader)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
				return
			}

			if e, a := int64(len(tt.Input)), n; e != a {
				t.Errorf("expect %v, got %v", e, a)
			}

			actual, err := io.ReadAll(ringBuffer)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
				return
			}

			if string(tt.Expected) != string(actual) {
				t.Errorf("%v != %v", string(tt.Expected), string(actual))
				return
			}
		})
	}
}
