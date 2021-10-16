package host

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/yngwiewang/carrier/common/color"
	"github.com/yngwiewang/carrier/pkg/config"
	"golang.org/x/crypto/ssh"
)

type Host struct {
	IP           string
	Port         string
	Username     string
	Password     string
	IsSucceeded  bool
	Result       string
	ExecDuration float64
	Error        string
}

type Hosts struct {
	HostSlice []*Host
	HostCh    chan *Host
}

func (hs *Hosts) SaveGob() error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(hs.HostSlice); err != nil {
		return err
	}
	f, err := os.Create("record")
	if err != nil && err != os.ErrExist {
		return err
	}
	w := bufio.NewWriter(f)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	w.Flush()
	return nil
}

func LoadGob() ([]*Host, error) {
	f, err := os.Open("record")
	if err != nil {
		return nil, err
	}
	var hs []*Host
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&hs); err != nil {
		return nil, err
	}
	return hs, nil
}

// GetHosts read the csv file and parse Hosts.
// Empty lines and lines starts with '#' will be ignored.
// The first column is remote ip, the second column is ssh port,
// the third column is ssh username(default is root), the forth column is password.
// The csv file must has at least the first column.
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
	return &Hosts{hosts, make(chan *Host)}, nil
}

// ExecuteSSH parallel execute ssh command on remote hosts.
func (hosts *Hosts) ExecuteSSH(cfg *config.Config, cmd string) {
	var wg sync.WaitGroup
	wg.Add(len(hosts.HostSlice))

	go func() {
		wg.Wait()
		close(hosts.HostCh)
	}()

	for _, h := range hosts.HostSlice {
		go func(h *Host, cmd string) {
			start := time.Now()
			h.exec(cfg, cmd)
			h.ExecDuration = math.Ceil(time.Since(start).Seconds()*1000) / 1000
			hosts.HostCh <- h
			wg.Done()
		}(h, cmd)
	}
}

// ExecuteSSH parallel execute ssh command on remote hosts.
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
			start := time.Now()
			h.copy(cfg, src, dst, mask)
			h.ExecDuration = math.Ceil(time.Since(start).Seconds()*1000) / 1000
			hosts.HostCh <- h
			wg.Done()
		}(h, src, dst, mask)
	}
	return nil
}

func (hosts Hosts) PrintResult() {
	for h := range hosts.HostCh {

		h.print()
	}
}

func (h *Host) print() {
	var state string
	if h.IsSucceeded {
		state = color.Green("OK")
	} else {
		state = color.Red("Failed ")
	}
	fmt.Printf("%-26s %-18s %.3fs\n", color.Yellow(h.IP), state, h.ExecDuration)
	fmt.Println("================================")
	if len(h.Result) > 0 {
		fmt.Println(h.Result)
	}
	if len(h.Error) > 0 {
		fmt.Println(h.Error)
	}
	fmt.Println()
}

// Conn wraps a net.Conn, and sets a deadline for every read
// and write operation.
type Conn struct {
	net.Conn
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (c *Conn) Read(b []byte) (int, error) {
	err := c.Conn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	if err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}

func (c *Conn) Write(b []byte) (int, error) {
	err := c.Conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	if err != nil {
		return 0, err
	}
	return c.Conn.Write(b)
}

func sshDialTimeout(network, addr string, config *ssh.ClientConfig, timeout time.Duration) (*ssh.Client, error) {
	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}

	timeoutConn := &Conn{conn, timeout, timeout}
	c, chans, reqs, err := ssh.NewClientConn(timeoutConn, addr, config)
	if err != nil {
		return nil, err
	}
	client := ssh.NewClient(c, chans, reqs)

	// this sends keepalive packets every 2 seconds
	// there's no useful response from these, so we can just abort if there's an error
	// go func() {
	// 	t := time.NewTicker(2 * time.Second)
	// 	defer t.Stop()
	// 	for range t.C {
	// 		_, _, err := client.Conn.SendRequest("keepalive", true, nil)
	// 		if err != nil {
	// 			return
	// 		}
	// 	}
	// }()
	return client, nil
}

func (h *Host) exec(cfg *config.Config, cmd string) {
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
			h.Error = err.Error()
			return
		}
		key, err := ioutil.ReadFile(filepath.Join(homeDir, ".ssh/id_rsa"))
		if err != nil {
			h.Error = err.Error()
			return
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			h.Error = err.Error()
			return
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
	// client, err := ssh.Dial("tcp", h.IP+":"+h.Port, config)
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

	defer session.Close()
	var res bytes.Buffer
	session.Stdout = &res

	err = session.Run(cmd)
	if err != nil {
		h.Result = strings.TrimSpace(res.String())
		h.Error = err.Error()
		return
	}
	h.IsSucceeded = true
	h.Result = strings.TrimSpace(res.String())
}

func (h *Host) copy(cfg *config.Config, src, dst, mask string) {
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
			h.Error = err.Error()
			return
		}
		key, err := ioutil.ReadFile(filepath.Join(homeDir, ".ssh/id_rsa"))
		if err != nil {
			h.Error = err.Error()
			return
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			h.Error = err.Error()
			return
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

	client := scp.NewClient(h.IP+":"+h.Port, config)

	err := client.Connect()
	if err != nil {
		h.Error = err.Error()
		return
	}

	file, err := os.Open(src)
	if err != nil {
		h.Error = err.Error()
		return
	}

	defer client.Close()
	defer file.Close()

	if filepath.Base(src) != filepath.Base(dst) {
		h.Error = "basename of src and dst must be the same"
		return
	}
	err = client.CopyFromFile(*file, dst, mask)
	if err != nil {
		h.Error = err.Error()
		return
	}
	h.IsSucceeded = true
	h.Result = "OK"
}
