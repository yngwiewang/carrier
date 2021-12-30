# Carrier
A command-line tool similar to Ansible ad-hoc mode, much more efficient, implemented in Go.

Just a binary file, You don't need Python, don't need to install any software or libraries. One line of code can use the power of all CPU cores to execute shell commands concurrently on thousands of hosts. Get results in real time.

# Overview
Carrier does:
1. Execute shell commands on the remote hosts concurrently, display and record the results and time-consuming of the last execution.
2. Copy local file/directory to the remote hosts concurrently, record the result of the last execution.
3. Check the result of the last execution.
4. Extract the last successful/Failed hosts.

Use `carrier -h` for more information about commands.

# Usage

1. Config the settings in the config file, adjust the timeout to suit your environment.

2. Prepare a list of host, *host* is an example.

3. Execute commands concurrently. It is best to check your shell commands by using `--dry-run` and test them on a few hosts first.
```sh
$ ./carrier sh "echo -n 'hostname: ';hostname;echo -n 'cpu: ';cat /proc/cpuinfo |grep processor |wc -l;echo -n 'mem: ';cat /proc/meminfo |grep MemTotal |awk '{printf \"%d\n\", \$2/1024/1024}';echo -n 'disk: ';df -m|grep '/dev/'|grep -v tmpfs|awk '{sum+=\$2};END{printf \"%d\", sum/1024}'"
192.168.220.120 OK      0.104s
================================
hostname: kb1
cpu: 2
mem: 3
disk: 17

192.168.220.102 OK      0.146s
================================
hostname: docker
cpu: 2
mem: 3
disk: 18

192.168.220.1   Failed  1.003s
================================
dial tcp 192.168.220.1:22: i/o timeout
```

4. Check the result of last execution, the format could be table or csv.
```sh
$ ./carrier logs
+-----------------+-----------+------------------+----------------------------------------+----------+
| IP              | SUCCEEDED | RESULT           | ERROR                                  | DURATION |
+-----------------+-----------+------------------+----------------------------------------+----------+
| 192.168.220.102 | true      | hostname: docker |                                        |    0.146 |
|                 |           | cpu: 2           |                                        |          |
|                 |           | mem: 3           |                                        |          |
|                 |           | disk: 18         |                                        |          |
+-----------------+-----------+------------------+----------------------------------------+----------+
| 192.168.220.120 | true      | hostname: kb1    |                                        |    0.104 |
|                 |           | cpu: 2           |                                        |          |
|                 |           | mem: 3           |                                        |          |
|                 |           | disk: 17         |                                        |          |
+-----------------+-----------+------------------+----------------------------------------+----------+
| 192.168.220.1   | false     |                  | dial tcp 192.168.220.1:22: i/o timeout |    1.003 |
+-----------------+-----------+------------------+----------------------------------------+----------+
```
5. Extract the last successful/Failed hosts, you can redirect the result to a temp file and make further processing.
```sh
$ ./carrier hosts -sfalse
192.168.220.1,22,root,11111
```

6. Copy local file to the remote hosts concurrently. If the path is a directory, carrier will copy directories recursively and each file in the directory will be transferred concurrently. In order to avoid mistakes, basename of src and dst must be the same.
```sh
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