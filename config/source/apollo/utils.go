package apollo

import (
	"github.com/micro/go-micro/v2/config/encoder"
	"strings"
)

func makeMap(e encoder.Encoder, kv map[string]string) (map[string]interface{}, error) {
	data := make(map[string]interface{})

	for k, v := range kv {
		pathString := k
		if pathString == "" {
			continue
		}

		var err error

		target := data
		path := strings.Split(k, ".")
		for _, dir := range path[:len(path)-1] {
			if _, ok := target[dir]; !ok {
				target[dir] = make(map[string]interface{})
			}
			target = target[dir].(map[string]interface{})
		}

		leafDir := path[len(path)-1]

		var vv interface{}
		err = e.Decode([]byte(v), &vv)
		if err != nil {
			return nil, err
		}
		target[leafDir] = vv
	}
	return data, nil
}
