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
