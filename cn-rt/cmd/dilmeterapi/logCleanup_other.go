//go:build !windows

package main

import "errors"

func moveFilesToRecycleBin(_ []string) error {
	return errors.New("当前系统不支持 Windows 回收站")
}
