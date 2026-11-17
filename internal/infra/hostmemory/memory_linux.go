// INPUT: /proc/meminfo 的总物理内存与无需换页即可使用的内存。
// OUTPUT: 宿主物理内存余量是否足够；缺失 MemAvailable 时返回错误。
// POS: Linux 宿主内存探针；不代替 cgroup 内存限制。
package hostmemory

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func memoryAvailable(reserveMiB uint64) (bool, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return false, err
	}
	defer f.Close()
	var total, available uint64
	var foundAvailable bool
	scanner := bufio.NewScanner(io.LimitReader(f, 64<<10))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || (fields[0] != "MemTotal:" && fields[0] != "MemAvailable:") {
			continue
		}
		if len(fields) != 3 || fields[2] != "kB" {
			return false, errors.New("invalid /proc/meminfo memory units")
		}
		kib, err := strconv.ParseUint(fields[1], 10, 54)
		if err != nil {
			return false, fmt.Errorf("read /proc/meminfo: %w", err)
		}
		if fields[0] == "MemTotal:" {
			total = kib << 10
		} else {
			available, foundAvailable = kib<<10, true
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	if !foundAvailable {
		return false, errors.New("/proc/meminfo has no MemAvailable")
	}
	return enoughMemory(total, available, reserveMiB)
}
