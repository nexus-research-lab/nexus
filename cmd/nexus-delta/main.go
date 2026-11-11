// 发布时生成并回放应用目录差分。
package main

import (
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/appdelta"
	"os"
)

func main() {
	a := os.Args[1:]
	if len(a) != 6 {
		fmt.Fprintln(os.Stderr, "usage: nexus-delta create|apply FROM TO OLD INPUT OUTPUT")
		os.Exit(2)
	}
	var err error
	switch a[0] {
	case "create":
		var f *os.File
		f, err = os.OpenFile(a[5], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			err = appdelta.WriteDelta(f, a[1], a[2], a[3], a[4], nil)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				os.Remove(a[5])
			}
		}
	case "apply":
		err = appdelta.ApplyFile(a[4], a[1], a[2], a[3], a[5])
	default:
		err = fmt.Errorf("unknown operation")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
