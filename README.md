# Problem Setup

Consider a scenario where you need to evaluate the performance impact of a code change in a service. When isolating the code for testing is impractical or requires extensive refactoring, integration testing becomes the only viable option. Typically, the service under test depends on other services. During periods of waiting for responses, the service does not utilize the CPU, which can skew statistics of response latencies (e.g., results of Go benchmarking or Gatling for Java). 

The goal of this project is to identify common metrics available across most operating systems and CPU architectures that can provide meaningful performance insights and be integrated into a CI/CD pipeline.

The initial candidates for these metrics include:
- Cumulative CPU time of the process, similar to the output of `top -b -d1 -p $pid` on Linux.
- The sum of CPU% values for the process (i.e., integral over time), which is particularly useful for Docker containers.
- The count of CPU cycles or instructions, obtainable on Linux using the `perf stat ...` command.

However, measurements on actual services revealed statistical errors in all the above metrics. Since CI/CD pipelines typically run tests only once, this brings us back to using response latencies. The proposed solution is to leverage t-test statistics of response latencies before and after the code change (refer to https://pkg.go.dev/golang.org/x/perf/cmd/benchstat). Final decision whether to use cumulative values od application performance or latencies statistics depends on the nature of the service and its eco system.

## Test Setup

The main entry point for performance testing is the `RunTest` method. It takes the following arguments:
- A list of tasks to run (e.g., REST requests to an external server) - `tasks`
- The total number of tests to run - `totalRuns`
- The number of tests to run in parallel - `concurrent`

The function ensures a constant number of concurrent tests to maintain an even load. It returns separate statistics for each task, including:
- Invocation count
- Fail count
- Average, median, minimum, and maximum values
- Standard deviation
- Raw values of response latencies

Separating statistics is done to allow extending the set of tasks as the service evolves.

### Statistical analysis
The library provides `CalcPvals` function to compare results of two test runs, It calculates probability that latencies in the second run are are greater  than in the first for each test using "t-test" statistics. `RunStats` structure is annotated to ease [de-]serialization to JSON or YAML.

## Sample Applications

The project includes:
- A [sample server](./cmd/sample-server), whose performance is measured.
- A [sample client](./cmd/sample-client) that demonstrates the use of the `RunTest` function with a single task.

The server performs calculations for the Flint-Hill Series with 128-bit precision.

## Additional Utilities

The project provides two similar utilities for measuring performance metrics:
- [proc-stat](./cmd/proc-stat): for native applications
- [docker-stat](./cmd/docker-stat): for Docker containers

These utilities produce uniform output similar to `top -b -d1 -p $pid` on Linux or `docker stats $cid`. Both utilities continue measuring until the process or Docker container exits. The measurement frequency is controlled by the `-refresh` command-line parameter, which supports fractional values of a second.

Metrics are specified using the `-params` command-line option. For example:  
`./proc-stat -params=CPU,PIDs,Cyc`

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
