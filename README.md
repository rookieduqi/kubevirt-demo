因为在k8s上部署kubevirt时使用的是0.49版本，所以基于kubevirt的tag0.49拉取的分支，并添加的代码。
目前最新的版本为0.58或者可以在master上去添加修改。



kubevirt 支持docker及crio两种容器运行时，调度、网络、存储都委托给k8s，而Kubevirt则提供虚拟化功能。

虚拟机创建的流程：
1、执行 kubectl apply -f vm.yaml后，k8s会选择合适的node节点并创建对应的virt-launcher-xxx Pod对象
2、启动 virt-launcher 进程监听来自 virt-handler 的消息
3、节点上的 virt-handler 通过 Informer 监听到有新 vmi 创建到自己的节点后
4、发送 gRPC 消息把该 vmi 的各项配置发给 virt-launcher 进程，启动虚拟机
5、virt-launcher 接收到 vmi 的配置后转成虚拟机的 xml 文件
6、启动该虚拟机，更新 vmi 的虚拟机状态


以实现代码的逻辑
spice连接使用传统的spice协议，没有websocket连接和virtctl二进制中的代理

1、启动虚拟机
2、相关的 virt-handler 在范围内的特定端口上创建一个 unix 到 tcp 代理，并使用 virt-handler pod ip 和创建的端口更新 spiceHandler 数据。
3、用户运行virtctl spice <vmi-name>
4、virt api 为 spice 连接（spice 密码）配置一个令牌以及请求的时间（用于令牌过期）
5、virt-api 找到指向 virt-spice pod 的 spice-service 并获取创建的 loadbalancer 类型服务的端口和 ip
6、用户使用运行命令后创建的 vv 文件virtctl spice(`remote-viewer .vv)
7、创建与其中一个 virt-spice 的连接，virt-spice 获取令牌并搜索相关的 vmi 对象（使用令牌作为 vmi 对象的标签）
8、virt-spice 检查过期并获取特定 vmi 的 spiceHandler 数据（virt-handler ip 和端口）
9、virt-handler 代理从 virt-spice 到与 qemu 进程相关的 unix 套接字的 tcp 连接的流量


项目目录：
kubevirt 组件都是从 cmd/virt-* 开始的
可以进去相关的组件，直接去编译，比如cmd/virt-api
