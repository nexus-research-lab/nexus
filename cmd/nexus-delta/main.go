// 发布工具生成补丁，也能在 CI 中以与客户端相同的代码验证重建。
package main

import (
	"bufio"
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/appdelta"
	"os"
)

func run(args []string) error {
	if len(args) != 4 {
		return fmt.Errorf("usage: nexus-delta create|apply BASE INPUT OUTPUT")
	}
	old, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer old.Close()
	input, err := os.Open(args[2])
	if err != nil {
		return err
	}
	defer input.Close()
	out, err := os.OpenFile(args[3], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(out)
	switch args[0] {
	case "create":
		err = appdelta.Create(old, input, w)
	case "apply":
		err = appdelta.Apply(old, input, w)
	default:
		err = fmt.Errorf("unknown operation")
	}
	if err == nil {
		err = w.Flush()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(out.Name())
	}
	return err
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
