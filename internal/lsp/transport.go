package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const (
	lengthHeader      = "Content-Length"
	protocolVersion   = "2.0"
	largestMessage    = 64 << 20
	methodNotFound    = -32601
	serverNotStarted  = -32002
	requestWasInvalid = -32600
	requestFailed     = -32803
)

type message struct {
	Protocol string           `json:"jsonrpc"`
	ID       *json.RawMessage `json:"id,omitempty"`
	Method   string           `json:"method,omitempty"`
	Params   json.RawMessage  `json:"params,omitempty"`
	Result   any              `json:"result,omitempty"`
	Error    *failure         `json:"error,omitempty"`
}

type failure struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type connection struct {
	reader  *bufio.Reader
	writing sync.Mutex
	writer  io.Writer
}

func newConnection(input io.Reader, output io.Writer) *connection {
	return &connection{reader: bufio.NewReader(input), writer: output}
}

func (c *connection) read() (*message, error) {
	length := -1
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, isAHeader := strings.Cut(line, ":")
		if !isAHeader || !strings.EqualFold(strings.TrimSpace(name), lengthHeader) {
			continue
		}
		length, err = strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("reading the length of a message: %w", err)
		}
	}
	if length < 0 || length > largestMessage {
		return nil, errors.New("a message came without a length this server accepts")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return nil, err
	}
	var received message
	if err := json.Unmarshal(body, &received); err != nil {
		return nil, fmt.Errorf("reading a message: %w", err)
	}
	return &received, nil
}

func (c *connection) write(sent message) error {
	sent.Protocol = protocolVersion
	body, err := json.Marshal(sent)
	if err != nil {
		return fmt.Errorf("writing a message: %w", err)
	}
	c.writing.Lock()
	defer c.writing.Unlock()
	if _, err := fmt.Fprintf(c.writer, "%s: %d\r\n\r\n", lengthHeader, len(body)); err != nil {
		return err
	}
	_, err = c.writer.Write(body)
	return err
}

func (c *connection) notify(method string, params any) error {
	encoded, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("writing %s: %w", method, err)
	}
	return c.write(message{Method: method, Params: encoded})
}

func (c *connection) answer(id *json.RawMessage, result any) error {
	if result == nil {
		result = json.RawMessage("null")
	}
	return c.write(message{ID: id, Result: result})
}

func (c *connection) refuse(id *json.RawMessage, code int, reason string) error {
	return c.write(message{ID: id, Error: &failure{Code: code, Message: reason}})
}
