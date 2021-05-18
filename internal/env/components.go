package env

import (
	"github.com/pkg/errors"
	"os"
	"os/signal"
	"syscall"
)

var appShutdownFuncs []func() error

func WaitClose() os.Signal {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGQUIT)
	s := <-signalChannel
	return s
}

func CloseComponents() []error {
	var errs []error
	for _, f := range appShutdownFuncs {
		//fmt.Printf("%v\n",f)
		err := invokeShutdownFunc(f)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

//逆序添加shutdownFunc
func AddShutdownFunc(f func() error) {
	appShutdownFuncs = append([]func() error{f}, appShutdownFuncs...)
}

func invokeShutdownFunc(f func() error) (err error) {
	defer func() {
		if pan := recover(); pan != nil {
			err = errors.Errorf("panic when invoking shutdown func, panic is: %v", pan)
		}
		return
	}()
	err = f()
	return
}
