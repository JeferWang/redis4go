package redis4go

import (
	"bytes"
	"errors"
	"net"
	"strconv"
)

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

// 释放连接
func (conn *RedisConn) Release() {
	conn.IsRelease = true
	conn.p.Queue <- conn
}

// 关闭连接
func (conn *RedisConn) Close() error {
	err := conn.TcpConn.Close()
	if err != nil {
		return err
	}
	conn.IsClose = true
	return nil
}

func (conn *RedisConn) Lock() {
	conn.mu.Lock()
}

func (conn *RedisConn) Unlock() {
	conn.mu.Unlock()
}

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
	return resp, nil
}

func (conn *RedisConn) getReply() (*RedisResp, error) {
	bt := make([]byte, 1)
	bf := bytes.Buffer{}
	_, err := conn.TcpConn.Read(bt)
	if err != nil {
		return nil, err
	}
	bf.Write(bt)
	switch bt[0] {
	case '+':
		fallthrough
	case '-':
		fallthrough
	case ':':
		// 读取一行
		for {
			_, err := conn.TcpConn.Read(bt)
			if err != nil {
				return nil, err
			}
			bf.Write(bt)
			if bt[0] == '\n' {
				break
			}
		}
	case '$':
		// 读取两行
		line := 0
		for {
			_, err := conn.TcpConn.Read(bt)
			if err != nil {
				return nil, err
			}
			bf.Write(bt)
			if bt[0] == '\n' {
				line += 1
				if line == 2 {
					break
				}
			}
		}
	case '*':
		numBuf := bytes.Buffer{}
		for {
			_, err := conn.TcpConn.Read(bt)
			if err != nil {
				return nil, err
			}
			bf.Write(bt)
			if bt[0] != '\r' && bt[0] != '\n' {
				numBuf.Write(bt)
			}
			if bt[0] == '\n' {
				break
			}
		}
		num, err := strconv.Atoi(numBuf.String())
		if err != nil {
			return nil, err
		}
		line := 0
		for {
			_, err := conn.TcpConn.Read(bt)
			if err != nil {
				return nil, err
			}
			bf.Write(bt)
			if bt[0] == '\n' {
				line += 1
				if line == num*2 {
					break
				}
			}
		}
	default:
		return nil, errors.New("错误的服务器回复")
	}
	return &RedisResp{respData: bf.Bytes(), respLen: bf.Len()}, nil
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
