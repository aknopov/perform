# Problem Setup

Existing tools like GoLang benchmark tests provide real-time latency of the tested code. This might be a good in CPU-bound application.
However, in real life services that depend on disk I/O or networks this doesn't work well. Execution time might vary significantly depending on external factors.
CPU profiling is not reliable even when performance tests run in controlled environment.
Consider situation when you want to evaluate change in service performance after adding/removing some features. 
Add on top the situation when extracing code is not feasible or too costly.
How to measure performance change?

## Performance Test Setup


## Performance Test Requirements
1. The test should apply significan and sustainable load on the application. 
1. Test should be reproducible before and after feature change. Most likely that means that test set should not change.
1. The tests should be lengthy so that "warm-up" period can be neglected

## Example of use

This repository contains two extra application - "sample-server" and "sample-client".

The server hashes sent password with ARGON2 algorithm and sends the hash back. The request also contains extra parameters for number of iterations of the algorithm, thus allowing to control stress on the server.

The client sends the same request to the server in parallel using "perform" API. Upon completion client prints statistics number of tests, concurrency and reponses latency statistics.

These values to evaluate performance, however, as was discussed in the "Problem Setup", more reliable performance parameter is total CPU time.

### Host Machine

If the server is run directly on a host machine it can be obtained wih `top` command - 
```
$ go build -C sample-server -o ../server.prog
$ ./server.prog &
$ pid=$(ps -C server.prog -o pid=)
$ top -b -d1 -p $pid | grep --line-buffered $pid | tee server.log
```
The output looks like 
```
...
13838 user     20   0 2746296   1.3g   7268 R  1999   8.7  33:50.01 server.pr+
```
and the key value is in the second column from the right on the last line. It shows that total CPU time (User + Kernel) across all 20 processors is 33 min 50 sec.


### Docker

The server could be run in a Docker container. Unfortunately `docker stats` command does not provide cumulative CPU times - only CPU% that requires to sum up the column values to get performance metric.