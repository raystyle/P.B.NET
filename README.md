# P.B.NET
P.B.NET为团队提供一个平台来渗透受限的网络，隐蔽、稳定、拥有可高度自定义的配置。
## Role
### Controller
  * 发送命令给Node或Beacon，获取返回结果
  * 连接指定数量的Node进行数据同步
  * Controller <-> Node 是C/S
  * 提供一个WebServer进行交互
### Node
  * 接收Controller的命令，执行并且返回结果
  * 连接指定数量的Node进行数据同步
  * 接收并存储来自Controller、Node、Beacon的数据
  * Node <-> Node 是P2P
### Beacon
  * 接收Controller的命令，执行并且返回结果
  * 连接指定数量的Node进行数据同步
  * Beacon <-> Node 是C/S
## Internal
以下关键模块组合使用可以实现在这些受限的网络中通信：
* 不允许所有向外流量，也没有Socks、HTTP代理，但是允许UDP流量
* 不允许所有向外流量，有Socks或者HTTP代理
* 不允许所有向外流量，也没有Socks、HTTP代理，这种情况怎么进来的就怎么出去\
  后续会添加一个Bind模式来解决这种情况(比如在配合reGeorg的情况下, Beacon使用\
  Bind模式，Node对应使用Connect+reGeorg代理的方式来连接内网的Beacon)
### Proxy
* 实现了Socks5的客户端和服务端，客户端支持代理链
* 实现了HTTP代理的客户端和服务端
* 实现了一个简单的代理池，方便后面的role.global调用
* DNS Client，Time Syncer，xnet都可以通过配置从代理池获取代理客户端 
### DNS
* 在系统之上实现了DNS客户端，提高安全性，当然仍然可以通过指定System模式使用系统解析
* 支持UDP、TCP、DoT（DNS-Over-TLS）、DoH(DNS-Over-HTTPS)四种方式解析域名
* 支持Punycode，也就是支持非英文的域名。
* 在客户端实现了缓存，可以设置超时时间以及手动清除缓存
* 除了UDP方式之外其他的模式都支持通过代理来解析
### Time Syncer
* 在系统之上实现了时间同步客户端，提高安全性（Node与Node之间的P2P网络非常依赖时间）
* 实现了NTP客户端（NTP使用UDP协议）
* 使用了一个小技巧能够使用一个HTTP客户端来同步时间（TCP）
* 内部自加时(虽然有误差，不过有同步所以不担心)，避免修改系统时间带来影响(GUID生成)
* 可以指定配置使用系统时间
* 使用HTTP同步方式的时候可以通过代理来同步时间
### NET(Role与Role之间的通信模块)
* 全部实现了net.Listener、net.Conn接口
* 实现了light自有协议(简单密钥交换，加密通信)
* 实现了TLS(TCP)、HTTP(Websocket TCP)、QUIC(UDP)
* 除了QUIC之外其他的模式都支持代理
### Bootstrap
* HTTP 是首选的使用方式，可以单独配置每一个bootstrap Node，加密存储在一个网页中并且内容\
  经过了Controller的签名，可以通过代理解析
* DNS 通过解析域名获取Node的IP地址，组合配置好的mode、network、port得到bootstrap Node\
  缺点是配置不方便改动，通常做为第二Bootstrap，可以通过代理解析
* Direct 也就是硬编码模式，直接把bootstrap Node写入其中，可以用来混淆、测试、配合内网跳板
* 每种方式在只有在程序加载配置以及Resolve的时候会出现明文配置，其余时间都是加密存储在内存中
* 可自己添加引导方式，需要实现Bootstrap接口
### Crypto
* 使用ed25519进行数字签名
* 使用curve25519进行密钥交换
* 使用AES256+SHA256进行数据加密和校验
* cert用来生成CA证书和签发证书
### GUID
* 实现了一个简单的GUID生成器
* 时间字段通过Time Syncer来获取(Node与Node之间的P2P网络非常依赖它)
