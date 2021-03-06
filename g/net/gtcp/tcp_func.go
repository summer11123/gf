// Copyright 2017 gf Author(https://gitee.com/johng/gf). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://gitee.com/johng/gf.

package gtcp

import (
    "io"
    "net"
    "time"
    "bytes"
)

const (
    gDEFAULT_RETRY_INTERVAL   = 100   // (毫秒)默认重试时间间隔
    gDEFAULT_READ_BUFFER_SIZE = 1024  // 默认数据读取缓冲区大小

)

type Retry struct {
    Count    int  // 重试次数
    Interval int  // 重试间隔(毫秒)
}

// 自定义的包分割符号，用于标识包是否读取结束
// 注意：
// 1. 必须使用gtcp包来发送和接收tcp数据才有效；
// 2. 只有在发送的字节数为buffer size倍数时才有效；
var pkgSplitMark = []byte{0, 'E', 'O', 'P', 0}

// 常见的二进制数据校验方式，生成校验结果
func Checksum(buffer []byte) uint32 {
    var checksum uint32
    for _, b := range buffer {
        checksum += uint32(b)
    }
    return checksum
}

// 创建原生TCP链接, addr地址格式形如：127.0.0.1:80
func NewNetConn(addr string, timeout...int) (net.Conn, error) {
    if len(timeout) > 0 {
        return net.DialTimeout("tcp", addr, time.Duration(timeout[0]) * time.Millisecond)
    } else {
        return net.Dial("tcp", addr)
    }
}

// 获取数据
func Receive(conn net.Conn, retry...Retry) ([]byte, error) {
    var err    error
    var length int
    var buffer []byte
    size := gDEFAULT_READ_BUFFER_SIZE
    data := make([]byte, 0)
    for {
        buffer      = make([]byte, size)
        length, err = conn.Read(buffer)
        // 这里使用 "&&" 只要有数据不管有无错误都将先进行解析
        if length < 1 && err != nil {
            // 链接已关闭
            if err == io.EOF {
                break
            }
            if len(retry) > 0 {
                // 其他错误，重试之后仍不能成功
                if retry[0].Count == 0 {
                    break
                }
                retry[0].Count--
                if retry[0].Interval == 0 {
                    retry[0].Interval = gDEFAULT_RETRY_INTERVAL
                }
                time.Sleep(time.Duration(retry[0].Interval) * time.Millisecond)
                continue
            }
            break
        } else {
            // 自定义结束标识符判断
            if length == len(pkgSplitMark) && bytes.Compare(pkgSplitMark, buffer[0 : length]) == 0 {
                break
            }
            data = append(data, buffer[0 : length]...)
            if length < gDEFAULT_READ_BUFFER_SIZE || err == io.EOF {
                break
            }
        }
    }
    return data, err
}

// 带超时时间的数据获取
func ReceiveWithTimeout(conn net.Conn, timeout time.Duration, retry...Retry) ([]byte, error) {
    conn.SetReadDeadline(time.Now().Add(timeout))
    return Receive(conn, retry...)
}

// 发送数据
func Send(conn net.Conn, data []byte, retry...Retry) error {
    if len(data) % gDEFAULT_READ_BUFFER_SIZE == 0 {
        data = append(data, pkgSplitMark...)
    }
    length := 0
    for {
        n, err := conn.Write(data)
        if err != nil {
            // 链接已关闭
            if err == io.EOF {
                return err
            }
            // 其他错误，重试之后仍不能成功
            if len(retry) == 0 || retry[0].Count == 0 {
                return err
            }
            if len(retry) > 0 {
                retry[0].Count--
                if retry[0].Interval == 0 {
                    retry[0].Interval = gDEFAULT_RETRY_INTERVAL
                }
                time.Sleep(time.Duration(retry[0].Interval) * time.Millisecond)
            }
        } else {
            length += n
            if length == len(data) {
                return nil
            }
        }
    }
    return nil
}

// 带超时时间的数据发送
func SendWithTimeout(conn net.Conn, data []byte, timeout time.Duration, retry...Retry) error {
    conn.SetWriteDeadline(time.Now().Add(timeout))
    return Send(conn, data, retry...)
}

// 发送数据并等待接收返回数据
func SendReceive(conn net.Conn, data []byte, retry...Retry) ([]byte, error) {
    if err := Send(conn, data, retry...); err == nil {
        return Receive(conn)
    } else {
        return nil, err
    }
}

// 发送数据并等待接收返回数据(带返回超时等待时间)
func SendReceiveWithTimeout(conn net.Conn, data []byte, timeout time.Duration, retry...Retry) ([]byte, error) {
    if err := Send(conn, data, retry...); err == nil {
        return ReceiveWithTimeout(conn, timeout)
    } else {
        return nil, err
    }
}
