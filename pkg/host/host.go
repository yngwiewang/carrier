package host

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/yngwiewang/carrier/pkg/color"
	"github.com/yngwiewang/carrier/pkg/config"
	"golang.org/x/crypto/ssh"
)

type Host struct {
	IP           string
	Port         string
	Username     string
	Password     string
	Succeed      bool
	Result       string
	Stdout       string
	Stderr       string
	ExecDuration time.Duration
	Error        string
}

type Hosts struct {
	HostSlice []*Host
	HostCh    chan *Host
}

// SaveGob serialize the result of the last execution to a gob file
// and save it in /tmp/carrier_log.
func (hs *Hosts) SaveGob() error {
	f, err := os.Create("/tmp/carrier_log")
	if err != nil && err != os.ErrExist {
		return err
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	if err := enc.Encode(hs.HostSlice); err != nil {
		return err
	}
	return nil
}

// LoadGob read the gob file /tmp/carrier_log and deserialize it to []*Host.
func LoadGob() ([]*Host, error) {
	f, err := os.Open("/tmp/carrier_log")
	defer f.Close()
	if err != nil {
		return nil, errors.New("open /tmp/carrier_log: no such file or directory")
	}
	var hs []*Host
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&hs); err != nil {
		return nil, err
	}
	return hs, nil
}

// GetHosts read the csv file and parse it's content to []*Host then init a *Hosts.
// Empty lines and lines starts with '#' will be ignored.
// The first column is remote ip, the second column is ssh port,
// the third column is ssh username(default is root), the forth column is password.
// The csv file must has at least the first column, then the default value of port
// and username will be 22 and root.
func GetHosts(fileName string) (*Hosts, error) {
	var hosts []*Host
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || line[0] == '#' {
			continue
		}
		items := strings.Split(line, ",")
		host := &Host{
			IP:       items[0],
			Port:     "22",
			Username: "root",
		}
		if len(items) > 1 {
			host.Port = items[1]
		}
		if len(items) > 2 {
			host.Username = items[2]
		}
		if len(items) > 3 {
			host.Password = items[3]
		}
		hosts = append(hosts, host)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Hosts{hosts, make(chan *Host, 0)}, nil
}

// ExecuteSSH concurrently execute ssh command on remote hosts and print result.
func (hosts *Hosts) ExecuteSSH(cfg *config.Config, cmd string) {
	var wg sync.WaitGroup
	wg.Add(len(hosts.HostSlice))

	go func() {
		wg.Wait()
		close(hosts.HostCh)
	}()

	for _, h := range hosts.HostSlice {
		go func(h *Host, cmd string) {
			defer wg.Done()
			start := time.Now()
			h.exec(cfg, cmd)
			h.ExecDuration = time.Since(start)
			hosts.HostCh <- h
		}(h, cmd)
	}
}

// ExecuteSCP concurrently copy file/directory to remote hosts.
func (hosts *Hosts) ExecuteSCP(cfg *config.Config, src, dst, mask string) error {
	if src == "" || dst == "" {
		return errors.New("must specify src and dst")
	}
	var wg sync.WaitGroup
	wg.Add(len(hosts.HostSlice))

	go func() {
		wg.Wait()
		close(hosts.HostCh)
	}()

	for _, h := range hosts.HostSlice {
		go func(h *Host, src, dst, mask string) {
			defer wg.Done()
			start := time.Now()
			h.copy(cfg, src, dst, mask)
			h.ExecDuration = time.Since(start)
			hosts.HostCh <- h
		}(h, src, dst, mask)
	}
	return nil
}

// PrintResult fetch the executed hosts from channel and print their results.
// The results will be printed immediately after execution.
func (hosts Hosts) PrintResult() {
	for h := range hosts.HostCh {
		h.print()
	}
}

func (h *Host) print() {
	var state string
	if h.Succeed {
		state = color.Green("OK")
	} else {
		state = color.Red("Failed ")
	}
	fmt.Printf("%-26s %-18s %s\n", color.Yellow(h.IP), state, h.ExecDuration)
	fmt.Println("================================")
	if len(h.Stdout) > 0 {
		fmt.Println(h.Stdout)
	}
	if len(h.Stderr) > 0 {
		fmt.Println(h.Stderr)
	}
	if len(h.Error) > 0 {
		fmt.Println(h.Error)
	}
	fmt.Println()
}

