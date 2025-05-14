# Problem Setup

Existing tools like GoLang benchmark tests provide real-time latency measurements for isolated code. This approach works well for CPU-bound applications. However, in real-world scenarios, services often depend on disk I/O or other external services/databases. While waiting on I/O or network responses, the service does not consume CPU, so this waiting time should not affect performance metrics. 

Consider a situation where you want to evaluate the performance impact of adding or removing features in a service. Additionally, extracting code for isolated testing might not be feasible or could be too costly.

The purpose of this project is to evaluate various metrics available on most operating systems and CPU architectures that can provide meaningful performance insights.

The following metrics were evaluated:
- **CPU time of the process**
- **Sum of CPU% values of the process** (i.e., integral over time)
- **Count of CPU cycles** on some processors (AMD64, PPC64, ARM64). On Linux, this metric can be obtained using the `perf stat -e cycles,instructions ...` command.

It was observed that all these metrics have statistical errors, making performance comparisons no more reliable than comparing response latency statistics using a paired t-test.

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
- Raw values used for statistical calculations

## Sample Applications

The project includes:
- A [sample server](./cmd/sample-server), whose performance is measured.
- A [sample client](./cmd/sample-client) that demonstrates the use of the `RunTest` function with a single task.

The server performs calculations for the Flint-Hill Series with 128-bit precision.

## Additional Utilities

The project provides two similar utilities for measuring performance metrics:
- [proc-stat](./cmd/proc-stat): for native applications.
- [docker-stat](./cmd/docker-stat): for Docker containers.

These utilities produce uniform output similar to `top -b -d1 -p $pid` on Linux or `docker stats $cid`. Both utilities continue measuring until the process or Docker container exits. The measurement frequency is controlled by the `-refresh` command-line parameter, which supports fractional values of a second.

Metrics are specified using the `-params` command-line option. For example: `./proc-stat -params=CPU,PIDs,Cyc`

Available parameters include:
- **Cpu**: CPU time spent by the process.
- **CpuPerc**: Percentage of CPU time used by all cores to run the process (up to 100%).
- **Mem**: Amount of committed RAM for the process.
- **PIDs**: Number of process threads (lightweight processes).
- **CPUs**: Number of host processor cores available to the process.
- **Rx**: Total network read bytes.
- **Tx**: Total network write bytes.
- **Cyc**: Total CPU cycles spent by the process (proportional to `CpuPerc`).

## Performance Test Tips

1. The test should apply significant and sustainable load on the application. However, excessive load can distort measurements.
2. Separating statistics by tasks allows for extending the set of tasks as the measured service evolves.
3. Tests should be lengthy enough to minimize the impact of the "warm-up" period.
4. Docker allows control over system resources (memory and CPU) used by a container. However, CPU time measurements can be skewed.
5. Raw test duration values can be used to evaluate performance changes using a t-test.