// Copyright 2015 Apcera Inc. All rights reserved.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/apcera/logray"
)

var log *logray.Logger
var processes = newProcessSet()
var shutdownch = make(chan bool, 10)

type processSet struct {
	processes  map[int]struct{}
	terminated bool
	mutex      sync.Mutex
}

func newProcessSet() *processSet {
	return &processSet{
		processes: make(map[int]struct{}),
	}
}

func (p *processSet) add(pid int) {
	p.mutex.Lock()
	// if we've already got the term, we shouldn't add more processes, so to try
	// and help the race, we'll sort of kill this new process
	if p.terminated {
		syscall.Kill(pid, syscall.SIGTERM)
	}

	p.processes[pid] = struct{}{}
	p.mutex.Unlock()
}

func (p *processSet) remove(pid int) {
	p.mutex.Lock()
	delete(p.processes, pid)
	p.mutex.Unlock()
}

func (p *processSet) term() {
	p.mutex.Lock()

	for pid, _ := range p.processes {
		syscall.Kill(pid, syscall.SIGTERM)
	}

	if !p.terminated {
		// If we haven't already gotten a sig term, then close the shutdown
		// channel. This is mainly to prevent receiving the signal twice causing a
		// panic.
		close(shutdownch)
	}

	p.terminated = true

	p.mutex.Unlock()
}

func main() {
	var configFile string
	var logLevel string

	flag.StringVar(&configFile, "f", "", "file to use to define processes")
	flag.StringVar(&logLevel, "log-levle", "info+", "log message levels to be logged")
	flag.Parse()

	// validate we have a config file
	if configFile == "" {
		fmt.Fprint(os.Stderr, "No config file specified with -f\n")
		os.Exit(1)
	}

	// validate the configured log level
	logclass, err := logray.ParseLogClass(logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse log class: %v\n", err)
		os.Exit(1)
	}
	logray.AddDefaultOutput("stdout://", logclass)
	log = logray.New()

	// load the config file
	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
		os.Exit(1)
	}

	// begin the signal handling
	handleSigterm()

	// begin managing processes
	wg := sync.WaitGroup{}
	for _, args := range config {
		wg.Add(1)
		go func(args []string) {
			defer wg.Done()
			manageProcess(args)
		}(args)
	}

	// wait for all to finish
	wg.Wait()
}

func handleSigterm() {
	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGTERM)
	go func() {
		for _ = range sigch {
			processes.term()
		}
	}()
}

// loadConfig handles loading specified configuration file and returning it.
func loadConfig(configFile string) ([][]string, error) {
	f, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config [][]string
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}
	return config, nil
}

func manageProcess(args []string) {
	for {
		// start the process
		log.Infof("Starting: %#v", args)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		// if the command fails to start, we'll retry it but delay 10 seconds
		if err := cmd.Start(); err != nil {
			log.Errorf("Command failed to start, retrying in 10 seconds: %v", err)
			select {
			case <-shutdownch:
				// told to shutdown
				return
			case <-time.After(time.Second * 10):
				// start again
				continue
			}
		}

		// if the command exits after its started, we'll relaunch it immediately
		pid := cmd.Process.Pid
		processes.add(pid)
		cmd.Wait()
		processes.remove(pid)

		// figure out what to do when it exits
		select {
		case <-shutdownch:
			// if it reads on this channel, it means we've got a sigterm and should
			// shutdown
			return
		default:
			// default will prevent this from blocking on the channel read, it is to trigger a restart
			log.Warnf("Command exited, relaunching")
		}
	}
}
