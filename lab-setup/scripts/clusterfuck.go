// clusterfuck is a simple go program to execute commands/scripts
// on a cluster of machines concurrently using the ssh protocol
// instead of the boring way of logging into each machine individually
// and doing manually the mundane tasks. I use snake_case in this file
// alot because its a script and sometimes i like doing things that are
// contrary to accepted norms just to see what it feels like to be on the
// sideline.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.comn/mikkeloscar/sshconfig"
)

// node_type represents the role of node a node in the cluster or
// it identifies what types of nodes to perform an operation on
//go:generate stringer -type=node_type
type node_type uint8

const (
	UNDEFINED node_type = iota
	ALL                 // rall masters and controllers in the cluster
	MASTER
	WORKER
	GATEWAY
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
	host    *sshconfig.SSHHost
}

type std_stream struct {
	connected    bool
	stdout       io.Reader
	in, out, err io.ReadWriteCloser
}

func split_path(path string) []string {
	return strings.Split(path, string(filepath.Separator))
}

// this mkdirp is very wrong and might only work in this settin
func mkdirp(path string, perm os.FileMode) error {
	dirs := split_path(path)
	d := split_path(out_dir)
	name := filepath.Base(out_dir)
	var idx int
	for i := len(d) - 1; i >= 0; i-- {
		if d[i] == name {
			idx = i
			break
		}
	}
	cpath := out_dir
	for _, s := range dirs[idx:] {
		if s != name {
			cpath = filepath.Join(cpath, s)
		}
		if err := os.Mkdir(cpath, perm); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return err
		}
	}
	return nil
}

