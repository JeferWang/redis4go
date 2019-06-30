package redis4go

import (
	"errors"
	"strconv"
)

func (resp *RedisResp) ParseError() error {
	if resp.respData[0] != '-' {
		return nil
	}
	return errors.New(string(resp.respData[1 : resp.respLen-2]))
}

func (resp *RedisResp) ParseInt() (int, error) {
	switch resp.respData[0] {
	case '-':
		return 0, resp.ParseError()
	case ':':
		return strconv.Atoi(string(resp.respData[1 : resp.respLen-2]))
	case '$':
		// 跳过第一行
		idx := -1
		for {
			idx += 1
			if resp.respData[idx] == '\n' {
				break
			}
		}
		return strconv.Atoi(string(resp.respData[idx+1 : resp.respLen-2]))
	default:
		return 0, errors.New("错误的整数回复")
	}
}
