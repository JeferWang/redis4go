package redis4go

import (
	"testing"
)

func TestRedisConn(t *testing.T) {
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
	conn := pool.getConn()
	res, err := conn.Call("set", "demoInt", "12345")
	if err != nil {
		t.Log("Error:", err.Error())
	} else {
		t.Log("Success:", string(res.respData))
	}
	res, err = conn.Call("get", "demoInt")
	if err != nil {
		t.Log("Error:", err.Error())
	} else {
		t.Log(string(res.respData))
		listLen, err := res.ParseInt()
		if err != nil {
			t.Log("Parse Fail:", err.Error())
		} else {
			t.Log("Success:", listLen)
		}
	}
}
