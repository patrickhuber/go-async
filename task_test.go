package async_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickhuber/go-async"
)

var _ = Describe("Task", func() {
	Describe("Run", func() {
		It("runs task", func() {
			t := async.Run(func() (interface{}, error) {
				time.Sleep(100 * time.Millisecond)
				return true, nil
			})

			Expect(t.IsComplete()).To(BeFalse())
			Expect(t).ToNot(BeNil())

			value, err := t.Result()
			Expect(t.IsComplete()).To(BeTrue())
			Expect(value).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
	})
	Describe("ContinueWith", func() {
		It("runs continuation", func() {
			t := async.Run(func() (interface{}, error) {
				defer GinkgoRecover()
				return 13, nil
			}).ContinueWith(func(parent async.Task) (interface{}, error) {
				defer GinkgoRecover()
				Expect(parent.IsComplete()).To(BeTrue())
				value, err := parent.Result()
				Expect(err).To(BeNil())
				Expect(value).To(Equal(13))
				return 80, nil
			})
			value, err := t.Result()
			Expect(err).To(BeNil())
			Expect(value).To(Equal(80))
		})
	})
})
