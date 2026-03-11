package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego/v2/server/web"
	"github.com/kardianos/service"

	"bigtoy/backend/routers"
)

const (
	serviceName        = "BigToyGarage"
	serviceDisplayName = "BigToy Garage Service"
	serviceDescription = "BigToy die-cast model gallery and admin backend service"
)

type program struct {
	done chan struct{}
}

var (
	registerOnce sync.Once
	registerErr  error
)

func (p *program) Start(_ service.Service) error {
	if p.done == nil {
		p.done = make(chan struct{})
	}
	go p.run()
	return nil
}

func (p *program) run() {
	defer close(p.done)
	if err := registerApplication(); err != nil {
		log.Printf("failed to initialize application: %v", err)
		return
	}
	web.Run()
}

func (p *program) Stop(_ service.Service) error {
	if web.BeeApp == nil || web.BeeApp.Server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := web.BeeApp.Server.Shutdown(ctx); err != nil {
		_ = web.BeeApp.Server.Close()
	}

	select {
	case <-p.done:
		return nil
	case <-time.After(12 * time.Second):
		return fmt.Errorf("timeout waiting for server shutdown")
	}
}

func main() {
	if err := ensureWorkingDirectory(); err != nil {
		log.Fatalf("failed to prepare working directory: %v", err)
	}

	svc, err := service.New(&program{}, &service.Config{
		Name:        serviceName,
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		Option: service.KeyValue{
			"DelayedAutoStart": true,
		},
	})
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}

	handled, err := handleServiceCommand(svc, os.Args[1:])
	if handled {
		if err != nil {
			log.Fatalf("service command failed: %v", err)
		}
		return
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service run failed: %v", err)
	}
}

func ensureWorkingDirectory() error {
	// Keep current cwd for normal development if config is already resolvable.
	if exists(filepath.Join("conf", "app.conf")) || exists(filepath.Join("backend", "conf", "app.conf")) {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(exePath)
	if exeDir == "" {
		return nil
	}
	return os.Chdir(exeDir)
}

func handleServiceCommand(svc service.Service, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	if !strings.EqualFold(args[0], "service") && !strings.EqualFold(args[0], "svc") {
		return false, nil
	}

	if len(args) == 1 {
		printServiceUsage()
		return true, nil
	}

	action := strings.ToLower(strings.TrimSpace(args[1]))
	switch action {
	case "install":
		if err := svc.Install(); err != nil {
			return true, err
		}
		fmt.Println("service installed")
		return true, nil
	case "uninstall":
		if err := svc.Uninstall(); err != nil {
			return true, err
		}
		fmt.Println("service uninstalled")
		return true, nil
	case "start", "stop", "restart":
		if err := service.Control(svc, action); err != nil {
			return true, err
		}
		fmt.Printf("service %s success\n", action)
		return true, nil
	case "status":
		state, err := svc.Status()
		if err != nil {
			return true, err
		}
		fmt.Printf("service status: %s\n", formatServiceStatus(state))
		return true, nil
	default:
		printServiceUsage()
		return true, fmt.Errorf("unsupported service action: %s", action)
	}
}

func formatServiceStatus(state service.Status) string {
	switch state {
	case service.StatusRunning:
		return "running"
	case service.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

func printServiceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  bigtoy.exe                 # run in foreground")
	fmt.Println("  bigtoy.exe service install")
	fmt.Println("  bigtoy.exe service uninstall")
	fmt.Println("  bigtoy.exe service start")
	fmt.Println("  bigtoy.exe service stop")
	fmt.Println("  bigtoy.exe service restart")
	fmt.Println("  bigtoy.exe service status")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func registerApplication() error {
	registerOnce.Do(func() {
		registerErr = routers.Register()
	})
	return registerErr
}
