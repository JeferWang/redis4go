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
