package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MagalixTechnologies/log-go"
	"github.com/MagalixTechnologies/uuid-go"
	"github.com/reconquest/karma-go"
	"github.com/ryanuber/go-glob"
)

var stderr *log.Logger

func SetLogger(logger *log.Logger) {
	stderr = logger
}

func ExpandEnv(ref *string) {
	key := *ref
	if key[0] != '$' {
		return
	}

	value := os.Getenv(key[1:])
	if len(value) == 0 {
		stderr.Fatalf(nil, "Environment variable %s not found or empty", key)
		os.Exit(1)
	}

	*ref = value
}

func ParseUuidString(val string) uuid.UUID {
	id, err := uuid.FromString(val)
	if err != nil {
		stderr.Fatalf(err, "invalid UUID: %s", val)
		os.Exit(1)
	}

	return id
}

func MustParseDuration(str string) time.Duration {
	duration, err := time.ParseDuration(str)
	if err != nil {
		stderr.Fatalf(err, "unable to parse %s value as duration", str)
		os.Exit(1)
	}

	return duration
}

func MustDecodeSecret(str string) []byte {
	secret, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		stderr.Fatalf(err, "unable to base64 decode client secret")
		os.Exit(1)
	}

	return secret
}

func GetSanitizedArgs() []string {
	sensitive := []string{"--client-secret"}

	args := []string{}

args:
	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		for _, flag := range sensitive {
			if strings.HasPrefix(arg, flag) {
				var value string
				if strings.HasPrefix(arg, flag+"=") {
					value = strings.TrimPrefix(arg, flag+"=")
					// no need to hide value if it's name of env variable
					if value != "" && !strings.HasPrefix(value, "$") {
						arg = flag + "=<sensitive:" + fmt.Sprint(len(value)) + ">"
					}

					args = append(args, arg)
				} else {
					args = append(args, arg)
					if len(os.Args) > i+1 {
						value = os.Args[i+1]
						if value != "" && !strings.HasPrefix(value, "$") {
							value = "<sensitive:" + fmt.Sprint(len(value)) + ">"
						}
						args = append(args, value)
						i++
					}
				}

				continue args
			}
		}

		args = append(args, arg)
	}

	return args
}

func InSkipNamespace(skipNamespacePatterns []string, namespace string) bool {

	for _, pattern := range skipNamespacePatterns {
		matched := glob.Glob(pattern, namespace)
		if matched {
			return true
		}
	}

	return false
}

func Throttle(
	name string,
	interval time.Duration,
	tickLimit int32,
	fn func(args ...interface{})) func(args ...interface{},
) {
	getNextTick := func() time.Time {
		return time.Now().
			Truncate(time.Second).
			Truncate(interval).
			Add(interval)
	}

	nextTick := getNextTick()

	stderr.Info("{%s throttler} next tick at %s", name, nextTick.Format(time.RFC3339))

	var tickFires int32 = 0

	return func(args ...interface{}) {
		now := time.Now()
		if now.After(nextTick) || now.Equal(nextTick) {
			stderr.Infof(nil, "{%s throttler} ticking", name)
			fn(args...)

			atomic.AddInt32(&tickFires, 1)
			if tickFires >= tickLimit {
				atomic.StoreInt32(&tickFires, 0)
				nextTick = getNextTick()
			}

			stderr.Infof(nil,
				"{%s throttler} next tick at %s",
				name,
				nextTick.Format(time.RFC3339),
			)
		} else {
			stderr.Infof(nil, "{%s throttler} throttled", name)
		}
	}
}

// After returns pointer to time after specific duration
func After(d time.Duration) *time.Time {
	t := time.Now().Add(d)
	return &t
}

func TruncateString(str string, num int) string {
	truncated := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		truncated = str[0:num] + "..."
	}
	return truncated
}

func Transcode(
	u interface{},
	v interface{},
) error {
	b, err := json.Marshal(u)
	if err != nil {
		return karma.Format(
			err,
			"unable to marshal %T to json",
			u,
		)
	}

	err = json.Unmarshal(b, v)
	if err != nil {
		return karma.Format(
			err,
			"unable to unmarshal json into %T",
			v,
		)
	}

	return nil
}