func getHostSSHConfig(cfg *config.Config, h *Host) (*ssh.ClientConfig, error) {
	var config *ssh.ClientConfig
	if cfg.AuthMode == "password" {
		config = &ssh.ClientConfig{
			User: h.Username,
			Auth: []ssh.AuthMethod{
				ssh.Password(h.Password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         cfg.ExecuteTimeout,
		}
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		key, err := ioutil.ReadFile(filepath.Join(homeDir, ".ssh/id_rsa"))
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		config = &ssh.ClientConfig{
			User: h.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         cfg.ExecuteTimeout,
		}
	}
	return config, nil
}

func (h *Host) exec(cfg *config.Config, cmd string) {
	config, err := getHostSSHConfig(cfg, h)
	if err != nil {
		h.Error = err.Error()
		return
	}
	client, err := sshDialTimeout("tcp", h.IP+":"+h.Port, config, cfg.ExecuteTimeout)
	if err != nil {
		h.Error = err.Error()
		return
	}

	session, err := client.NewSession()
	if err != nil {
		h.Error = err.Error()
		return
	}
	stdoutPipe, _ := session.StdoutPipe()
	stderrPipe, _ := session.StderrPipe()
	defer session.Close()

	err = session.Run(cmd)
	if err != nil {
		h.Error = err.Error()
	}
	stdout, _ := ioutil.ReadAll(stdoutPipe)
	h.Stdout = strings.TrimSpace(string(stdout))
	stderr, _ := ioutil.ReadAll(stderrPipe)
	h.Stderr = strings.TrimSpace(string(stderr))

	h.Succeed = true
}

func (h *Host) copy(cfg *config.Config, src, dst, mask string) {
	fi, err := os.Stat(src)
	if err != nil {
		h.Error = err.Error()
		return
	}

	var wg sync.WaitGroup
	switch mode := fi.Mode(); {
	// scp a file
	case mode.IsRegular():
		if filepath.Base(src) != filepath.Base(dst) {
			h.Error = "basename of src and dst must be the same"
			return
		}
		wg.Add(1)
		h.copyFile(cfg, &wg, src, dst, mask)
		return
	// scp a directory
	case mode.IsDir():
		h.exec(cfg, fmt.Sprintf("mkdir -p %s;chmod %s %s", dst, mask, dst))
		if h.Error != "" {
			return
		}
		h.copyDir(cfg, &wg, src, dst, mask)
		wg.Wait()
		return
	}
}

func (h *Host) copyFile(cfg *config.Config, wg *sync.WaitGroup, src, dst, mask string) {
	defer wg.Done()
	sshConfig, err := getHostSSHConfig(cfg, h)
	if err != nil {
		// If copying a directory, the "Succeed" flag will be True if "mkdir" succeed.
		h.Succeed = false
		h.Error = err.Error()
		return
	}
	file, err := os.Open(src)
	if err != nil {
		h.Succeed = false
		h.Error = err.Error()
		return
	}
	defer file.Close()

	client := scp.NewClient(h.IP+":"+h.Port, sshConfig)
	err = client.Connect()
	if err != nil {
		h.Succeed = false
		h.Error = err.Error()
		return
	}
	defer client.Close()

	err = client.CopyFromFile(*file, dst, mask)
	if err != nil {
		h.Succeed = false
		h.Error = err.Error()
		return
	}
	h.Succeed = true

	return
}

func (h *Host) copyDir(cfg *config.Config, wg *sync.WaitGroup, src, dst, mask string) {
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		h.Error = err.Error()
		return
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		fi, err := os.Stat(srcPath)
		if err != nil {
			h.Error = err.Error()
			return
		}

		switch fi.Mode() & os.ModeType {
		case os.ModeDir:
			h.exec(cfg, fmt.Sprintf("mkdir -p %s;chmod %s %s", dstPath, mask, dstPath))
			if h.Error != "" {
				return
			}
			h.copyDir(cfg, wg, srcPath, dstPath, mask)
			if h.Error != "" {
				return
			}
		default:
			wg.Add(1)
			h.copyFile(cfg, wg, srcPath, dstPath, mask)
		}
	}
}
