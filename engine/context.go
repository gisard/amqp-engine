package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/gisard/amqp-engine"
	"github.com/pkg/errors"
	openamqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

const (
	stackSkip = 3
)

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")

	reset = "\033[0m"
)

type HandlerFunc func(*Context)

type HandlerFuncWithErr func(*Context) error

type handlersChain []HandlerFunc

func logger() HandlerFunc {
	return func(ctx *Context) {
		startTime := time.Now()

		ctx.Next()
		latency := time.Since(startTime) / time.Microsecond
		var message string
		if ctx.err != nil {
			message = ctx.err.Error()
			logrus.Errorf("%v\n%s\n", ctx.err.Error(), debug.Stack())
		} else {
			message = operateFinished
		}

		defer func() {
			logMap := map[string]interface{}{
				"subscribeQueue":  ctx.queue.name,             // 请求队列
				"subscribeBody":   string(ctx.delivery.Body),  // 请求体
				"responseMessage": message,                    // 响应错误信息
				"responseLatency": strconv.Itoa(int(latency)), // 响应时延
			}
			if ctx.err == nil {
				logrus.WithFields(logMap).Info()
			} else {
				logrus.WithFields(logMap).Errorf("%s", ctx.err.Error())
			}
		}()
	}
}

// A middleware that ack message by autoAck and ctx.err.
func ack() HandlerFunc {
	return func(ctx *Context) {
		defer func() {
			if !ctx.queue.autoAck && ctx.err != nil {
				_ = ctx.delivery.Nack(false, false)
				return
			}
			for i := 0; i < defaultRetryNum; i++ {
				err := ctx.delivery.Ack(false)
				if err == nil {
					break
				}
			}
		}()
		ctx.Next()
	}
}

// A middleware that recovers from any panics and put it to context.err.
func recovery() HandlerFunc {
	return func(ctx *Context) {
		defer func() {
			if msg := recover(); msg != nil {
				stack := stack(stackSkip)
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				ctx.err = errors.Errorf("panic: %s\n%s%s", msg, stack, reset)
			}
		}()
		ctx.Next()
	}
}

// stack returns a nicely formatted stack frame, skipping skip frames.
func stack(skip int) []byte {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		_, _ = fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		_, _ = fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}

type Context struct {
	// delivery captures the fields for a previously delivered message resident in
	// a queue to be delivered by the server to a consumer.
	delivery *openamqp.Delivery
	// queue info
	queue *queue

	err error

	// exec chain
	handlers handlersChain
	index    int8

	// This mutex protect Keys map
	mu sync.RWMutex
	// Keys is a key/value pair exclusively for the context of each request.
	Keys map[interface{}]interface{}
}

// Deadline always returns that there is no deadline (ok==false)
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return
}

// Done always returns nil (chan which will wait forever)
func (c *Context) Done() <-chan struct{} {
	return nil
}

// Err always returns nil
func (c *Context) Err() error {
	return c.err
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
func (c *Context) Value(key interface{}) interface{} {
	if key == 0 {
		return nil
	}
	val, _ := c.Get(key)
	return val
}

// Set is used to store a new key/value pair exclusively for this context.
// It also lazy initializes c.Keys if it was not used previously.
func (c *Context) Set(key interface{}, value interface{}) {
	c.mu.Lock()
	if c.Keys == nil {
		c.Keys = make(map[interface{}]interface{})
	}

	c.Keys[key] = value
	c.mu.Unlock()
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exists it returns (nil, false)
func (c *Context) Get(key interface{}) (value interface{}, exists bool) {
	c.mu.RLock()
	value, exists = c.Keys[key]
	c.mu.RUnlock()
	return
}

func newCtx(queue *queue, delivery *openamqp.Delivery, funcs handlersChain) *Context {
	return &Context{
		delivery: delivery,
		queue:    queue,
		handlers: funcs,
	}
}

// ShouldBindJson is a shortcut for c.ShouldBind(desc)
func (c *Context) ShouldBindJson(desc interface{}) error {
	return c.ShouldBind(amqp.JsonFormat, desc)
}

// ShouldBindProto is a shortcut for c.ShouldBind(desc)
func (c *Context) ShouldBindProto(desc interface{}) error {
	return c.ShouldBind(amqp.ProtoFormat, desc)
}

// ShouldBind parser to desc by specific format.
func (c *Context) ShouldBind(format amqp.Format, desc interface{}) error {
	if desc == nil || reflect.TypeOf(desc).Kind() != reflect.Pointer {
		return ErrSubscribeDescIsNotPoint
	}

	content := c.delivery.Body
	var err error
	switch format {
	case amqp.JsonFormat:
		err = json.Unmarshal(content, desc)
		if err != nil {
			return err
		}
	case amqp.ProtoFormat:
		if _, ok := desc.(proto.Message); !ok {
			return ErrProtoFormatNotMatch
		}
		err = proto.Unmarshal(content, desc.(proto.Message))
		if err != nil {
			return err
		}
	}
	return nil
}

// GetContent get []byte from delivery.
func (c *Context) GetContent() []byte {
	return c.delivery.Body
}

// Next Context will exec the next func preferentially.
func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort prevents pending handlers from being called. Note that this will not stop the current handler.
// Call Abort to ensure the remaining handlers for this request are not called.
func (c *Context) Abort() {
	c.index = abortIndex
}

// run start Context run its funcChain.
func (c *Context) run() {
	for c.index < int8(len(c.handlers)) {
		c.handlers[c.index](c)
		c.index++
	}
}
