# Carrier
A command-line tool similar to Ansible ad-hoc mode, much more efficient, implemented in Go.

Just a binary file, You don't need Python, don't need to install any software or libraries. One line of code to execute shell commands on thousands of hosts parallelly. Get results in real time.

# Overview
Carrier does:
1. Executes shell commands on the remote hosts parallelly, record the result of the last execution.
2. Copy local file to the remote hosts parallelly, record the result of the last execution.
3. Check the result of the last execution.
4. Extract the last successful/Failed hosts.

Use `carrier -h` for more information about commands.

# Usage

1. Config the settings in the config file, adjust the timeout to suit your environment.

2. Prepare a list of host, *host* is an example.

3. Execute commands parallelly. Note that multiple statements are separated by semicolons. It is best to test your shell commands on one or two hosts first.
```
$ ./carrier sh 'hostname;date'
192.168.220.120 OK      0.154s
================================
kb1
Wed Sep 15 17:56:59 CST 2021

192.168.220.102 OK      0.210s
================================
docker
Mon Oct 11 05:49:18 CST 2021

192.168.220.1   Failed  1.002s
================================
dial tcp 192.168.220.1:22: i/o timeout
```

4. Check the result of last execution, the format could be table or csv.
```
$ ./carrier logs
+-----------------+-----------+------------------------------+----------------------------------------+----------+
| IP              | SUCCEEDED | RESULT                       | ERROR                                  | DURATION |
+-----------------+-----------+------------------------------+----------------------------------------+----------+
| 192.168.220.102 | true      | docker                       |                                        |     0.21 |
|                 |           | Mon Oct 11 05:49:18 CST 2021 |                                        |          |
+-----------------+-----------+------------------------------+----------------------------------------+----------+
| 192.168.220.120 | true      | kb1                          |                                        |    0.154 |
|                 |           | Wed Sep 15 17:56:59 CST 2021 |                                        |          |
+-----------------+-----------+------------------------------+----------------------------------------+----------+
| 192.168.220.1   | false     |                              | dial tcp 192.168.220.1:22: i/o timeout |    1.002 |
+-----------------+-----------+------------------------------+----------------------------------------+----------+
```
5. Extract the last successful/Failed hosts, you can redirect the result to a temp file and make further processing.
```
$ ./carrier hosts -sfalse
192.168.220.1,22,root,11111
```

6. Copy local file to the remote hosts parallelly. In order to avoid mistakes, basename of src and dst must be the same.
```
$ ./carrier cp -s /mnt/d/abc -d /root/test/abc -m 0644
192.168.220.120 Failed  0.161s
================================
scp: /root/test/abc: No such file or directory


192.168.220.102 OK      0.221s
================================
OK
```

# Benchmark
Here is an approximate benchmark for reference. Results are affected by hosts and network environment. According to the scale of concurrency, the CPU of the machine initiating the ssh command also greatly affects performance.
- shelll command: hostname
- hosts: distributed in 2 IDCs more than 1200 kilometers apart

| Hosts | Duration |
| ----- | -------- |
| 1000  | 2.5s     |
| 5000  | 40s      |
| 10000 | 70s      |
| 27000 | 2m       |

Resource consumption of the machine that sending the command to 27000 hosts
- cpu: exhaust 4 virtual cores(Intel Xeon @ 2.00GHz)
- mem: 1.8G

# Credits
- [github.com/bramvdbogaerde/go-scp](https://github.com/bramvdbogaerde/go-scp)
- [github.com/spf13/cobra](https://github.com/spf13/cobra)
- [github.com/jedib0t/go-pretty](https://github.com/jedib0t/go-pretty)