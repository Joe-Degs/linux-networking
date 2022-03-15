package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
)

type machine struct {
	name string
	in   string
	out  string
	err  string
}

func mach(name string) machine {
	return machine{
		name,
		name + "\\stdin",
		name + "\\stdout",
		name + "\\stderr",
	}
}

var (
	wg sync.WaitGroup

	// vagrant working directory
	vagrantDir = "C:\\Users\\big yeti\\linux-networking\\lab-setup"

	// command line arguments
	allnodes    bool
	masters     bool
	workers     bool
	single      string
	script_name string
	cmdline     string

	// nodes in the cluster of virtual machines
	master_nodes = []machine{mach("master0"), mach("master1"), mach("master2")}
	worker_nodes = []machine{mach("worker0"), mach("worker1"), mach("worker2"), mach("worker3"), mach("worker4"), mach("worker5")}
)

func main() {
	flag.BoolVar(&allnodes, "all", false, "run script on all nodes in cluster")
	flag.BoolVar(&workers, "workers", false, "run script on only master nodes in cluster")
	flag.BoolVar(&masters, "masters", false, "run script on only worker nodes in cluster")
	flag.StringVar(&single, "single", "", "run script only on specified node")
	flag.StringVar(&script_name, "script", "", "path of the script relative to the Vagrantfile directory")
	flag.StringVar(&cmdline, "cmdline", "", "run commandline script")

	flag.Parse()

	fmt.Println(script_name)

	if script_name != "" {
		if single != "" {
			ssh(mach(single), script_path(script_name))
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

	if cmdline != "" {
		if single != "" {
			ssh(mach(single), cmdline)
			return
		}

		if allnodes {
			exec_on_all(cmdline)
		} else if masters {
			exec_on_masters(cmdline)
		} else if workers {
			exec_on_workers(cmdline)
		}
	}
}

// open files to be used as stdin, stout, stderr for virtual machine
func stdfile(dir, file string) *os.File {
	f, err := os.OpenFile(dir+file, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

// prepare and ssh into specified machine
func ssh(mach machine, args ...string) error {
	cmd := exec.Command("ssh", append([]string{mach.name}, args...)...)
	cmd.Stdin = stdfile(vagrantDir+"\\outputs\\", mach.in)
	cmd.Stdout = stdfile(vagrantDir+"\\outputs\\", mach.out)
	cmd.Stderr = stdfile(vagrantDir+"\\outputs\\", mach.err)
	fmt.Println(cmd.Dir)
	return cmd.Run()
}

// part of the script in the virtual machine
func script_path(name string) string {
	return "/vagrant/" + name
}

func exec_on(nodes []machine, script string) {
	for _, node := range nodes {
		wg.Add(1)
		go func(node machine) {
			defer wg.Done()
			ssh(node, script)
		}(node)
	}
	wg.Wait()
}

func exec_on_masters(script string) {
	exec_on(master_nodes, script)
}

func exec_on_workers(script string) {
	exec_on(worker_nodes, script)
}

func exec_on_all(script string) {
	exec_on(append(master_nodes, worker_nodes...), script)
}
