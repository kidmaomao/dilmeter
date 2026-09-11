//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"gitlab.com/prilus/mabidilmeter/lib/pcaputil"
	"gitlab.com/prilus/mabidilmeter/lib/util"
)

func main2(ctx context.Context) {
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	logger.Println("* dilmatulgi", mode)
	cfg := loadConfig()
	if cfg.Key != "" && cfg.IV != "" {
		if err := packet.InitKoreaDamageBlock(cfg.Key, cfg.IV); err != nil {
			logger.Println("config: InitKoreaDamageBlock failed:", err)
		} else {
			logger.Println("config: loaded key/iv from config")
		}
	} else {
		logger.Println("config: no key/iv in config")
		saveConfig(cfg)
	}

	switch mode {
	case "list":
		// nic list 출력
		nics, err := pcaputil.NicList()
		if err != nil {
			messagebox("nic list load failed: " + err.Error())
			logger.Fatalln("nic list load failed:", err)
			return
		}

		sb := strings.Builder{}

		for _, nic := range nics {
			sb.WriteString(nic.String())
			sb.WriteRune('\n')
		}

		s := sb.String()
		messagebox(s)
		logger.Println(s)
		return

	case "file":
		fileName := ""

		if len(os.Args) > 2 {
			fileName = os.Args[2]
		}

		r, pub := run(ctx, cfg)

		err := r.OpenFile(fileName)
		if err != nil {
			msg := fmt.Sprintf("OpenFile failed: %v", err)
			logger.Println(msg)
			pub.publish(newMessageBoxEvent(msg))
			messagebox(msg)
		}

	case "stdin":
		r, pub := run(ctx, cfg)

		err := r.OpenStdin()
		if err != nil {
			msg := fmt.Sprintf("OpenStdin failed: %v", err)
			logger.Println(msg)
			pub.publish(newMessageBoxEvent(msg))
			messagebox(msg)
		}

	case "":
		r, pub := run(ctx, cfg)
		go func() {
			if util.IsOpenedStdin() {
				if err := r.OpenStdin(); err != nil {
					msg := fmt.Sprintf("OpenStdin failed: %v", err)
					logger.Println(msg)
					pub.publish(newMessageBoxEvent(msg))
					messagebox(msg)
				}
				return
			}

			logger.Println("find nic...")

			nicName, err := pcaputil.FindNic()
			if err != nil {
				msg := fmt.Sprintf("%v\nis mabinogi running?", err)
				logger.Println(msg)
				pub.publish(newMessageBoxEvent(msg))
				messagebox(msg)
				return
			}

			err = r.OpenNic(nicName)
			if err != nil {
				msg := fmt.Sprintf("OpenNic failed: %v", err)
				logger.Println(msg)
				pub.publish(newMessageBoxEvent(msg))
				messagebox(msg)
			}
		}()

	default:
		_, err := os.Stat(mode)
		fileExists := err == nil

		nicName, fileName := "", ""

		if fileExists {
			fileName = mode
		} else {
			nicName = mode
		}

		r, pub := run(ctx, cfg)

		if fileName != "" {
			err := r.OpenFile(fileName)
			if err != nil {
				msg := fmt.Sprintf("OpenFile failed: %v", err)
				logger.Println(msg)
				pub.publish(newMessageBoxEvent(msg))
				messagebox(msg)
			}
		} else if nicName != "" {
			err := r.OpenNic(nicName)
			if err != nil {
				msg := fmt.Sprintf("OpenNic failed: %v", err)
				logger.Println(msg)
				pub.publish(newMessageBoxEvent(msg))
				messagebox(msg)
			}
		} else {
			go func() {
				logger.Println("find nic...")

				nicName2, err := pcaputil.FindNic()
				if err != nil {
					msg := fmt.Sprintf("%v\nis mabinogi running?", err)
					logger.Println(msg)
					pub.publish(newMessageBoxEvent(msg))
					messagebox(msg)
					return
				}

				err = r.OpenNic(nicName2)
				if err != nil {
					msg := fmt.Sprintf("OpenNic failed: %v", err)
					logger.Println(msg)
					pub.publish(newMessageBoxEvent(msg))
					messagebox(msg)
				}
			}()
		}
	}

	for {
		time.Sleep(1 * time.Second)
	}
}