// open files to be used as stdin, stout, stderr for virtual machine
func new_file_stream(dir, file string) *os.File {
	ndir, _ := filepath.Split(file)
	ndir = filepath.Join(dir, ndir)
	_, err := os.Stat(ndir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Fatal(err)
		}

		if err := mkdirp(ndir, 0644); err != nil {
			log.Fatal(err)
		}
	}
	f, err := os.OpenFile(filepath.Join(dir, file), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

// open stream for reading or writing without truncating the old
// contents of the file. this is useful for reading or writing a file
// after its operations are done
func open_stream(dir, file string) *os.File {
	f, err := os.OpenFile(filepath.Join(dir, file), os.O_RDONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

// paths returns a function with a parent path that you can add multiple other
// children files to.
func paths(parent string) func(string) string {
	return func(file string) string {
		return filepath.Join(parent, file)
	}
}

// open file streams on the host machine connecting the standard streams of the
// remote machine to the files
func (s *std_stream) get_new_streams(hostname string) {
	file := paths(hostname)
	s.in = new_file_stream(out_dir, file("stdin"))
	s.out = new_file_stream(out_dir, file("stdout"))
	s.err = new_file_stream(out_dir, file("stderr"))
}

// return a new stream to connect to remote host's std streams
func new_streams(hostname string) *std_stream {
	s := &std_stream{}
	s.get_new_streams(hostname)
	return s
}

// dummy makes an io.Writer an io.ReadWriteCloser
type dummy struct{ w io.Writer }

func (dummy) Close() error                  { return nil }
func (dummy) Read(p []byte) (int, error)    { return 0, nil }
func (d dummy) Write(p []byte) (int, error) { return d.w.Write(p) }

// connect machine's stdout file to the terminal stdout
func (s *std_stream) StreamOut() {
	r, w := io.Pipe()
	s.stdout = r
	s.out = dummy{io.MultiWriter(w, s.out)}
	s.connected = true
}

// connect machine's stdin file to the terminal stdin
func (s *std_stream) WriteIn() {}

// connect machine's stderr file to the terminal stderr
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

func machine_from_sshconfig(host *sshconfig.SSHHost) machine {
	return machine{}
}

func makeCmd(command string, streams *std_stream, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdin = streams.in
	cmd.Stdout = streams.out
	cmd.Stderr = streams.err
	return cmd
}

// prepare and ssh into specified machine
func ssh(mach machine, args ...string) error {
	cmd := makeCmd("ssh", mach.streams, append([]string{mach.name}, args...)...)
	if debug {
		fmt.Println("\t ** Executing command in node.. " + mach.String())
	}
	if mach.streams.connected {
		go io.Copy(os.Stdout, mach.streams.stdout)
	}
	return cmd.Run()
}

//go:generate stringer -type=Op
type Op uint8

const (
	START Op = iota
	STOP
	PAUSE
	RESUME
	SAVESTATE
	RESET
	ACPIPOWEROFF
	NOP
)

func vbox(m machine, op string) error {
	var args []string

	makeArg := func(args ...string) []string {
		return append([]string{}, args...)
	}

	switch getOp(op) {
	case START:
		args = makeArg("startvm", m.name, "--type", "headless")
	case STOP:
		args = makeArg("controlvm", m.name, "poweroff")
	case SAVESTATE:
		args = makeArg("controlvm", m.name, "savestate")
	case PAUSE:
		args = makeArg("controlvm", m.name, "pause")
	case RESUME:
		args = makeArg("controlvm", m.name, "resume")
	case ACPIPOWEROFF:
		args = makeArg("controlvm", m.name, "acpipowerbutton")
	case RESET:
		args = makeArg("controlvm", m.name, "reset")
	default:
		log.Fatal(op, " not supported")
	}

	// fmt.Println(args)
	cmd := makeCmd("VBoxManage", m.streams, args...)
	return cmd.Run()
}

func getOp(op string) Op {
	switch op {
	case "start":
		return START
	case "stop":
		return STOP
	case "pause":
		return PAUSE
	case "resume":
		return RESUME
	case "reset":
		return RESET
	case "savestate":
		return SAVESTATE
	case "acpipoweroff":
		return ACPIPOWEROFF
	}
	return NOP
}

// path of the script in the virtual machine
func script_path(name string) string {
	return "/vagrant/" + name
}

type Cmd uint8

const (
	SSH Cmd = iota
	VBOX
)

// execute's script/command on specified machine types
func exec_on(kind node_type, c Cmd, args string) {
	for _, node := range cluster {
		if node.kind == kind || kind == ALL {
			wg.Add(1)
			go func(node machine) {
				defer wg.Done()
				switch c {
				case SSH:
					ssh(node, args)
				case VBOX:
					vbox(node, args)
				}
			}(node)
		}
	}
	wg.Wait()
}

// execute script/command on master nodes
func exec_on_masters(cmd Cmd, arg string) {
	exec_on(MASTER, cmd, arg)
}

// execute script/command on worker nodes
func exec_on_workers(cmd Cmd, arg string) {
	exec_on(WORKER, cmd, arg)
}

// execute script/command on all nodes
func exec_on_all(cmd Cmd, arg string) {
	exec_on(ALL, cmd, arg)
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
func printStdout(m machine) {
	stdout := open_stream(out_dir, filepath.Join(m.name, "stdout"))
	io.Copy(os.Stdout, stdout)
}

func read_std_streams(machs string) {
	ms := strings.Split(machs, ",")
	for _, m := range ms {
		mach := find_machine(m)
		if mach.name != "" {
			fmt.Println("found machine " + mach.String())
			printStdout(mach)
		}
	}
}

func get_ssh_config() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(homedir, ".ssh", "config")
}

var (
	wg sync.WaitGroup

	// shared directory of the vagrant working directory
	//shared_directory = "C:\\Users\\big yeti\\linux-networking\\lab-setup"
	shared_dir string

	// where to keep all the file streams connected to the remote machine's
	// standard streams
	out_dir string

	// ssh config
	ssh_config string

	// command line argument variables
	debug       bool
	allnodes    bool
	masters     bool
	workers     bool
	vm          string
	single      string
	script_name string
	cmd         string
	connect     string
	list        string

	// cluster contains all vagrant virtual machines in the cluster
	cluster []machine
)

func main() {
	flag.BoolVar(&debug, "v", false, "verbose output")
	flag.BoolVar(&allnodes, "all", false, "perform operation on all nodes in cluster")
	flag.BoolVar(&workers, "workers", false, "perform operation on nodes designated workers in cluster")
	flag.BoolVar(&masters, "masters", false, "perform operation on nodes designated masters in cluster")
	flag.StringVar(&vm, "vm", "", "specify vboxmanage command for vms in cluster")
	flag.StringVar(&single, "single", "", "specify a single remote host on which to run script or command")
	flag.StringVar(&script_name, "script", "", "specify path of script to run on remote host's shell\nthe script must be in the shared vagrant directory and relative to it")
	flag.StringVar(&cmd, "cmd", "", "specify a command to run on the remote host's shell")
	flag.StringVar(&connect, "c", "", "connect a machines stderr and stdout to terminal")
	flag.StringVar(&list, "l", "", "specify comma separated list of nodes to read stdout and stderr to terminal")
	flag.StringVar(&shared_dir, "s", "", "specify the path of the vagrant directory")
	flag.StringVar(&out_dir, "o", "", "specify path of output streams of remote host machines")
	flag.StringVar(&ssh_config, "ssh_config", "", "specify path to openssh config file")

	flag.Parse()

	var err error
	if shared_dir == "" {
		flag.Usage()
		os.Exit(1)
	} else {
		if !filepath.IsAbs(shared_dir) {
			if shared_dir, err = filepath.Abs(shared_dir); err != nil {
				log.Fatal(err)
			}
		}
	}

	if out_dir == "" {
		out_dir = filepath.Join(shared_dir, "outputs")
	} else {
		if !filepath.IsAbs(out_dir) {
			if out_dir, err = filepath.Abs(out_dir); err != nil {
				log.Fatal(err)
			}
		}
	}

	if ssh_config == "" {
		ssh_config = get_ssh_config()
	}
	log.Fatal(ssh_config)

	// read stdout and stdout of machines
	if list != "" {
		read_std_streams(list)
		return
	}

	// cluster = append(cluster, []machine{
	// 	mach("master0", MASTER), mach("master1", MASTER), mach("master2", MASTER),
	// 	mach("worker0", WORKER), mach("worker1", WORKER), mach("worker2", WORKER),
	// 	mach("worker3", WORKER), mach("worker4", WORKER), mach("worker5", WORKER),
	// 	mach("gateway", GATEWAY),
	// }...)

	// run vboxmanage operation
	if vm != "" {
		if single != "" {
			vbox(mach(single, UNDEFINED), vm)
			return
		}
		if allnodes {
			exec_on_all(VBOX, vm)
		} else if masters {
			exec_on_masters(VBOX, vm)
		} else if workers {
			exec_on_workers(VBOX, vm)
		}
		return
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
			exec_on_all(SSH, script_path(script_name))
		} else if masters {
			exec_on_masters(SSH, script_path(script_name))
		} else if workers {
			exec_on_workers(SSH, script_path(script_name))
		}
	}

	// execute a command or set of commands
	if cmd != "" {
		if single != "" {
			ssh(mach(single, UNDEFINED), cmd)
			return
		}

		if allnodes {
			exec_on_all(SSH, cmd)
		} else if masters {
			exec_on_masters(SSH, cmd)
		} else if workers {
			exec_on_workers(SSH, cmd)
		}
	}
}
