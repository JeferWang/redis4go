# GoLang之从0到1实现一个简单的Go-Redis库

## 实现流程

1. 首先，我们根据官网文档了解到了Redis传输协议，即Redis使用TCP传输指令的格式和接收数据的格式，据此，我们可以使用Go实现对Redis协议的解析
2. 接下来，在可以建立Redis连接并进行数据传输的前提下，实现一个连接池。
3. 实现一些拼接Redis指令的方法
4. 实现一些解析Redis响应数据的方法
5. 最后，整理模块对外暴露的结构和方法，实现一个基本可用的Go-Redis连接库


## 模块结构分析

简单分析Redis连接池的结构，可以先简单规划为5个部分：

- 总成入口文件（留一个备用）`main.go`
- 实体定义文件`entity.go`
- Redis连接建立和维护`redis_protocol.go`
- Redis协议解析`redis_protocol.go`
- Redis数据类型解析`data_type.go`
- 连接池实现`pool.go`

共划分为上述六个部分，存放在redis4go包中。


## 对象结构定义

为了实现连接池及Redis数据库连接，我们需要如下结构：

- Redis服务器配置`RedisConfig`：包含Host、Port等信息
- Redis连接池配置`PoolConfig`：继承`RedisConfig`，包含PoolSize等信息
- Redis连接池结构：包含连接队列、连接池配置等信息
- 单个Redis连接：包含TCP连接Handler、是否处于空闲标记位、当前使用的数据库等信息

```go
package redis4go

import (
	"net"
	"sync"
)

type RedisConfig struct {
	Host     string // RedisServer主机地址
	Port     int    // RedisServer主机端口
	Password string // RedisServer需要的Auth验证，不填则为空
}

// 连接池的配置数据
type PoolConfig struct {
	RedisConfig
	PoolSize int // 连接池的大小
}

// 连接池结构
type Pool struct {
	Config PoolConfig      // 建立连接池时的配置
	Queue  chan *RedisConn // 连接池
	List   []*RedisConn    // 所有的连接
	mu     sync.Mutex      // 加锁
}

// 单个Redis连接的结构
type RedisConn struct {
	IsRelease bool         // 是否处于释放状态
	TcpConn   *net.TCPConn //建立起的到RedisServer的连接
	DBIndex   int          // 当前连接正在使用第几个Redis数据库
}
```

简单划分完成基本的结构之后，我们可以先实现一个简单的Pool对象池



## 实现Redis连接池

### 建立到RedisServer的连接

首先我们需要实现一个建立Redis连接的方法

```go
package redis4go

import "net"

func createRedisConn(config RedisConfig) (*RedisConn, error) {
	tcpAddr := &net.TCPAddr{IP: net.ParseIP(config.Host), Port: config.Port}
	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return &RedisConn{
		TcpConn:   tcpConn,
		IsRelease: true,
		DBIndex:   0,
	}, nil
}
```

### 实现一个简单的连接池机制

在Go语言中，我们可以使用一个`chan`来很轻易地实现一个指定容量的队列，来作为连接池使用，当池中没有连接时，申请获取连接时将会被阻塞，直到放入新的连接。

```go
package redis4go

func CreatePool(config PoolConfig) (*Pool, error) {
	pool := &Pool{
		Config: config,
		Queue:  make(chan *RedisConn, config.PoolSize),
		List:   make([]*RedisConn, config.PoolSize),
	}
	for i := 0; i < config.PoolSize; i++ {
		// 创建每一个Redis连接
		conn, err := createRedisConn(config.RedisConfig)
		if err != nil {
			// todo 处理之前已经创建好的链接
			return nil, err
		}
		// TCP连接包装一下
		redisConn := &RedisConn{
			TcpConn:   conn,
			IsRelease: true,
			DBIndex:   0,
		}
		pool.Queue <- redisConn
		pool.List = append(pool.List, redisConn)
	}
	return pool, nil
}
```
