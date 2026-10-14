package plugin

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

var (
	errBatchTestFirst  = errors.New("first failure")
	errBatchTestSecond = errors.New("second failure")
)

func TestBatchError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		errs    []error
		wantErr bool
	}{
		{
			name: "all succeed",
			errs: []error{nil, nil, nil},
		},
		{
			name: "partial success is not an error",
			errs: []error{nil, errBatchTestFirst, nil, errBatchTestSecond, nil},
		},
		{
			name:    "all attempted failed",
			errs:    []error{errBatchTestFirst, errBatchTestSecond},
			wantErr: true,
		},
		{
			name: "nothing attempted",
		},
	}

	ig := &InstanceGroup{log: hclog.NewNullLogger()}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := ig.batchError("batch failed", testCase.errs)

			if !testCase.wantErr {
				require.NoError(t, err)

				return
			}

			// Every individual failure must survive the aggregate.
			for _, expectedErr := range testCase.errs {
				require.ErrorIs(t, err, expectedErr)
			}
		})
	}
}

func TestRunParallel(t *testing.T) {
	t.Parallel()

	errs := runParallel(3, func(index int) error {
		if index == 1 {
			return errBatchTestFirst
		}

		return nil
	})

	// Every call's error lands in its own slot, so a failure stays matched to its index.
	require.Equal(t, []error{nil, errBatchTestFirst, nil}, errs)

	require.Empty(t, runParallel(0, func(int) error { return errBatchTestFirst }))

	// A panic in one call becomes that slot's error instead of taking the process down, and
	// the other slots still complete.
	panicked := runParallel(3, func(index int) error {
		if index == 2 {
			panic("boom")
		}

		return nil
	})

	require.ErrorIs(t, panicked[2], ErrInstanceOperationPanic)
	require.ErrorContains(t, panicked[2], "boom")
	require.NoError(t, panicked[0])
	require.NoError(t, panicked[1])
}

// A batch as large as the pool runs no more than maxParallelInstanceOperations operations at
// once, and every slot still completes.
func TestRunParallelCapsConcurrency(t *testing.T) {
	t.Parallel()

	const batch = maxParallelInstanceOperations * 2

	var (
		inFlight    atomic.Int64
		maxInFlight atomic.Int64
	)

	errs := runParallel(batch, func(int) error {
		current := inFlight.Add(1)

		for {
			previous := maxInFlight.Load()
			if current <= previous || maxInFlight.CompareAndSwap(previous, current) {
				break
			}
		}

		time.Sleep(time.Millisecond)

		inFlight.Add(-1)

		return nil
	})

	require.Len(t, errs, batch)

	for index, attemptErr := range errs {
		require.NoError(t, attemptErr, "slot %d", index)
	}

	require.LessOrEqual(t, maxInFlight.Load(), int64(maxParallelInstanceOperations))
}
