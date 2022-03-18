// clusterfuck is a simple go program to execute commands/scripts
// on a cluster of machines concurrently using the ssh protocol
// instead of the boring way of logging into each machine individually
// and doing manually the mundane tasks. I use snake_case in this file
// alot because its a script and sometimes i like doing things that are
// contrary to accepted norms just to see what it feels like to be on the
// sideline.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
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
	name    string
	kind    node_type
	streams *std_stream
}

type std_stream struct {
	connected    bool
	stdout       io.Reader
	in, out, err io.ReadWriteCloser
}

// open files to be used as stdin, stout, stderr for virtual machine
func new_file_stream(dir, file string) *os.File {
	f, err := os.OpenFile(dir+file, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

// open stream for reading or writing without truncating the old
// contents of the file. this is useful for reading or writing a file
// after its operations are done
func open_stream(dir, file string) *os.File {
	f, err := os.OpenFile(dir+file, os.O_RDONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

// open a  stream on the host machine connecting its remote streams to it
func (s *std_stream) get_new_streams(hostname string) {
	s.in = new_file_stream(shared_directory+"\\outputs\\", hostname+"\\stdin")
	s.out = new_file_stream(shared_directory+"\\outputs\\", hostname+"\\stdout")
	s.err = new_file_stream(shared_directory+"\\outputs\\", hostname+"\\stderr")
}

// return a new stream to connect to remote host's std streams
func new_streams(hostname string) *std_stream {
	s := &std_stream{}
	s.get_new_streams(hostname)
	return s
}

// dummy makes an io.Writer an io.ReadWriteCloser
type dummy struct{ w io.Writer }

func (d dummy) Close() error                { return nil }
func (d dummy) Read(p []byte) (int, error)  { return 0, nil }
func (d dummy) Write(p []byte) (int, error) { return d.w.Write(p) }

// connect machine's stdout file to the terminal stdout
func (s *std_stream) StreamOut() {
	r, w := io.Pipe()
	s.stdout = r
	s.out = dummy{io.MultiWriter(w, s.out)}
	s.connected = true
}

// connect machine's stdout file to the terminal stdout
func (s *std_stream) WriteIn() {}

// connect machine's stdout file to the terminal stdout
func (s *std_stream) StreamErr() {}

func (m machine) String() string {
	return "NAME: " + m.name + " TYPE: " + m.kind.String()
}

// create a new node machine
func mach(hostname string, hosttype node_type) machine {
	return machine{
		hostname,
		hosttype,
		new_streams(hostname),
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
	connect     string
	list        string

	// cluster contains all vagrant virtual machines in the cluster
	cluster = []machine{
		mach("master0", MASTER), mach("master1", MASTER), mach("master2", MASTER),
		mach("worker0", WORKER), mach("worker1", WORKER), mach("worker2", WORKER),
		mach("worker3", WORKER), mach("worker4", WORKER), mach("worker5", WORKER),
	}
)

// prepare and ssh into specified machine
func ssh(mach machine, args ...string) error {
	cmd := exec.Command("ssh", append([]string{mach.name}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = mach.streams.in, mach.streams.out, mach.streams.err
	if debug {
		fmt.Println("\t ** Executing command in node.. " + mach.String())
	}
	if mach.streams.connected {
		go io.Copy(os.Stdout, mach.streams.stdout)
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

func find_machine(name string) machine {
	for _, m := range cluster {
		if m.name == name {
			return m
		}
	}
	return machine{}
}

// TODO(Joe-Degs):
// this whole reading the file things does not work. probably stop
// doing it
func print(m machine) {
	stdout := open_stream(shared_directory+"\\outputs\\", m.name+"\\stdout")
	io.Copy(os.Stdout, stdout)
}

func read_std_streams(machs string) {
	ms := strings.Split(machs, ",")
	for _, m := range ms {
		mach := find_machine(m)
		if mach.name != "" {
			fmt.Println("found machine " + mach.String())
			print(mach)
		}
	}
}

func main() {
	flag.BoolVar(&debug, "v", false, "verbose output")
	flag.BoolVar(&allnodes, "all", false, "perform operation on all nodes in cluster")
	flag.BoolVar(&workers, "workers", false, "perform operation on nodes designated masters in cluster")
	flag.BoolVar(&masters, "masters", false, "perform operation on nodes designated workers in cluster")
	flag.StringVar(&single, "single", "", "specify a single remote host on which to run script or command")
	flag.StringVar(&script_name, "script", "", "specify path of script to run on remote host's shell\nthe script must be in the shared vagrant directory and relative to it")
	flag.StringVar(&cmd, "cmd", "", "specify a command to run on the remote host's shell")
	flag.StringVar(&connect, "c", "", "connect a machines stderr and stdout to terminal")
	flag.StringVar(&list, "l", "", "specify comma separated list of nodes to read stdout and stderr to terminal")

	flag.Parse()

	// read stdout and stdout of machines
	if list != "" {
		read_std_streams(list)
	}

	// connect stdout and stderr to terminal
	if connect != "" {
		m := find_machine(connect)
		if m.name != "" {
			fmt.Println("\n\t** Streaming from.. " + m.String() + "\n")
			m.streams.StreamOut()
		}
	}

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
