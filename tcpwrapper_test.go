package tcpwrapper

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	isrequest "github.com/mmskazak/tcpwrapper/is_request"
)

// mockConn is a mock implementation of net.Conn for testing
type mockConn struct {
	readData  []byte
	writeData []byte
	mu        sync.Mutex
	closed    bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.EOF
	}
	if len(m.readData) == 0 {
		return 0, io.EOF
	}
	n = copy(b, m.readData)
	m.readData = m.readData[n:]
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr  { return nil }
func (m *mockConn) RemoteAddr() net.Addr { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestIsFirstThreeZero_EmptyMessage(t *testing.T) {
	// Test that empty message doesn't cause panic
	result := isrequest.IsFirstThreeZero([]byte{})
	if result != false {
		t.Errorf("Expected false for empty message, got %v", result)
	}
}

func TestIsFirstThreeZero_ShortMessage(t *testing.T) {
	// Test that short messages don't cause panic
	result1 := isrequest.IsFirstThreeZero([]byte{0})
	if result1 != false {
		t.Errorf("Expected false for 1-byte message, got %v", result1)
	}

	result2 := isrequest.IsFirstThreeZero([]byte{0, 0})
	if result2 != false {
		t.Errorf("Expected false for 2-byte message, got %v", result2)
	}
}

func TestNewTCPWrapper_DefaultLogger(t *testing.T) {
	conn := &mockConn{readData: []byte("test\n")}
	wrapper := NewTCPWrapper(conn)
	if wrapper == nil {
		t.Fatal("Expected non-nil wrapper")
	}
}

func TestAddRequestMiddleware_ThreadSafe(t *testing.T) {
	conn := &mockConn{}
	wrapper := NewTCPWrapper(conn)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wrapper.AddRequestMiddleware(func(ctx context.Context, data []byte) ([]byte, error) {
				return data, nil
			})
		}()
	}
	wg.Wait()
}

func TestAddResponseMiddleware_ThreadSafe(t *testing.T) {
	conn := &mockConn{}
	wrapper := NewTCPWrapper(conn)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wrapper.AddResponseMiddleware(func(ctx context.Context, data []byte) ([]byte, error) {
				return data, nil
			})
		}()
	}
	wg.Wait()
}

func TestClose_MultipleCalls(t *testing.T) {
	conn := &mockConn{}
	wrapper := NewTCPWrapper(conn)

	// Close multiple times - should not panic
	err1 := wrapper.Close()
	err2 := wrapper.Close()
	err3 := wrapper.Close()

	if err1 != nil {
		t.Errorf("First close failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second close failed: %v", err2)
	}
	if err3 != nil {
		t.Errorf("Third close failed: %v", err3)
	}
}

func TestProcessMessage_RequestDetection(t *testing.T) {
	conn := &mockConn{readData: []byte{0, 0, 0, 't', 'e', 's', 't', '\n'}}
	wrapper := NewTCPWrapper(conn,
		WithRequestChecker(isrequest.IsFirstThreeZero),
		WithResponseChecker(func(data []byte) bool { return false }),
	)

	ctx := context.Background()
	err := wrapper.(interface{ ProcessMessage(context.Context) error }).ProcessMessage(ctx)
	if err != nil && err != io.EOF {
		t.Errorf("ProcessMessage failed: %v", err)
	}
}

func TestWithConnectionTimeout(t *testing.T) {
	conn := &mockConn{}
	timeout := 5 * time.Second
	wrapper := NewTCPWrapper(conn, WithConnectionTimeout(timeout))
	if wrapper == nil {
		t.Fatal("Expected non-nil wrapper")
	}
}

func TestReadMessage_WithDelimiter(t *testing.T) {
	conn := &mockConn{readData: []byte("hello\n")}
	wrapper := &tcpWrapper{
		conn:             conn,
		requestDelimiter: []byte("\n"),
	}

	msg, err := wrapper.readMessage([]byte("\n"))
	if err != nil {
		t.Fatalf("readMessage failed: %v", err)
	}
	if !bytes.Equal(msg, []byte("hello\n")) {
		t.Errorf("Expected 'hello\\n', got %q", string(msg))
	}
}
