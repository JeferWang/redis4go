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
