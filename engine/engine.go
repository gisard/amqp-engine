package engine

import (
	"github.com/gisard/amqp-engine"
	"github.com/sirupsen/logrus"
)

// amqp Engine
type Engine struct {
	QueueGroup

	// amqp url
	url      string
	isClosed bool

	// amqp connection info
	conn amqp.Connection
	// engine listening queues
	queues []*queue
	// channel close
	channelClose chan int8
}

// New an asyn messaging engine.
// url like amqp://username:password@127.0.0.1:5672/
func New(url string) *Engine {
	e := &Engine{
		url: url,
		QueueGroup: QueueGroup{
			name:     DefaultRouterGroupName,
			handlers: nil,
		},
	}
	e.QueueGroup.engine = e
	e.Use(logger(), ack(), recovery())
	return e
}

// Engine use middleware funcs.
func (e *Engine) Use(handlers ...HandlerFunc) {
	if len(handlers) == 0 {
		return
	}
	e.handlers = append(e.handlers, handlers...)
}

// Engine start.
func (e *Engine) Run() {
	defer func() {
		e.stop()
	}()
	logrus.Info("Asynchronous Messaging Engine start...")
	// 1.check queue names is unique
	err := e.check()
	if err != nil {
		panic(err)
	}
	// 2.create amqp connection
	e.conn, err = amqp.New(e.url)
	if err != nil {
		panic(err)
	}
	// 3.create channel for every queue，start up goroutine to run every queue
	e.channelClose = make(chan int8, len(e.queues))
	for idx := range e.queues {
		channel, err := e.conn.NewChannel()
		if err != nil {
			panic(err)
		}
		e.queues[idx].channel = channel
		e.queues[idx].index = int8(idx)
		e.queues[idx].closeErr = e.channelClose
		go e.queues[idx].run()
	}

	// main listening connection exception and queue exception.
A:
	for {
		select {
		case closeIdx := <-e.channelClose:
			channel, err := e.conn.NewChannel()
			if err != nil {
				panic(err)
			}
			e.queues[closeIdx].channel = channel
			go e.queues[closeIdx].run()
			logrus.Infof("goroutine: %d already stop，restart it\n", closeIdx)
		case closeErr := <-e.conn.NotifyClose():
			if closeErr == nil {
				e.stop()
				break A
			}
			logrus.Info("connect exception...")
			err = e.restart()
			if err != nil {
				panic(err)
			}
		}
	}
}

// Engine restart.
func (e *Engine) restart() (err error) {
	// engine stop.
	e.stop()

	logrus.Info("Asynchronous Messaging Engine restart...")
	// reconnect amqp server.
	err = e.conn.Reconnect()
	if err != nil {
		return err
	}
	// create channel for every queue，start up goroutine to run every queue.
	e.channelClose = make(chan int8, len(e.queues))
	for idx := range e.queues {
		channel, err := e.conn.NewChannel()
		if err != nil {
			return err
		}
		e.queues[idx].channel = channel
		e.queues[idx].index = int8(idx)
		e.queues[idx].closeErr = e.channelClose
		go e.queues[idx].run()
		logrus.Infof("Begin listening queue: %s\n", e.queues[idx].name)
	}
	e.isClosed = false
	return nil
}

// Engine stop.
func (e *Engine) stop() {
	if e.isClosed {
		return
	}

	e.isClosed = true
	logrus.Info("Asynchronous Messaging Engine close...")
	// close every goroutine
	for idx := range e.queues {
		e.queues[idx].stop()
	}
	// close connection
	if e.conn != nil && !e.conn.IsClosed() {
		e.conn.Close()
	}
}

// check engine params.
func (e *Engine) check() error {
	queueMap := make(map[string]interface{})
	for _, router := range e.queues {
		if router.name == "" {
			return ErrQueueNameIsEmpty
		}
		if _, ok := queueMap[router.name]; ok {
			return ErrQueueNameNotUnique
		}
		queueMap[router.name] = nil
	}
	return nil
}
