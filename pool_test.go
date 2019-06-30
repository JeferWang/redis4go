package redis4go

import (
	"fmt"
	"sync"
	"testing"
)

func TestPool(t *testing.T) {
	pool, err := CreatePool(PoolConfig{
		RedisConfig: RedisConfig{
			Host: "127.0.0.1",
			Port: 6379,
		},
		PoolSize: 10,
	})
	if err != nil {
		t.Log("Pool创建失败：", err.Error())
		return
	}
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func() {
			conn := pool.getConn()
			defer func() {
				conn.Release()
				wg.Done()
			}()
			t.Log("Get：", fmt.Sprintf("%p", conn))
		}()
		go func() {
			conn := pool.getConn()
			defer func() {
				conn.Release()
				wg.Done()
			}()
			t.Log("Get：", fmt.Sprintf("%p", conn))
		}()
		go func() {
			conn := pool.getConn()
			defer func() {
				conn.Release()
				wg.Done()
			}()
			t.Log("Get：", fmt.Sprintf("%p", conn))
		}()
	}
	wg.Wait()
	pool.Close()
}
