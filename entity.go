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
