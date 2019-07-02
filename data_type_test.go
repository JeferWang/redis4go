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
