# Chaosblade-exec-os: Basic Resources Chaos Experiment Executor
![license](https://img.shields.io/github/license/chaosblade-io/chaosblade.svg)

中文版 [README](README_CN.md)

## Introduction
The chaosblade-exec-os project encapsulates basic resource failure scenarios. Supported components include CPU, memory, network, disk, process, shell scripts, etc., using linux commands or the golang language itself, and cgroup resource management implementation. Each component is further subdivided into many faults, such as network packet loss and network delay, and the scenario supports many parameters to control the influence surface, and each fault scenario has a bottom-up strategy to ensure controllable fault injection.

## How to use
This project can be compiled and used separately, but it is more recommended to use [chaosblade](https://github.com/chaosblade-io/chaosblade) CLI tool to execute, because its operation is simple and it has perfect experiments management and command prompt. For detailed Chinese documentation, please refer to: https://chaosblade-io.gitbook.io/chaosblade-help-zh-cn/

## Compile
This project is written in golang, so you need to install the latest golang version first. The minimum supported version is 1.11. After the Clone project, enter the project directory and execute the following command to compile:
```shell script
make
```
If on a mac system, compile the current system version, execute:
```shell script
make build_darwin
```
If you want to compile linux system version on mac system, execute:
```shell script
make build_linux
```
You can also only clone [chaosblade] (https://github.com/chaosblade-io/chaosblade) project, execute `make` or` make build_linux` in the project directory to compile it uniformly, and implement this project through blade cli Failure scenario.

## Bugs and Feedback
For bug report, questions and discussions please submit [GitHub Issues](https://github.com/chaosblade-io/chaosblade/issues). 

You can also contact us via:
* Dingding group (recommended for chinese): 23177705
* Gitter room: [chaosblade community] (https://gitter.im/chaosblade-io/community)
* Email: chaosblade.io.01@gmail.com
* Twitter: [chaosblade.io] (https://twitter.com/ChaosbladeI)

## Contributing
We welcome every contribution, even if it is just punctuation. See details of [CONTRIBUTING](CONTRIBUTING.md)

## License
The chaosblade-exec-os is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
