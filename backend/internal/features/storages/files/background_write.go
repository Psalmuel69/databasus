package storage_files

import (
	"context"
	"io"
)

// BackgroundWrite is an upload running beside the producer that feeds it through a
// pipe. Logical dumps and physical streams drive it the same way: peek at the error
// while the producer is still running, then take the receipt once the writer is done.
type BackgroundWrite struct {
	// Errors stays writable because the engines put a peeked value back and their
	// cancellation cleanup drains it.
	Errors   chan error
	Receipts chan WriteReceipt
}

// The receipt is sent before the error, so a caller holding
// the error knows the receipt is already waiting.
func StartBackgroundWrite(
	ctx context.Context,
	store FileStore,
	reference StoredFileReference,
	source *io.PipeReader,
	onFailure context.CancelCauseFunc,
) BackgroundWrite {
	write := BackgroundWrite{
		Errors:   make(chan error, 1),
		Receipts: make(chan WriteReceipt, 1),
	}

	go func() {
		receipt, err := store.WriteFile(ctx, reference, source)
		if err != nil {
			_ = source.CloseWithError(err)
			onFailure(err)
		}

		write.Receipts <- receipt
		write.Errors <- err
	}()

	return write
}
