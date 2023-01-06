package engine

type GroupList []*QueueGroup

// QueueGroup is a group of queues.
type QueueGroup struct {
	engine *Engine

	// group name
	name string
	// group exec chain
	handlers handlersChain
}

// Group create a group inherited from g.
func (group *QueueGroup) Group(name string, handlers ...HandlerFunc) *QueueGroup {
	return &QueueGroup{
		engine:   group.engine,
		name:     name,
		handlers: group.combineHandlers(handlers),
	}
}

// Group add middleware funcs.
func (group *QueueGroup) Use(handlers ...HandlerFunc) {
	if len(handlers) == 0 {
		return
	}
	group.handlers = append(group.handlers, handlers...)
}

// Group create a queue to handle specific name by f.
// autoAck is true, receive and confirm automatically.
// autoAck is false, acknowledge will be return if err is nil, otherwise return unacknowledge.
func (group *QueueGroup) Handle(queueName string, autoAck bool, f HandlerFuncWithErr) *QueueGroup {
	group.engine.queues = append(group.engine.queues,
		&queue{
			name:     queueName,
			autoAck:  autoAck,
			handlers: group.combineHandlers(handleFuncWithErrList(f)),
		})
	return group
}

// combineHandlers combine group's handlers and params.
func (group *QueueGroup) combineHandlers(handlers handlersChain) handlersChain {
	finalSize := len(group.handlers) + len(handlers)
	if finalSize >= int(abortIndex) {
		panic("too many handlers")
	}
	mergedHandlers := make(handlersChain, finalSize)
	copy(mergedHandlers, group.handlers)
	copy(mergedHandlers[len(group.handlers):], handlers)
	return mergedHandlers
}

// handleFuncWithErr convert HandlerFuncWithErr to HandlerFunc list.
func handleFuncWithErrList(f HandlerFuncWithErr) (handlers handlersChain) {
	handlers = append(handlers, func(ctx *Context) {
		err := f(ctx)
		if err != nil {
			ctx.err = err
		}
	})
	return
}
