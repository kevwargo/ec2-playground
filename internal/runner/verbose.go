package runner

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func (r InstanceRunner) dumpParams(in ec2.RunInstancesInput) {
	jsonPayload, _ := json.Marshal(in)
	var jsonData map[string]any
	json.Unmarshal(jsonPayload, &jsonData)

	jsonData = pruneMap(jsonData)
	jsonPayload, _ = json.MarshalIndent(jsonData, "", "  ")
	r.sess.Log("running ec2.RunInstances(%s)", string(jsonPayload))
}

func pruneMap(data map[string]any) map[string]any {
	for k, v := range data {
		v = pruneValue(v)
		if v != nil {
			data[k] = v
		} else {
			delete(data, k)
		}
	}

	if len(data) == 0 {
		return nil
	}
	return data
}

func pruneValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		return pruneMap(v)
	case []any:
		return pruneSlice(v)
	case string:
		if len(v) == 0 {
			return nil
		}
	}

	return v
}

func pruneSlice(src []any) (pruned []any) {
	for _, v := range src {
		if v = pruneValue(v); v != nil {
			pruned = append(pruned, v)
		}
	}

	if len(pruned) == 0 {
		return nil
	}

	return pruned
}
