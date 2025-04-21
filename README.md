
Existing tools like GoLang benchmark tests provide real-time latency of the tested code. This might be a good in CPU-bound application.
However, in real life services that depend on disk I/O or networks this doesn't work well. Execution time might vary significantly depending on external factors.
CPU profiling is not reliable even when performance tests run in controlled environment.
Consider situation when you want to evaluate change in service performance after adding/removing some features. 
Add on top the situation when extracing code is not feasible or too costly.
How to measure performance change?

== Using Performance Test


== Performance Test Requirements
1. The test should apply significan and sustainable load on the application. 
1. Test should be reproducible before and after feature change. Most likely that means that test set should not change.
1. The tests should be lengthy so that "warm-up" period can be neglected

Go shows clock time in benchmark tests (testing.B.Elapsed())
Java Gatling
Problem setup - how to measure service performance when it dependes on external elements - disks, DBs, other services.
