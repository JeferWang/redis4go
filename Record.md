# Go语言之从0到1实现一个简单的Redis连接池

## 前言

最近学习了一些Go语言开发相关内容，但是苦于手头没有可以练手的项目，学的时候理解不清楚，学过容易忘。

结合之前组内分享时学到的Redis相关知识，以及Redis Protocol文档，就想着自己造个轮子练练手。

这次我把目标放在了Redis client implemented with Go，使用原生Go语言和TCP实现一个简单的Redis连接池和协议解析，以此来让自己入门Go语言，并加深理解和记忆。（这样做直接导致的后果是，最近写JS时if语句总是忘带括号QAQ）。

本文只能算是学习Go语言时的一个随笔，并不是真正要造一个线上环境可用的Go-Redis库~(︿(￣︶￣)︿摊手)

Redis协议主要参考这篇文档[通信协议（protocol）](http://doc.redisfans.com/topic/protocol.html)，阅读后了解到，Redis Protocol并没有什么复杂之处，主要是使用TCP来传输一些固定格式的字符串数据达到发送命令和解析Response数据的目的。

### 命令格式

根据文档了解到，Redis命令格式为（CR LF即\r\n）：
```
*<参数数量N> CR LF
$<参数 1 的字节数量> CR LF
<参数 1 的数据> CR LF
...
$<参数 N 的字节数量> CR LF
<参数 N 的数据> CR LF
```
命令的每一行都使用CRLF结尾，在命令结构的开头就声明了命令的参数数量，每一条参数都带有长度标记，方便服务端解析。

例如，发送一个SET命令`set name jeferwang`：
```
*3
$3
SET
$4
name
$9
jeferwang
```

### 响应格式

Redis的响应回复数据主要分为五种类型：

- 状态回复：一行数据，使用`+`开头（例如：OK、PONG等）
```
+OK\r\n
+PONG\r\n
```
- 错误回复：一行数据，使用`-`开头（Redis执行命令时产生的错误）
```
-ERR unknown command 'demo'\r\n
```
- 整数回复：一行数据，使用`:`开头（例如：llen返回的长度数值等）
```
:100\r\n
```
- 批量回复（可以理解为字符串）：两行数据，使用`$`开头，第一行为内容长度，第二行为具体内容
```
$5\r\n
abcde\r\n

特殊情况：$-1\r\n即为返回空数据，可以转化为nil
```
- 多条批量回复：使用`*`开头，第一行标识本次回复包含多少条批量回复，后面每两行为一个批量回复（lrange、hgetall等命令的返回数据）
```
*2\r\n
$5\r\n
ABCDE\r\n
$2\r\n
FG\r\n
```

> 更详细的命令和回复格式可以从Redis Protocol文档了解到，本位只介绍一些基本的开发中需要用到的内容

以下为部分代码，完整代码见GitHub：[redis4go](https://github.com/JeferWang/redis4go)


## 实现流程

1. 首先，我们根据官网文档了解到了Redis传输协议，即Redis使用TCP传输命令的格式和接收数据的格式，据此，我们可以使用Go实现对Redis协议的解析
2. 接下来，在可以建立Redis连接并进行数据传输的前提下，实现一个连接池。
3. 实现拼接Redis命令的方法，通过TCP发送到RedisServer
4. 读取RedisResponse，实现解析数据的方法



## 模块结构分析

简单分析Redis连接池的结构，可以先简单规划为5个部分：

- 结构体定义`entity.go`
- Redis连接和调用`redis_conn.go`
- Redis数据类型解析`data_type.go`
- 连接池实现`pool.go`

共划分为上述四个部分


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
	Config PoolConfig          // 建立连接池时的配置
	Queue  chan *RedisConn     // 连接池
	Store  map[*RedisConn]bool // 所有的连接
	mu     sync.Mutex          // 加锁
}

// 单个Redis连接的结构
type RedisConn struct {
	mu        sync.Mutex   // 加锁
	p         *Pool        // 所属的连接池
	IsRelease bool         // 是否处于释放状态
	IsClose   bool         // 是否已关闭
	TcpConn   *net.TCPConn // 建立起的到RedisServer的连接
	DBIndex   int          // 当前连接正在使用第几个Redis数据库
}

type RedisResp struct {
	rType byte     // 回复类型(+-:$*)
	rData [][]byte // 从TCP连接中读取的数据统一使用二维数组返回
}
```

根据之前的规划，定义好基本的结构之后，我们可以先实现一个简单的Pool对象池

## Redis连接

### 建立连接

首先我们需要实现一个建立Redis连接的方法

```go
// 创建一个RedisConn对象
func createRedisConn(config RedisConfig) (*RedisConn, error) {
	tcpAddr := &net.TCPAddr{IP: net.ParseIP(config.Host), Port: config.Port}
	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return &RedisConn{
		IsRelease: true,
		IsClose:   false,
		TcpConn:   tcpConn,
		DBIndex:   0,
	}, nil
}
```

### 实现连接池

在Go语言中，我们可以使用一个`chan`来很轻易地实现一个指定容量的队列，来作为连接池使用，当池中没有连接时，申请获取连接时将会被阻塞，直到放入新的连接。

```go
package redis4go

func CreatePool(config PoolConfig) (*Pool, error) {
	pool := &Pool{
		Config: config,
		Queue:  make(chan *RedisConn, config.PoolSize),
		Store:  make(map[*RedisConn]bool, config.PoolSize),
	}
	for i := 0; i < config.PoolSize; i++ {
		redisConn, err := createRedisConn(config.RedisConfig)
		if err != nil {
			// todo 处理之前已经创建好的链接
			return nil, err
		}
		redisConn.p = pool
		pool.Queue <- redisConn
		pool.Store[redisConn] = true
	}
	return pool, nil
}

// 获取一个连接
func (pool *Pool) getConn() *RedisConn {
	pool.mu.Lock()
	// todo 超时机制
	conn := <-pool.Queue
	conn.IsRelease = false
	pool.mu.Unlock()
	return conn
}

// 关闭连接池
func (pool *Pool) Close() {
	for conn := range pool.Store {
		err := conn.Close()
		if err != nil {
			// todo 处理连接关闭的错误？
		}
	}
}
```
## 发送命令&解析回复数据

下面是向RedisServer发送命令，以及读取回复数据的简单实现
```go
func (conn *RedisConn) Call(params ...interface{}) (*RedisResp, error) {
	reqData, err := mergeParams(params...)
	if err != nil {
		return nil, err
	}
	conn.Lock()
	defer conn.Unlock()
	_, err = conn.TcpConn.Write(reqData)
	if err != nil {
		return nil, err
	}
	resp, err := conn.getReply()
	if err != nil {
		return nil, err
	}
	if resp.rType == '-' {
		return resp, resp.ParseError()
	}
	return resp, nil
}

func (conn *RedisConn) getReply() (*RedisResp, error) {
	b := make([]byte, 1)
	_, err := conn.TcpConn.Read(b)
	if err != nil {
		return nil, err
	}
	resp := new(RedisResp)
	resp.rType = b[0]
	switch b[0] {
	case '+':
		// 状态回复
		fallthrough
	case '-':
		// 错误回复
		fallthrough
	case ':':
		// 整数回复
		singleResp := make([]byte, 1)
		for {
			_, err := conn.TcpConn.Read(b)
			if err != nil {
				return nil, err
			}
			if b[0] != '\r' && b[0] != '\n' {
				singleResp = append(singleResp, b[0])
			}
			if b[0] == '\n' {
				break
			}
		}
		resp.rData = append(resp.rData, singleResp)
	case '$':
		buck, err := conn.readBuck()
		if err != nil {
			return nil, err
		}
		resp.rData = append(resp.rData, buck)
	case '*':
		// 条目数量
		itemNum := 0
		for {
			_, err := conn.TcpConn.Read(b)
			if err != nil {
				return nil, err
			}
			if b[0] == '\r' {
				continue
			}
			if b[0] == '\n' {
				break
			}
			itemNum = itemNum*10 + int(b[0]-'0')
		}
		for i := 0; i < itemNum; i++ {
			buck, err := conn.readBuck()
			if err != nil {
				return nil, err
			}
			resp.rData = append(resp.rData, buck)
		}
	default:
		return nil, errors.New("错误的服务器回复")
	}
	return resp, nil
}

func (conn *RedisConn) readBuck() ([]byte, error) {
	b := make([]byte, 1)
	dataLen := 0
	for {
		_, err := conn.TcpConn.Read(b)
		if err != nil {
			return nil, err
		}
		if b[0] == '$' {
			continue
		}
		if b[0] == '\r' {
			break
		}
		dataLen = dataLen*10 + int(b[0]-'0')
	}
	bf := bytes.Buffer{}
	for i := 0; i < dataLen+3; i++ {
		_, err := conn.TcpConn.Read(b)
		if err != nil {
			return nil, err
		}
		bf.Write(b)
	}
	return bf.Bytes()[1 : bf.Len()-2], nil
}

func mergeParams(params ...interface{}) ([]byte, error) {
	count := len(params) // 参数数量
	bf := bytes.Buffer{}
	// 参数数量
	{
		bf.WriteString("*")
		bf.WriteString(strconv.Itoa(count))
		bf.Write([]byte{'\r', '\n'})
	}
	for _, p := range params {
		bf.Write([]byte{'$'})
		switch p.(type) {
		case string:
			str := p.(string)
			bf.WriteString(strconv.Itoa(len(str)))
			bf.Write([]byte{'\r', '\n'})
			bf.WriteString(str)
			break
		case int:
			str := strconv.Itoa(p.(int))
			bf.WriteString(strconv.Itoa(len(str)))
			bf.Write([]byte{'\r', '\n'})
			bf.WriteString(str)
			break
		case nil:
			bf.WriteString("-1")
			break
		default:
			// 不支持的参数类型
			return nil, errors.New("参数只能是String或Int")
		}
		bf.Write([]byte{'\r', '\n'})
	}
	return bf.Bytes(), nil
}
```

实现几个常用数据类型的解析

```go
package redis4go

import (
	"errors"
	"strconv"
)

func (resp *RedisResp) ParseError() error {
	if resp.rType != '-' {
		return nil
	}
	return errors.New(string(resp.rData[0]))
}

func (resp *RedisResp) ParseInt() (int, error) {
	switch resp.rType {
	case '-':
		return 0, resp.ParseError()
	case '$':
		fallthrough
	case ':':
		str, err := resp.ParseString()
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(str)
	default:
		return 0, errors.New("错误的回复类型")
	}
}

func (resp *RedisResp) ParseString() (string, error) {
	switch resp.rType {
	case '-':
		return "", resp.ParseError()
	case '+':
		fallthrough
	case ':':
		fallthrough
	case '$':
		return string(resp.rData[0]), nil
	default:
		return "", errors.New("错误的回复类型")
	}
}
func (resp *RedisResp) ParseList() ([]string, error) {
	switch resp.rType {
	case '-':
		return nil, resp.ParseError()
	case '*':
		list := make([]string, 0, len(resp.rData))
		for _, data := range resp.rData {
			list = append(list, string(data))
		}
		return list, nil
	default:
		return nil, errors.New("错误的回复类型")
	}
}
func (resp *RedisResp) ParseMap() (map[string]string, error) {
	switch resp.rType {
	case '-':
		return nil, resp.ParseError()
	case '*':
		mp := make(map[string]string)
		for i := 0; i < len(resp.rData); i += 2 {
			mp[string(resp.rData[i])] = string(resp.rData[i+1])
		}
		return mp, nil
	default:
		return nil, errors.New("错误的回复类型")
	}
}
```

在开发的过程中，随手编写了几个零零散散的测试文件，经测试，一些简单的Redis命令以及能跑通了。

```go
package redis4go

import (
	"testing"
)

func getConn() (*RedisConn, error) {
	pool, err := CreatePool(PoolConfig{
		RedisConfig: RedisConfig{
			Host: "127.0.0.1",
			Port: 6379,
		},
		PoolSize: 10,
	})
	if err != nil {
		return nil, err
	}
	conn := pool.getConn()
	return conn, nil
}

func TestRedisResp_ParseString(t *testing.T) {
	demoStr := string([]byte{'A', '\n', '\r', '\n', 'b', '1'})
	conn, _ := getConn()
	_, _ = conn.Call("del", "name")
	_, _ = conn.Call("set", "name", demoStr)
	resp, err := conn.Call("get", "name")
	if err != nil {
		t.Fatal("Call Error:", err.Error())
	}
	str, err := resp.ParseString()
	if err != nil {
		t.Fatal("Parse Error:", err.Error())
	}
	if str != demoStr {
		t.Fatal("结果错误")
	}
}

func TestRedisResp_ParseList(t *testing.T) {
	conn, _ := getConn()
	_, _ = conn.Call("del", "testList")
	_, _ = conn.Call("lpush", "testList", 1, 2, 3, 4, 5)
	res, err := conn.Call("lrange", "testList", 0, -1)
	if err != nil {
		t.Fatal("Call Error:", err.Error())
	}
	ls, err := res.ParseList()
	if err != nil {
		t.Fatal("Parse Error:", err.Error())
	}
	if len(ls) != 5 {
		t.Fatal("结果错误")
	}
}

func TestRedisResp_ParseMap(t *testing.T) {
	conn, _ := getConn()
	_, _ = conn.Call("del", "testMap")
	_, err := conn.Call("hmset", "testMap", 1, 2, 3, 4, 5, 6)
	if err != nil {
		t.Fatal("设置Value失败")
	}
	res, err := conn.Call("hgetall", "testMap")
	if err != nil {
		t.Fatal("Call Error:", err.Error())
	}
	ls, err := res.ParseMap()
	if err != nil {
		t.Fatal("Parse Error:", err.Error())
	}
	if len(ls) != 3 || ls["1"] != "2" {
		t.Fatal("结果错误")
	}
}
```

至此，已经算是达到了学习Go语言和学习Redis Protocol的目的，不过代码中也有很多地方需要优化和完善，性能方面考虑的也并不周全。轮子就不重复造了，毕竟有很多功能完善的库，从头造一个轮子需要消耗的精力太多啦并且没必要~



下一次我将会学习官方推荐的`gomodule/redigo`源码，并分享我的心得。

--The End--

