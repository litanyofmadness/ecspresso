package ecspresso

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type jsonStringer interface {
	JSON() string
}

type stringer interface {
	String() string
}

func WriteOutput(v any) (int, error) {
	w := os.Stdout
	switch logFormat {
	case logFormatJSON:
		switch v := v.(type) {
		case jsonStringer:
			s := v.JSON()
			if strings.HasSuffix(s, "\n") {
				return io.WriteString(w, s)
			} else {
				return io.WriteString(w, s+"\n")
			}
		case string:
			b, _ := json.Marshal(v)
			b = append(b, '\n')
			return w.Write(b)
		default:
			return OutputJSONForAPI(w, v)
		}
	case logFormatText, "":
		switch v := v.(type) {
		case stringer:
			s := v.String()
			if strings.HasSuffix(s, "\n") {
				return io.WriteString(w, s)
			}
			return io.WriteString(w, s+"\n")
		case string:
			if strings.HasSuffix(v, "\n") {
				return io.WriteString(w, v)
			}
			return io.WriteString(w, v+"\n")
		default:
			return io.WriteString(w, fmt.Sprintf("%s\n", v))
		}
	}
	return 0, fmt.Errorf("unknown log format %s", logFormat)
}
