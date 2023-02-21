package main

import (
	"os"
	"os/signal"
	"syscall"

	controllers "digode.dev/mycontroller/controller"
)

func main() {
	controller := controllers.NewDeployWatcher()
	stopCh := make(chan struct{})
	defer close(stopCh)

	go controller.Run(stopCh)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
