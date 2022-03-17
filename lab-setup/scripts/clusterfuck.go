// clusterfuck is a simple go program to execute commands/scripts
// on a cluster of machines concurrently using the ssh protocol
// instead of the boring way of logging into each machine individually
// and doing manually the mundane tasks.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
)

// node_type represents the role of node a node in the cluster or
// it identifies what types of nodes to perform an operation on
//go:generate stringer -type=node_type
type node_type uint8

const (
	UNDEFINED node_type = iota
	ALL                 // represents all nodes in the cluster
	MASTER
	WORKER
)

// machine holds the hostname and files representing stdin, stdout and stdout of
// a remote host in the cluster. This is done because writing the the std's
// concurrently fucks up my terminal. The name of the machine should also be
// in the .ssh/config file of your host, because it's used to ssh into the
// remote host
type machine struct {
	name, in, out, err string
	kind               node_type
}

func (m machine) String() string {
	return "NAME: " + m.name + " TYPE: " + m.kind.String()
}

// create a new node machine
func mach(hostname string, hosttype node_type) machine {
	return machine{
		hostname,
		hostname + "\\stdin",
		hostname + "\\stdout",
		hostname + "\\stderr",
		hosttype,
	}
}

var (
	wg sync.WaitGroup

	// shared directory of the vagrant working directory
	shared_directory = "C:\\Users\\big yeti\\linux-networking\\lab-setup"

	// command line argument variables
	debug       bool
	allnodes    bool
	masters     bool
	workers     bool
	single      string
	script_name string
	cmd         string

	// cluster contains all vagrant virtual machines in the cluster
	cluster = []machine{
		mach("master0", MASTER), mach("master1", MASTER), mach("master2", MASTER),
		mach("worker0", WORKER), mach("worker1", WORKER), mach("worker2", WORKER),
		mach("worker3", WORKER), mach("worker4", WORKER), mach("worker5", WORKER),
	}
)

func main() {
	flag.BoolVar(&debug, "v", false, "verbose output")
	flag.BoolVar(&allnodes, "all", false, "perform operation on all nodes in cluster")
	flag.BoolVar(&workers, "workers", false, "perform operation on nodes designated masters in cluster")
	flag.BoolVar(&masters, "masters", false, "perform operation on nodes designated workers in cluster")
	flag.StringVar(&single, "single", "", "specify a single remote host on which to run script or command")
	flag.StringVar(&script_name, "script", "", "specify path of script to run on remote host's shell\nthe script must be in the shared vagrant directory and relative to it")
	flag.StringVar(&cmd, "cmd", "", "specify a command to run on the remote host's shell")

	flag.Parse()

	// execute a script
	if script_name != "" {
		if single != "" {
			ssh(mach(single, UNDEFINED), script_path(script_name))
			return
		}
		if allnodes {
			exec_on_all(script_path(script_name))
		} else if masters {
			exec_on_masters(script_path(script_name))
		} else if workers {
			exec_on_workers(script_path(script_name))
		}
	}

	// execute a command or set of commands
	if cmd != "" {
		if single != "" {
			ssh(mach(single, UNDEFINED), cmd)
			return
		}

		if allnodes {
			exec_on_all(cmd)
		} else if masters {
			exec_on_masters(cmd)
		} else if workers {
			exec_on_workers(cmd)
		}
	}
}

// open files to be used as stdin, stout, stderr for virtual machine
func stdfile(dir, file string) *os.File {
	f, err := os.OpenFile(dir+file, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

// prepare and ssh into specified machine
func ssh(mach machine, args ...string) error {
	cmd := exec.Command("ssh", append([]string{mach.name}, args...)...)
	cmd.Stdin = stdfile(shared_directory+"\\outputs\\", mach.in)
	cmd.Stdout = stdfile(shared_directory+"\\outputs\\", mach.out)
	cmd.Stderr = stdfile(shared_directory+"\\outputs\\", mach.err)
	if debug {
		fmt.Println("\t ** Executing command in node.. " + mach.String())
	}
	return cmd.Run()
}

// part of the script in the virtual machine
func script_path(name string) string {
	return "/vagrant/" + name
}

// execute's script/command con specified machine types
func exec_on(kind node_type, script string) {
	for _, node := range cluster {
		if node.kind == kind || kind == ALL {
			wg.Add(1)
			go func(node machine) {
				defer wg.Done()
				ssh(node, script)
			}(node)
		}
	}
	wg.Wait()
}

// execute script/command on master nodes
func exec_on_masters(script string) {
	exec_on(MASTER, script)
}

// execute script/command on worker nodes
func exec_on_workers(script string) {
	exec_on(WORKER, script)
}

// execute script/command on all nodes
func exec_on_all(script string) {
	exec_on(ALL, script)
}
