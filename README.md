

### Kubevirt

### 概述



### 组件和架构

KubeVirt 由一组服务组成：

支持docker及crio两种容器运行时，调度、网络、存储都委托给k8s，而Kubevirt则提供虚拟化功能。

```
            |
    Cluster | (virt-controller)
            |
------------+---------------------------------------
            |
 Kubernetes | (VMI CRD)
            |
------------+---------------------------------------
            |
  DaemonSet | (virt-handler) (vm-pod M)
            |

M: Managed by KubeVirt
CRD: Custom Resource Definition
```

![image-20221119235440203](/Users/duqi/Library/Application Support/typora-user-images/image-20221119235440203.png) 

#### 二、场景和流程

虚拟机创建的流程：

```
1、执行 kubectl apply -f vm.yaml后，k8s会选择合适的node节点并创建对应的virt-launcher-xxx Pod对象 2、启动 virt-launcher 进程监听来自 virt-handler 的消息 
3、节点上的 virt-handler 通过 Informer 监听到有新 vmi 创建到自己的节点后 
4、发送 gRPC 消息把该 vmi 的各项配置发给 virt-launcher 进程，启动虚拟机 
5、virt-launcher 接收到 vmi 的配置后转成虚拟机的 xml 文件 
6、启动该虚拟机，更新 vmi 的虚拟机状态
```

VNC连接虚拟机：

```
1、用户执行virtctl vnc命令
2、请求到达apiServer后通过aggregator机制转发到virt-api
3、virt-api根据vmi中的节点信息找到该节点的ip，并通过约定好的端口请求对应节点上的virt-handler

4、virt-handler通过unix sock文件找到虚拟机对应的virt-launcher，发起vnc请求
5、virt-launcher调libvirtd的vnc接口
```



#### 三、项目目录和组件

项目目录:

kubevirt 组件都是从 cmd/virt-* 开始的。可以进去相关的组件直接去编译，比如cmd/virt-api

```
kubevirt
|---api  OpenAPI规范
|---bazel       提供编译脚本，编译不同平台
|
|---cluster-up  提供了一个围绕 gocli 的包装器,
|
|---cmd         此目录包含所有主要组件源，子目录还包含Dockerfiles，可用于为每个组件构建映像
    ｜
		｜----virt-api          如下组件功能，功能的实现调用pkg目录下的相同目录
		｜----virt-controller
		｜----virt-handler
		｜----virt-launcher
		｜----virt-operator
		｜----virtctl
|
|---docs        md帮助文档
|---example     vm、vmi 例子
|---manifests   需要在目录下添加API访问控制
|
|---pkg
	  |
	  |-----virt*   供cmd调用，实现virt*组件的功能
	  |-----
	  |-----
	  |-----
|
|---staging      client-go的SDK代码可以通过修改如下目录


client-go的SDK代码可以通过修改如下目录：
kubevirt\staging\src\kubevirt.io\client-go修改后会在vendor中生效

需要在目录下添加API访问控制：
kubevirt\manifests\generated\rbac-operator.authorization.k8s.yaml.in


kubevirt\pkg\virt-operator\resource\generate\components\deployments.go
NewApiServerService(namespace string) *corev1.Service

kubevirt\pkg\virt-operator\resource\generate\install\strategy.go
controllerDeployments（）
用于生成SpiceServerDeployment pod资源服务的

kubevirt\pkg\virt-operator\resource\generate\components\apiservices.go
newApiServerClusterRole（）函数添加上面的rbac访问控制
```

cluster-up：

```
提供版本的预部署Kubernetes,
纯粹在docker容器中使用qemu,提供的虚拟机完全是临时的,并且在每次集群重启时重新创建

环境预部署：
1、make cluster-up：
这将部署一个全新的环境，其中的内容 KUBE_PROVIDER将用于确定cluster 将部署目录中的哪个提供程序。

2、make cluster-sync: 
在部署新环境后，或在此树中更改代码后，此命令会将运行中的 KubeVirt 环境中的 Pod 和 DaemonSet 与此树的状态同步。

3、make cluster-down：
这将关闭正在运行的 KubeVirt 环境


在kubevirt的代码中
1、cluster-up/kubectl.sh：
这是 Kubernetes 的 kubectl 命令的包装器，因此它可以直接从此运行，而无需登录节点。

2、cluster-up/virtctl.sh
是一个包装器virtctl。virtctl带来所有虚拟机特定的命令。例如cluster-up/virtctl.sh console testvm。

3、cluster-up/cli.sh
帮助您创建临时 kubernetes 和 openshift 集群进行测试。当需要直接管理或访问集群节点时，这很有帮助。
例如cluster-up/cli.sh ssh node01。
```



