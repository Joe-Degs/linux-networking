// clusterfuck is a simple go program to execute commands/scripts
// on a cluster of machines concurrently using the ssh protocol
// instead of the boring way of logging into each machine individually
// and doing manually the mundane tasks. I use snake_case in this file
// alot because its a script and sometimes i like doing things that are
// contrary to accepted norms just to see what it feels like to be on the
// sideline.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/DavidGamba/go-getoptions"
	"github.com/mikkeloscar/sshconfig"
)

type (
	// node_type represents the role of node a node in the cluster or
	// it identifies what types of nodes to perform an operation on
	//go:generate stringer -type=node_type
	node_type uint8

	// machine holds the hostname and files representing stdin, stdout and stdout of
	// a remote host in the cluster. This is done because writing the the std's
	// concurrently fucks up my terminal. The name of the machine should also be
	// in the .ssh/config file of your host, because it's used to ssh into the
	// remote host
	machine struct {
		name    string
		kind    node_type
		streams *std_stream
		config  *sshconfig.SSHHost
	}

	std_stream struct {
		connected    bool
		stdout       io.Reader
		in, out, err io.ReadWriteCloser
	}

	// dummy makes an io.Writer an io.ReadWriteCloser
	dummy struct{ w io.Writer }

	//go:generate stringer -type=Op
	Op uint8

	// A command is a command that can be executed
	Command interface {
		// run the command
		Run(args []string) error

		// display help and quit the program
		Help()
	}

	// the list command type. it lists things right
	List struct {
		args string
	}

	// vbox command options
	Opts struct {
		Connect string
		Type    string
		Script  string
		Command string
		Name    string
	}

	// vboxmanage command executor
	SshCmd struct {
		opt  *getoptions.GetOpt
		opts *Opts
	}

	// ssh command executor
	VboxCmd struct {
		opt  *getoptions.GetOpt
		opts *Opts
	}

	CmdLineCmd struct {
		keys  []string
		funcs map[string]func([]string) error
	}

	Cmd uint8
)

func (c *CmdLineCmd) register(key string, val Command) {
	c.keys = append(c.keys, key)
	c.funcs[key] = val.Run
}

func (c *CmdLineCmd) register_func(key string, val func()) {
	f := func(_ []string) error {
		val()
		return nil
	}
	c.keys = append(c.keys, key)
	c.funcs[key] = f
}

func (c *CmdLineCmd) Help() {
	fmt.Printf("Commands: %s\n", strings.Join(c.keys, " "))
	fmt.Printf("'cmd help' for help for specific command\n")
}

func (c *CmdLineCmd) get(key string) (func([]string) error, bool) {
	f, ok := c.funcs[key]
	return f, ok
}

// node types
const (
	UNDEFINED node_type = iota
	ALL                 // rall masters and controllers in the cluster
	MASTER
	WORKER
	GATEWAY
)

// vboxmanage commands
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

// types of commands to execute. might not need this for too long as a better
// way is making its way out of my guts
const (
	SSH Cmd = iota
	VBOX
)

func (dummy) Close() error                  { return nil }
func (dummy) Read(p []byte) (int, error)    { return 0, nil }
func (d dummy) Write(p []byte) (int, error) { return d.w.Write(p) }

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
func mach(hostname string, hosttype node_type) *machine {
	return &machine{
		name:    hostname,
		kind:    hosttype,
		streams: new_streams(hostname),
	}
}

func sshconfig_cluster(hosts []*sshconfig.SSHHost) {
	for _, h := range hosts {
		m := &machine{
			name:   h.Host[0],
			kind:   UNDEFINED,
			config: h,
		}
		cluster = append(cluster, m)
	}
}

func makeCmd(command string, streams *std_stream, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdin = streams.in
	cmd.Stdout = streams.out
	cmd.Stderr = streams.err
	return cmd
}

// prepare and ssh into specified machine
func ssh(mach *machine, args ...string) error {
	cmd := makeCmd("ssh", mach.streams, append([]string{mach.name}, args...)...)
	if debug {
		fmt.Println("\t ** Executing command in node.. " + mach.String())
	}
	if mach.streams.connected {
		go io.Copy(os.Stdout, mach.streams.stdout)
	}
	return cmd.Run()
}

