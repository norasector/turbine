package util

import "github.com/influxdata/influxdb-client-go/api/write"

type MockWriteAPI struct{}

// WriteRecord writes asynchronously line protocol record into bucket.
// WriteRecord adds record into the buffer which is sent on the background when it reaches the batch size.
// Blocking alternative is available in the WriteApiBlocking interface
func (m *MockWriteAPI) WriteRecord(line string) {}

// WritePoint writes asynchronously Point into bucket.
// WritePoint adds Point into the buffer which is sent on the background when it reaches the batch size.
// Blocking alternative is available in the WriteApiBlocking interface
func (m *MockWriteAPI) WritePoint(point *write.Point) {}

// Flush forces all pending writes from the buffer to be sent
func (m *MockWriteAPI) Flush() {}

// Flushes all pending writes and stop async processes. After this the Write client cannot be used
func (m *MockWriteAPI) Close() {}

// Errors returns a channel for reading errors which occurs during async writes.
// Must be called before performing any writes for errors to be collected.
// The chan is unbuffered and must be drained or the writer will block.
func (m *MockWriteAPI) Errors() <-chan error { return nil }
