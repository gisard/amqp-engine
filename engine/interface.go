package engine

import (
	"errors"
	"math"
)

var (
	ErrProtoFormatNotMatch = errors.New("proto format is not match")

	ErrSubscribeDescIsNotPoint = errors.New("desc is not a point")

	ErrQueueNameNotUnique = errors.New("queue name is not unique")

	ErrQueueNameIsEmpty = errors.New("name name is empty")
)

const (
	ErrQueueNameNotUniqueFmt = "queue name is not unique: %s"
)

const (
	DefaultRouterGroupName = ""

	defaultRetryNum = 5
)

const abortIndex int8 = math.MaxInt8 / 2

const (
	operateFinished = "处理完成"
)