func vbox(m *machine, op string) error {
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

func ListCommand(args string) List {
	return List{args: args}
}

func (l List) Run() error {
	fmt.Println(l.args)
	return nil
}

func (l List) Help() {
	log.Fatal("list command help!")
}

// execute's script/command on specified machine types
func exec_on(kind node_type, c Cmd, args string) {
	for _, node := range cluster {
		if node.kind == kind || kind == ALL {
			wg.Add(1)
			go func(node *machine) {
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

func find_machine(name string) *machine {
	for _, m := range cluster {
		if m.name == name {
			return m
		}
	}
	return nil
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
	debug      bool
	list       string

	// cluster contains all vagrant virtual machines in the cluster
	cluster []*machine

	// all the commands supported
	commands = &CmdLineCmd{funcs: make(map[string]func([]string) error)}
)

func (o *Opts) Clear() {
	o.Type = ""
	o.Command = ""
	o.Script = ""
	o.Connect = ""
}

func New(cmd Command) Command {
	var opts Opts
	opt := getoptions.New()

	opt.Bool("help", false, opt.Alias("h", "?"))

	opt.StringVar(&opts.Connect, "connect", "", opt.Alias("c"), opt.Description("connect machine stderr and stdout to terminal"))
	opt.StringVar(&opts.Type, "type", "", opt.Alias("t"), opt.Description("specify type of machine to run script or command on"))
	opt.StringVar(&opts.Script, "script", "", opt.Alias("s"), opt.Description("specify script the script to run"))
	opt.StringVar(&opts.Command, "command", "", opt.Alias("cmd"), opt.Description("specify command the script to run"))
	opt.StringVar(&opts.Name, "name", "", opt.Alias("n"), opt.Description("specify machine when executing on single machine"))

	switch cmd.(type) {
	case *SshCmd:
		return &SshCmd{
			opts: &opts,
			opt:  opt,
		}
	case *VboxCmd:
		return &VboxCmd{
			opts: &opts,
			opt:  opt,
		}
	}
	return nil
}

func (v *VboxCmd) Help() {
	return
}

func (v *SshCmd) Help() {
	return
}

func (v *VboxCmd) Run(args []string) error {
	v.opts.Clear()
	largs, err := v.opt.Parse(args)
	if err != nil {
		return err
	}

	if v.opt.Called("help") {
		fmt.Fprintln(os.Stderr, v.opt.Help())
		os.Exit(1)
	}

	vm := strings.Join(largs, " ")
	log.Println(vm)

	if v.opts.Type == "" {
		vbox(find_machine(v.opts.Name), vm)
	} else if v.opts.Type == "all" {
		exec_on_all(VBOX, vm)
	} else if v.opts.Type == "master" {
		exec_on_masters(VBOX, vm)
	} else if v.opts.Type == "worker" {
		exec_on_workers(VBOX, vm)
	}

	return nil
}

func (v *SshCmd) Run(args []string) error {
	v.opts.Clear()
	if _, err := v.opt.Parse(args); err != nil {
		return err
	}

	if v.opt.Called("help") {
		fmt.Fprintln(os.Stderr, v.opt.Help())
		os.Exit(1)
	}

	// connect stdout and stderr to terminal
	if v.opts.Connect != "" {
		m := find_machine(v.opts.Connect)
		if m.name != "" {
			fmt.Println("\n\t** Streaming from.. " + m.String() + "\n")
			m.streams.StreamOut()
		}
	}

	// execute a script
	if v.opts.Script != "" {
		if v.opts.Type == "" {
			ssh(find_machine(v.opts.Name), script_path(v.opts.Script))
		} else if v.opts.Type == "all" {
			exec_on_all(SSH, script_path(v.opts.Script))
		} else if v.opts.Type == "master" {
			exec_on_masters(SSH, script_path(v.opts.Script))
		} else if v.opts.Type == "workers" {
			exec_on_workers(SSH, script_path(v.opts.Script))
		}
	}

	// execute a command or set of commands
	if v.opts.Command != "" {
		if v.opts.Type != "" {
			ssh(find_machine(v.opts.Name), v.opts.Command)
			return nil
		}

		if v.opts.Type == "all" {
			exec_on_all(SSH, v.opts.Command)
		} else if v.opts.Type == "master" {
			exec_on_masters(SSH, v.opts.Command)
		} else if v.opts.Type == "workers" {
			exec_on_workers(SSH, v.opts.Command)
		}
	}

	return nil
}

func main() {
	flag.BoolVar(&debug, "v", false, "verbose output")
	flag.StringVar(&list, "list", "", "specify comma separated list of nodes to read stdout and stderr to terminal")
	flag.StringVar(&shared_dir, "shared", "", "specify the path of the vagrant directory")
	flag.StringVar(&out_dir, "output", "", "specify path of output streams of remote host machines")
	flag.StringVar(&ssh_config, "sshconfig", "", "specify path to openssh config file")

	flag.Parse()

	// get ssh config path
	if ssh_config == "" {
		ssh_config = get_ssh_config()
	}

	// parse the ssh config file
	hosts := sshconfig.MustParse(ssh_config)
	sshconfig_cluster(hosts)

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

	prompt := func() {
		fmt.Print(os.Args[0], "> ")
	}

	clear := func() {
		var cmd string
		if runtime.GOOS == "windows" {
			cmd = "cls"
		} else if runtime.GOOS == "linux" {
			cmd = "clear"
		}
		log.Println("execing... ", cmd)
		c := exec.Command(cmd)
		c.Stdout = os.Stdout
		c.Run()
	}

	get_args := func(s string) (cmd string, args []string) {
		out := strings.Split(strings.TrimSpace(strings.ToLower(s)), " ")
		cmd = out[0]
		if len(cmd) >= 1 {
			args = out[1:]
		}
		return
	}

	commands.register_func("clear", clear)
	commands.register_func("help", commands.Help)
	commands.register_func("quit", clear)
	commands.register("ssh", New(&SshCmd{}))
	commands.register("vbox", New(&VboxCmd{}))

	cmdline := bufio.NewScanner(os.Stdin)
	prompt()
	for cmdline.Scan() {
		cmd, args := get_args(cmdline.Text())
		if f, ok := commands.get(cmd); ok {
			if err := f(args); err != nil {
				log.Fatal(err)
			}
		}
		prompt()
	}
}
