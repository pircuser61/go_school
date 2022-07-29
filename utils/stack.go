package utils

import (
	"errors"
	"sync"
)

type stack struct {
	lock       sync.Mutex
	stackArray []interface{}
	length     int
}

func NewStack() *stack {
	return &stack{
		sync.Mutex{},
		make([]interface{}, 0),
		0,
	}
}

func (stack *stack) GetLength() int {
	return len(stack.stackArray)
}

func (stack *stack) IsEmpty() bool {
	return stack.GetLength() == 0
}

func (stack *stack) PushElement(value interface{}) {
	stack.lock.Lock()
	defer stack.lock.Unlock()
	stack.stackArray = append(stack.stackArray, value)
}

func (stack *stack) PushElementsArray(array []interface{}) {
	stack.lock.Lock()
	defer stack.lock.Unlock()
	stack.stackArray = append(stack.stackArray, array)
}

func (stack *stack) Pop() (interface{}, error) {
	stack.lock.Lock()
	defer stack.lock.Unlock()

	length := stack.GetLength()
	if length == 0 {
		return 0, errors.New("Stack is empty.")
	}

	result := stack.stackArray[length-1]
	stack.stackArray = stack.stackArray[:length-1]
	return result, nil
}
