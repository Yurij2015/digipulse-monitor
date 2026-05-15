package worker

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PingRunner runs an ICMP probe for a host. Production uses the system ping binary.
type PingRunner interface {
	Run(ctx context.Context, host string) ([]byte, error)
}

// ExecPingRunner invokes the system ping command (Linux-style flags).
type ExecPingRunner struct{}

func (ExecPingRunner) Run(ctx context.Context, host string) ([]byte, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(pingCtx, "ping", "-c", "4", "-W", "2", host)

	return cmd.CombinedOutput()
}

func applyPingOutput(result *CheckResult, output []byte, err error) {
	if err != nil {
		result.Status = "down"
		result.ErrorMessage = "Ping failed: " + err.Error()

		return
	}

	summaryRe := regexp.MustCompile(`[\d.,]+/([\d.,]+)/[\d.,]+`)
	summaryMatches := summaryRe.FindStringSubmatch(string(output))

	if len(summaryMatches) > 1 {
		avgStr := strings.Replace(summaryMatches[1], ",", ".", 1)
		avg, _ := strconv.ParseFloat(avgStr, 64)
		v := int64(avg)
		result.ResponseTimeMS = &v
	} else {
		lineRe := regexp.MustCompile(`time=([\d.,]+)`)
		lineMatches := lineRe.FindAllStringSubmatch(string(output), -1)
		if len(lineMatches) > 0 {
			var total float64
			for _, m := range lineMatches {
				valStr := strings.Replace(m[1], ",", ".", 1)
				val, _ := strconv.ParseFloat(valStr, 64)
				total += val
			}
			v := int64(total / float64(len(lineMatches)))
			result.ResponseTimeMS = &v
		}
	}

	result.Status = "up"
	if result.ResponseTimeMS == nil || *result.ResponseTimeMS <= 0 {
		one := int64(1)
		result.ResponseTimeMS = &one
	}
}
