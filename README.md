# Problem Setup

Consider a situation where you want to evaluate the performance impact of code change in a service. In situations when extracting code for isolated testing is not feasible or require significant refactoring, the only resort would be integration testing. Most likely than not, the tested service is not alone and depends on others. While waiting on response, the service does not consume CPU, making most common approach for measuring response latencies (Go benchmarking, Gatling for Java) skewed. The purpose of this project was to evaluate common metrics available across most of operating systems and CPU architectures that can provide meaningful performance insights and be used in CI/CD pipeline.

First candidates for these metrics were:
- Cumulative CPU time of the process, like output of `top -b -d1 -p $pid` on Linux.
- Sum of CPU% values of the process (i.e., integral over time) that is suitable for Docker containers.
- Count of CPU cycles/instructions. On Linux, this metric can be obtained using the `perf stat ...` command.

Measurements on actual services showed that all the above metrics have statistical errors. Since CI/CD pipeline would run most likely once, it brings us back to the question of using response latencies. The solution would be using t-test statistics of response latencies before and after code change (see https://pkg.go.dev/golang.org/x/perf/cmd/benchstat).

## Test Setup

This project provides function `RunTest` that applies an even load on the tested service/code. The function starts executing a number of tasks simultaneously at the beginning and then maintains the same number concurrent tasks till the test end. In contrary to regular benchmarking approach, the function allows to benchmark different functionalities in one run.

The function takes the following arguments:
- `tasks` - a list of tasks to run (e.g. requests to an external server)
- `totalRuns` - total number of tests to run
- `concurrent` - the number of tests to run in parallel
  
It is not required but assumed that length of `tasks` is a multiple of `totalRuns`. In this case number of each task executions would be `totalRuns / len(tasks)`

`RunTest` returns separate statistics for each task, including:
- Invocation count
- Fail count
- Average, median, minimum, and maximum values
- Standard deviation
- Raw values of response latencies

Separating statistics is done to allow extending the set of tasks as the service evolves.

## Sample Applications

The project includes:
- A [sample server](./cmd/sample-server), whose performance is measured -calculats Flint-Hill Series with 128-bit precision.
- A [sample client](./cmd/sample-client) that demonstrates the use of the `RunTest` function with a single task.

## Additional Utilities

The project provides two utilities for measuring performance metrics:
- [proc-stat](./cmd/proc-stat): for native applications
- [docker-stat](./cmd/docker-stat): for Docker containers

These utilities produce consistent output of performance parameters. Both utilities continue measuring until the process or Docker container exits. The measurement frequency is controlled by the `-refresh` command-line parameter, which supports fractional values of a second.
Metrics are specified using the `-params` command-line option. For example: `./proc-stat -params=CPU,PIDs,Cyc`

Available parameters include:
- `Cpu`: CPU time spent by the process.
- `CpuPerc`: Percentage of CPU time used by all cores to run the process (up to 100%).
- `Mem`: Amount of committed RAM for the process.
- `PIDs`: Number of process threads (lightweight processes).
- `CPUs`: Number of host processor cores available to the process.
- `Rx`: Total network read bytes.
- `Tx`: Total network write bytes.
- `Cyc`: Total CPU cycles spent by the process (proportional to `CpuPerc`).

## Performance Test Tips

1. The test should apply significant and sustainable load on the application. Control it with number of concurrent tests.
1. Number of tests should be large enough to minimize the impact of the "warm-up" period.
1. Docker allows control over system resources (memory and CPU) used by a container. However, CPU time measurements can be skewed.

TODO: Implement serialization and parsing of `RunTest` output. Implement a separate application that provides t-test results similar to "benchstat".