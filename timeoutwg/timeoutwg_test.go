// Copyright 2017 Apcera Inc. All rights reserved.

package timeoutwg

import (
	"testing"
	"time"

	"github.com/apcera/util/testtool"
)

func TestTimeoutWaitGroup(t *testing.T) {
	testcases := []struct {
		desc              string
		addDeltas         []int
		removes           int
		expectTestTimeout bool
		expectWaitReturn  int
		retry             bool
		expectPanic       bool
		postWaitAdd       bool
	}{
		{
			desc:              "Success",
			addDeltas:         []int{1, 1, 1},
			removes:           3,
			expectTestTimeout: false,
			expectWaitReturn:  0,
		},
		{
			desc:              "ZeroWait",
			addDeltas:         []int{},
			removes:           0,
			expectTestTimeout: false,
			expectWaitReturn:  0,
		},
		{
			desc:              "Fail",
			addDeltas:         []int{1, 1, 1},
			removes:           2,
			expectTestTimeout: true,
			expectWaitReturn:  1,
		},
		{
			desc:              "MultiAdd",
			addDeltas:         []int{1, 3, 1},
			removes:           5,
			expectTestTimeout: false,
			expectWaitReturn:  0,
		},
		{
			desc:              "SuccessWithRetry",
			addDeltas:         []int{1, 1, 1},
			removes:           3,
			expectTestTimeout: false,
			expectWaitReturn:  0,
			retry:             true,
		},
		{
			desc:              "FailWithRetry",
			addDeltas:         []int{1, 1, 1},
			removes:           2,
			expectTestTimeout: true,
			expectWaitReturn:  1,
			retry:             true,
		},
		{
			desc:              "TooManyDone",
			addDeltas:         []int{1},
			removes:           2,
			expectTestTimeout: false,
			retry:             true,
			expectPanic:       true,
		},
		{
			desc:              "AddAfterWait",
			addDeltas:         []int{1},
			removes:           2,
			postWaitAdd:       true,
			expectTestTimeout: false,
			retry:             true,
			expectPanic:       true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			var twg TimeoutWaitGroup

			if tc.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Fatalf("Expected panic, but did not get one.")
					}
				}()
			}

			totalAdds := 0
			for _, delta := range tc.addDeltas {
				totalAdds += delta
				twg.Add(delta)
			}
			done := make(chan int, 1)
			defer func() { close(done) }()

			if tc.postWaitAdd {
				twg.Wait(0)
				twg.Add(1) // should panic here, can verify that in panic handler
			}

			if tc.retry {
				// We expect this to always timeout.
				testtool.TestEqual(t, twg.Wait(0), totalAdds)
			}
			for i := 0; i < tc.removes; i++ {
				twg.Done()
			}
			go func() {
				done <- twg.Wait(300 * time.Millisecond)
			}()
			select {
			case <-time.After(100 * time.Millisecond):
				if !tc.expectTestTimeout {
					t.Fatal("Wait did not return within the expected time.")
				}
				// Get the RV when the Wait finally times out.
				rv := <-done
				testtool.TestEqual(t, rv, tc.expectWaitReturn)
			case rv := <-done:
				if tc.expectTestTimeout {
					t.Fatal("Wait unexpectedly returned.")
				}
				testtool.TestEqual(t, rv, tc.expectWaitReturn)
			}
		})
	}
}
