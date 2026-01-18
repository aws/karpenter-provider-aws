package gbytes_test

import (
	"io"
	"time"

	. "github.com/onsi/gomega/gbytes"

	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type SlowReader struct {
	R io.Reader
	D time.Duration
}

func (s SlowReader) Read(p []byte) (int, error) {
	time.Sleep(s.D)
	return s.R.Read(p)
}

var _ = Describe("Buffer", func() {
	var buffer *Buffer

	BeforeEach(func() {
		buffer = NewBuffer()
	})

	Describe("dumping the entire contents of the buffer", func() {
		It("should return everything that's been written", func() {
			buffer.Write([]byte("abc"))
			buffer.Write([]byte("def"))
			Expect(buffer.Contents()).Should(Equal([]byte("abcdef")))

			Expect(buffer).Should(Say("bcd"))
			Expect(buffer.Contents()).Should(Equal([]byte("abcdef")))
		})
	})

	Describe("creating a buffer with bytes", func() {
		It("should create the buffer with the cursor set to the beginning", func() {
			buffer := BufferWithBytes([]byte("abcdef"))
			Expect(buffer.Contents()).Should(Equal([]byte("abcdef")))
			Expect(buffer).Should(Say("abc"))
			Expect(buffer).ShouldNot(Say("abc"))
			Expect(buffer).Should(Say("def"))
		})
	})

	Describe("creating a buffer that wraps a reader", func() {
		Context("for a well-behaved reader", func() {
			It("should buffer the contents of the reader", func() {
				reader := bytes.NewBuffer([]byte("abcdef"))
				buffer := BufferReader(reader)
				Eventually(buffer).Should(Say("abc"))
				Expect(buffer).ShouldNot(Say("abc"))
				Eventually(buffer).Should(Say("def"))
				Eventually(buffer.Closed).Should(BeTrue())
			})
		})

		Context("for a slow reader", func() {
			It("should allow Eventually to time out", func() {
				slowReader := SlowReader{
					R: bytes.NewBuffer([]byte("abcdef")),
					D: time.Second,
				}
				buffer := BufferReader(slowReader)
				failures := InterceptGomegaFailures(func() {
					Eventually(buffer, 100*time.Millisecond).Should(Say("abc"))
				})
				Expect(failures).ShouldNot(BeEmpty())

				fastReader := SlowReader{
					R: bytes.NewBuffer([]byte("abcdef")),
					D: time.Millisecond,
				}
				buffer = BufferReader(fastReader)
				Eventually(buffer, 100*time.Millisecond).Should(Say("abc"))
				Eventually(buffer.Closed).Should(BeTrue())
			})
		})
	})

	Describe("reading from a buffer", func() {
		It("should read the current contents of the buffer", func() {
			buffer := BufferWithBytes([]byte("abcde"))

			dest := make([]byte, 3)
			n, err := buffer.Read(dest)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(n).Should(Equal(3))
			Expect(string(dest)).Should(Equal("abc"))

			dest = make([]byte, 3)
			n, err = buffer.Read(dest)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(n).Should(Equal(2))
			Expect(string(dest[:n])).Should(Equal("de"))

			n, err = buffer.Read(dest)
			Expect(err).Should(Equal(io.EOF))
			Expect(n).Should(Equal(0))
		})
	})

	Describe("detecting regular expressions", func() {
		It("should fire the appropriate channel when the passed in pattern matches, then close it", func() {
			go func() {
				time.Sleep(10 * time.Millisecond)
				buffer.Write([]byte("abcde"))
			}()

			A := buffer.Detect("%s", "a.c")
			B := buffer.Detect("def")

			var gotIt bool
			select {
			case gotIt = <-A:
			case <-B:
				Fail("should not have gotten here")
			case <-time.After(time.Second):
				Fail("timed out waiting for detection")
			}

			Expect(gotIt).Should(BeTrue())
			Eventually(A).Should(BeClosed())

			buffer.Write([]byte("f"))
			Eventually(B).Should(Receive())
			Eventually(B).Should(BeClosed())
		})

		It("should fast-forward the buffer upon detection", func() {
			buffer.Write([]byte("abcde"))
			select {
			case <-buffer.Detect("abc"):
			case <-time.After(time.Second):
				Fail("timed out waiting for detection")
			}
			Expect(buffer).ShouldNot(Say("abc"))
			Expect(buffer).Should(Say("de"))
		})

		It("should only fast-forward the buffer when the channel is read, and only if doing so would not rewind it", func() {
			buffer.Write([]byte("abcde"))
			A := buffer.Detect("abc")
			time.Sleep(20 * time.Millisecond) //give the goroutine a chance to detect and write to the channel
			Expect(buffer).Should(Say("abcd"))
			select {
			case <-A:
			case <-time.After(time.Second):
				Fail("timed out waiting for detection")
			}
			Expect(buffer).ShouldNot(Say("d"))
			Expect(buffer).Should(Say("e"))
			Eventually(A).Should(BeClosed())
		})

		It("should be possible to cancel a detection", func() {
			A := buffer.Detect("abc")
			B := buffer.Detect("def")
			buffer.CancelDetects()
			buffer.Write([]byte("abcdef"))
			Eventually(A).Should(BeClosed())
			Eventually(B).Should(BeClosed())

			Expect(buffer).Should(Say("bcde"))
			select {
			case <-buffer.Detect("f"):
			case <-time.After(time.Second):
				Fail("timed out waiting for detection")
			}
		})
	})

	Describe("clearing the buffer", func() {
		It("should clear out the contents of the buffer", func() {
			buffer.Write([]byte("abc"))
			Expect(buffer).To(Say("ab"))
			Expect(buffer.Clear()).To(Succeed())
			Expect(buffer).NotTo(Say("c"))
			Expect(buffer.Contents()).To(BeEmpty())
			buffer.Write([]byte("123"))
			Expect(buffer).To(Say("123"))
			Expect(buffer.Contents()).To(Equal([]byte("123")))
		})

		It("should error when the buffer is closed", func() {
			buffer.Write([]byte("abc"))
			buffer.Close()
			err := buffer.Clear()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("closing the buffer", func() {
		It("should error when further write attempts are made", func() {
			_, err := buffer.Write([]byte("abc"))
			Expect(err).ShouldNot(HaveOccurred())

			buffer.Close()

			_, err = buffer.Write([]byte("def"))
			Expect(err).Should(HaveOccurred())

			Expect(buffer.Contents()).Should(Equal([]byte("abc")))
		})

		It("should be closed", func() {
			Expect(buffer.Closed()).Should(BeFalse())

			buffer.Close()

			Expect(buffer.Closed()).Should(BeTrue())
		})
	})
})
