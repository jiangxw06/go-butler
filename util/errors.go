package util

import (
	"github.com/pkg/errors"
	"runtime"
)

func ErrorWithStack(err error) error {
	stack := make([]byte, 2<<11)
	length := runtime.Stack(stack, false)
	return errors.Errorf("%v,stack: %s\n", err, stack[:length])
}
