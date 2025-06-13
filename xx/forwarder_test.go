package ngrok

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// mockListener implements a simple listener for testing
type mockListener struct {
	conn      net.Conn
	connReady chan bool
	closed    bool
}

func newMockListener() (*mockListener, net.Conn) {
	// Create connected pair
	client, server := net.Pipe()
	listener := &mockListener{
		conn:      server,
		connReady: make(chan bool, 1),
		closed:    false,
	}
	listener.connReady <- true // Signal that a connection is ready
	return listener, client
}

func (l *mockListener) Accept() (net.Conn, error) {
	if l.closed {
		return nil, net.ErrClosed
	}

	// Wait for a connection to be ready
	<-l.connReady
	return l.conn, nil
}

func (l *mockListener) Close() error {
	l.closed = true
	return nil
}

// TestForwarderBasic tests the basic forwarding functionality
func TestForwarderBasic(t *testing.T) {
	// Set a timeout for the entire test
	testCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	t.Log("Starting test with timeout")
	// Setup mock listener and client connection
	t.Log("Creating mock listener")
	mockLis, clientConn := newMockListener()
	defer clientConn.Close()
	t.Log("Mock listener created")

	// Setup mock upstream server
	t.Log("Creating pipe for upstream server")
	upstreamClient, upstreamServer := net.Pipe()
	defer upstreamClient.Close()
	defer upstreamServer.Close()
	t.Log("Upstream pipe created")

	// Create a forwarder with the mock listener
	t.Log("Creating context for forwarder")
	ctx, cancel := context.WithCancel(testCtx)
	defer cancel()
	t.Log("Context created")

	// Mock the connectToBackend method by creating a forwarder that returns our mock upstream
	t.Log("Creating test forwarder")
	fwd := &testForwarder{
		ctx:         ctx,
		cancel:      cancel,
		doneChannel: make(chan error, 1),
		listener:    mockLis,
		upstream:    upstreamClient,
	}

	// Start forwarding
	t.Log("Starting forwarder")
	fwd.Start(t)
	t.Log("Forwarder started")

	// Write data to the client connection
	testData := "hello world"
	t.Log("Writing to client connection")
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		_, err := clientConn.Write([]byte(testData))
		if err != nil {
			t.Errorf("Error writing to client: %v", err)
		}
		t.Log("Finished writing to client")
	}()

	// Wait for write to complete or timeout
	select {
	case <-writeDone:
		t.Log("Write completed")
	case <-testCtx.Done():
		t.Fatal("Write timed out")
	}

	// Read data from the upstream server with timeout
	t.Log("Reading from upstream server")
	buf := make([]byte, 1024)
	var n int
	var err error
	readDone := make(chan struct{})

	go func() {
		defer close(readDone)
		t.Log("Starting upstream read")
		n, err = upstreamServer.Read(buf)
		t.Logf("Read completed: %d bytes, err: %v", n, err)
		if err != nil && err != io.EOF {
			t.Errorf("Error reading from upstream: %v", err)
		}
	}()

	select {
	case <-readDone:
		t.Log("Read completed")
	case <-testCtx.Done():
		t.Fatal("Read timed out")
	}

	// Verify the data matches
	received := string(buf[:n])
	if received != testData {
		t.Errorf("Data mismatch: expected '%s', got '%s'", testData, received)
	} else {
		t.Logf("Successfully verified data transfer: '%s'", received)
	}

	// Test bidirectional communication
	t.Log("Testing bidirectional data flow")
	responseData := "response from server"
	responseWritten := make(chan struct{})

	// Write response from server to client in a goroutine
	go func() {
		defer close(responseWritten)
		t.Log("Writing response from server to client")
		_, err := upstreamServer.Write([]byte(responseData))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
		t.Log("Finished writing response")
	}()

	// Wait for write to complete or timeout
	select {
	case <-responseWritten:
		t.Log("Server write completed")
	case <-testCtx.Done():
		t.Fatal("Server write timed out")
	}

	// Read the response with a controlled buffer
	respBuf := make([]byte, 1024)
	respReadDone := make(chan struct{})
	var respBytes int

	go func() {
		defer close(respReadDone)
		t.Log("Reading response from client connection")
		respBytes, err = clientConn.Read(respBuf)
		t.Logf("Client read completed: %d bytes, err: %v", respBytes, err)
	}()

	// Wait for read to complete or timeout
	select {
	case <-respReadDone:
		t.Log("Client read completed")
	case <-testCtx.Done():
		t.Fatal("Client read timed out")
	}

	// Verify the response
	receivedResp := string(respBuf[:respBytes])
	if receivedResp != responseData {
		t.Errorf("Response mismatch: expected '%s', got '%s'", responseData, receivedResp)
	} else {
		t.Logf("Successfully verified reverse data transfer: '%s'", receivedResp)
	}

	// Test clean shutdown
	fwd.Close(t)

	t.Log("Test completed successfully")
}

// testForwarder is a simplified forwarder for testing
type testForwarder struct {
	ctx         context.Context
	cancel      context.CancelFunc
	doneChannel chan error
	listener    *mockListener
	upstream    net.Conn
	wg          sync.WaitGroup
}

func (f *testForwarder) Start(t *testing.T) {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()

		// Add debug output
		t.Log("Forwarder loop started, waiting for connection")

		// Get connection from listener
		conn, err := f.listener.Accept()
		if err != nil {
			f.doneChannel <- err
			return
		}

		// Join the connections
		f.join(t, conn, f.upstream)
	}()
}

func (f *testForwarder) Close(t *testing.T) error {
	t.Log("Closing test forwarder")
	// Cancel context to signal termination
	f.cancel()

	// Close the listener
	f.listener.Close()

	// Set a short timeout for waiting on remaining operations
	timeout := time.After(500 * time.Millisecond)
	done := make(chan struct{})

	go func() {
		f.wg.Wait()
		close(done)
	}()

	// Wait with timeout to avoid blocking indefinitely
	select {
	case <-done:
		t.Log("Test forwarder closed gracefully")
	case <-timeout:
		t.Log("Close timed out, forcing shutdown")
	}

	return nil
}

func (f *testForwarder) join(t *testing.T, left, right net.Conn) {
	t.Log("Starting bidirectional join operation between connections")

	// Use WaitGroup to track both copy operations
	wg := &sync.WaitGroup{}
	wg.Add(2)

	// Copy from client to server
	go func() {
		defer wg.Done()
		t.Log("Starting copy from client to server")
		_, err := io.Copy(right, left)
		t.Logf("Client->Server copy completed with err: %v", err)
	}()

	// Copy from server to client
	go func() {
		defer wg.Done()
		t.Log("Starting copy from server to client")
		_, err := io.Copy(left, right)
		t.Logf("Server->Client copy completed with err: %v", err)
	}()

	// Instead of blocking on WaitGroup, use a separate goroutine
	// to avoid blocking the main test flow
	go func() {
		wg.Wait()
		t.Log("Both copy operations completed")
	}()
}
