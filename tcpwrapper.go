package tcpwrapper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	isrequest "github.com/mmskazak/tcpwrapper/is_request"
	isresponse "github.com/mmskazak/tcpwrapper/is_response"
	"go.uber.org/zap"
)

// Middleware defines a type of middleware function for processing messages.
// It now accepts context to support timeouts and cancellation.
type Middleware func(ctx context.Context, data []byte) ([]byte, error)

// Wrapper defines the public API for TCP wrapper
type Wrapper interface {
	AddRequestMiddleware(mw Middleware)
	AddResponseMiddleware(mw Middleware)
	// HandleMessage processes a single message blockingly.
	HandleMessage(ctx context.Context) error
	// Serve starts listening for messages until context is cancelled or connection closes.
	Serve(ctx context.Context) error
	Close() error
}

// tcpWrapper is a wrapper over a TCP connection that allows
// applying different middleware chains for processing requests and responses.
type tcpWrapper struct {
	conn                net.Conn
	requestDelimiter    []byte
	responseDelimiter   []byte
	requestMiddlewares  []Middleware
	responseMiddlewares []Middleware
	isRequest           isrequest.IsRequestFunc
	isResponse          isresponse.IsResponseFunc
	logger              *zap.SugaredLogger
}

// NewTCPWrapper creates a new instance of TCPWrapper with the given connection and options.
func NewTCPWrapper(conn net.Conn, opts ...Option) Wrapper {
	// Create wrapper with default values
	w := &tcpWrapper{
		conn:                conn,
		requestDelimiter:    []byte("\n"), // default delimiter
		responseDelimiter:   []byte("\n"), // default delimiter
		requestMiddlewares:  make([]Middleware, 0),
		responseMiddlewares: make([]Middleware, 0),
		isRequest:           isrequest.IsDummy,  // default checker
		isResponse:          isresponse.IsDummy, // default checker
	}

	// Create default logger if not set
	if w.logger == nil {
		logger, _ := zap.NewProduction()
		w.logger = logger.Sugar()
	}

	// Apply all options
	for _, opt := range opts {
		opt(w)
	}

	return w
}

// AddRequestMiddleware adds a middleware for request processing.
func (tw *tcpWrapper) AddRequestMiddleware(mw Middleware) {
	tw.requestMiddlewares = append(tw.requestMiddlewares, mw)
}

// AddResponseMiddleware adds a middleware for response processing.
func (tw *tcpWrapper) AddResponseMiddleware(mw Middleware) {
	tw.responseMiddlewares = append(tw.responseMiddlewares, mw)
}

// readMessage reads data from the connection until one of the following conditions is met:
// 1. If a Content-Length header is found, reads the specified number of bytes.
// 2. If an explicit delimiter is detected, considers the message complete.
// 3. If EOF is received, returns the accumulated data.
func (tw *tcpWrapper) readMessage(delimiter []byte) ([]byte, error) {
	var buffer []byte
	temp := make([]byte, 256)
	expectedLength := -1

	for {
		n, err := tw.conn.Read(temp)
		if err != nil && err != io.EOF {
			return nil, err
		}
		buffer = append(buffer, temp[:n]...)

		// If expected length is not set, try to extract Content-Length from headers.
		if expectedLength == -1 {
			// Assume headers end with \r\n\r\n
			if headerEnd := bytes.Index(buffer, []byte("\r\n\r\n")); headerEnd != -1 {
				headers := buffer[:headerEnd]
				if cl, err := extractContentLength(headers); err == nil {
					// Final length = headers + 4 bytes (\r\n\r\n) + body length
					expectedLength = headerEnd + 4 + cl
				}
			}
		}

		if expectedLength != -1 && len(buffer) >= expectedLength {
			break
		}

		if len(delimiter) > 0 && bytes.HasSuffix(buffer, delimiter) {
			break
		}

		if err == io.EOF {
			break
		}
	}

	return buffer, nil
}

// HandleMessage reads a complete message, determines its type (response or request),
// and runs the corresponding middleware chain before sending the result back.
func (tw *tcpWrapper) HandleMessage(ctx context.Context) error {
	// Use RequestDelimiter to read the message.
	message, err := tw.readMessage(tw.requestDelimiter)
	if err != nil {
		return err
	}

	// Use the provided isRequest and isResponse functions to determine message type
	if tw.isRequest(message) {
		tw.logger.Infof("Request received: %s", string(message))
		for _, mw := range tw.requestMiddlewares {
			message, err = mw(ctx, message)
			if err != nil {
				return err
			}
			// Check if context was cancelled during middleware execution
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	} else if tw.isResponse(message) {
		tw.logger.Infof("Response received: %s", string(message))
		for _, mw := range tw.responseMiddlewares {
			message, err = mw(ctx, message)
			if err != nil {
				return err
			}
			// Check if context was cancelled during middleware execution
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	}

	_, err = tw.conn.Write(message)
	return err
}

// Serve starts an infinite loop to handle messages until the context is cancelled or connection closes.
func (tw *tcpWrapper) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := tw.HandleMessage(ctx); err != nil {
				if err == io.EOF {
					return nil // Normal connection close
				}
				return err
			}
		}
	}
}

// Close properly closes the connection.
func (tw *tcpWrapper) Close() error {
	return tw.conn.Close()
}

// extractContentLength searches for the "Content-Length" header in headers and returns its value.
// If not found, returns an error.
func extractContentLength(headers []byte) (int, error) {
	lines := strings.Split(string(headers), "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				clStr := strings.TrimSpace(parts[1])
				return strconv.Atoi(clStr)
			}
		}
	}
	return 0, fmt.Errorf("Content-Length not found")
}