KubeVirt的关键组件是virt-api、virt-controller、virt-handler、virt-launcher

```
virt-api-server:
HTTP API 服务器，作为所有虚拟化相关流程的入口点,它负责默认和验证所提供的 VMI CRD。
以deployment的形式运行，提供VM、VMI等CRD资源的 webhook接口
通过k8s的aggregator特性，对外暴露虚拟机操作的相关接口，包括restart、stop、console、vnc等操作。


VMI(CRD):
VMI 定义作为自定义资源保存在 Kubernetes API 服务器中。
VMI 定义是定义虚拟机本身的所有属性，例如
机器的种类、中央处理器类型、RAM 和 vCPU 的数量、NIC 的数量和类型


virt-controller:
virt-controller 具有所有集群范围的虚拟化功能
该控制器负责监控 VMI (CR) 并管理关联的 pod。目前，控制器将确保创建和管理与 VMI 对象关联的 pod 的生命周期


virt-handler：
以daemonSet的形式运行在每个节点上（配置hostNetwork+hostpid）
1、提供rest接口供virt-api调用，接口功能包括console、vnc等功能
virt-api通过VMI的status.nodeName+约定好的端口找到VMI对应节点上的virt-handler https服务，
这也是为什么virt-handler要配置hostNetwork网络。

2、通过list-watch机制确保相应的libvirt domain的启动或者停止，但是这个过程并不是virt-handler
与对应的libvirt直接交互，而是virt-handler通过unix socket文件访问virt-launcher，
virt-launcher在与libvirt交互。
virt-handler与virt-launcher交互的sock文件为/var/run/kubevirt/sockets/{vmi_uid}_sock


virt-launcher：
每个正在运行的 VMI 都有一个。从 virt-handler 接收生命周期命令。
1、以unix-sock形式启动一个grpc server，该server负责提供接口供virt-handler调用
2、管理VMI对应的POD内的libvirt、qemu进程的生命周期，例如qemu进程异常后会自动重启。


libvirtd:
使用libvirtd来管理VMI进程的生命周期
```



#### 四、代码实现

```
最新的版本为0.58,目前是基于master分支上去新建分支去添加修改。

已经实现代码的逻辑 
spice连接使用传统的spice协议，没有websocket连接和virtctl二进制中的代理

1、启动虚拟机 
2、相关的 virt-handler 在范围内的特定端口上创建一个 unix 到 tcp 代理，并使用 virt-handler pod ip 和创建的端口更新 spiceHandler 数据。 
3、用户运行virtctl spice 
4、virt api 为 spice 连接（spice 密码）配置一个令牌以及请求的时间（用于令牌过期） 
5、virt-api 找到指向 virt-spice pod 的 spice-service 并获取创建的 loadbalancer 类型服务的端口和 ip 6、用户使用运行命令后创建的 vv 文件virtctl spice(`remote-viewer .vv) 
7、创建与其中一个 virt-spice 的连接，virt-spice 获取令牌并搜索相关的 vmi 对象（使用令牌作为 vmi 对象的标签） 
8、virt-spice 检查过期并获取特定 vmi 的 spiceHandler 数据（virt-handler ip 和端口） 
9、virt-handler 代理从 virt-spice 到与 qemu 进程相关的 unix 套接字的 tcp 连接的流量
```

#### 五、编译调试和安装
**GO**
```
目前kubevirt的最新版本需要go1.19以上支持,go mod tidy的时候需要本地有相应的版本，或者在go.mod修改相应版本


Linux安装如下：
wget https://golang.google.cn/dl/go1.19.3.linux-amd64.tar.gz    
tar zxf go1.19.3.linux-amd64.tar.gz  -C /usr/local/

vim ~/.bashrc
export GOROOT=/usr/local/go
export GOPATH=/home/duqi/golang
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin

go env -w GOPROXY=https://goproxy.cn,direct

go version

```


**Kubevirt**
```
如果生成operator执行make即可，最后会在manifests/release目录下生成kubevirt-operator.yaml、kubevirt-cr.yaml
用于在新的k8s环境中部署，代码修改
遇到的问题详见文档


调试目前采用替换Docker镜像方式进行替换更新，详见文档


可以通过添加日志或者打印的方式去调试添加的代码，查看日志可以通过
选择其中一个 pod 并获取其日志或者/var/log/pod/下面组件的日志:
kubectl logs -n <KubeVirt Install Namespace> virt-handler-2m86x | head -n8
kubectl logs -n <KubeVirt Install Namespace> virt-handler-2m86x

```