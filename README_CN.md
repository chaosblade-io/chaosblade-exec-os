# Chaosblade-exec-os: 基础资源混沌实验场景执行器
![license](https://img.shields.io/github/license/chaosblade-io/chaosblade.svg)

## 介绍
Chaosblade-exec-os 项目封装了基础资源故障场景，支持的组件包括 CPU、内存、网络、磁盘、进程、脚本等，使用 linux 命令或 golang 语言本身，以及 cgroup 资源管理实现。每个组件下又细分了很多故障，比如网络丢包、网络延迟，且场景有支持很多参数来控制影响面，且每个故障场景都有兜底策略，保障故障注入可控。

## 使用
此项目可以单独编译后使用，但更建议通过 [chaosblade](https://github.com/chaosblade-io/chaosblade) CLI 工具使用，因为其操作简单且就有完善的场景管理和命令提示。详细的中文使用文档请参考：https://chaosblade-io.gitbook.io/chaosblade-help-zh-cn/

## 编译
此项目采用 golang 语言编写，所以需要先安装最新的 golang 版本，最低支持的版本是 1.11。Clone 工程后进入项目目录执行以下命令进行编译：
```shell script
make
```
如果在 mac 系统上，编译当前系统的版本，请执行：
```shell script
make build_darwin
```
如果想在 mac 系统上，编译 linux 系统版本，请执行：
```shell script
make build_linux
```
你也可以只 clone [chaosblade](https://github.com/chaosblade-io/chaosblade) 项目，在项目目录下执行 `make` 或 `make build_linux` 来统一编译，实现通过 blade cli 工具执行此项目故障场景。

## 缺陷&建议
欢迎提交缺陷、问题、建议和新功能，所有项目（包含其他项目）的问题都可以提交到[Github Issues](https://github.com/chaosblade-io/chaosblade/issues) 

你也可以通过以下方式联系我们：
* 钉钉群（推荐）：23177705
* Gitter room: [chaosblade community](https://gitter.im/chaosblade-io/community)
* 邮箱：chaosblade.io.01@gmail.com
* Twitter: [chaosblade.io](https://twitter.com/ChaosbladeI)

## 参与贡献
我们非常欢迎每个 Issue 和 PR，即使一个标点符号，如何参加贡献请阅读 [CONTRIBUTING](CONTRIBUTING.md) 文档，或者通过上述的方式联系我们。

## 开源许可证
Chaosblade-exec-os 遵循 Apache 2.0 许可证，详细内容请阅读 [LICENSE](LICENSE)
