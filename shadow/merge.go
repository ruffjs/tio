package shadow

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ruff.io/tio/pkg/log"
)

const (
	statePathSeparator = "."
)

// MergeMaps merge source map into target map
//
//   - Delete from the target when the field of source is nil
//   - Update target when the field of source is different from the target
//   - allMeta - all meta that also record the timestamp in milliseconds when the field of target is updated
//   - updatedMeta - just record for updated field
//
//   The path of the timestamp field of meta is the same as the target field.

func MergeState(tgt *StateValue, src StateValue, allMeta, updatedMeta *MetaValue) {
	tgtM := (map[string]any)(*tgt)
	srcM := (map[string]any)(src)
	am := (map[string]any)(*allMeta)
	um := (map[string]any)(*updatedMeta)
	doMergeState(&tgtM, srcM, &am, &um)
	*allMeta = am
	*updatedMeta = um
}

func doMergeState(tgt *map[string]any, src map[string]any, allMeta, updatedMeta *map[string]any) {
	if *updatedMeta == nil {
		*updatedMeta = map[string]any{}
	}
	if *allMeta == nil {
		*allMeta = map[string]any{}
	}
	if *tgt == nil {
		*tgt = src
		genMeta(*tgt, updatedMeta)
		return
	}
	tg := *tgt
	amt := *allMeta
	umt := *updatedMeta
	for key, srcValue := range src {
		if srcValue == nil {
			delete(tg, key)
			delete(amt, key)
		} else {
			tgtValue, exists := tg[key]
			mtVal, existM := amt[key]
			if exists {
				if subSrc, ok := srcValue.(map[string]any); ok {
					if subTgt, ok := tgtValue.(map[string]any); ok {
						// Recursive merge for nested maps
						var mVal map[string]any
						if existM {
							mVal = mtVal.(map[string]any)
						} else {
							mVal = make(map[string]any)
							amt[key] = mVal
						}
						umVal := make(map[string]any)
						umt[key] = umVal
						doMergeState(&subTgt, subSrc, &mVal, &umVal)
						if len(subTgt) == 0 {
							delete(umt, key)
							delete(amt, key)
						}
					} else {
						tg[key] = srcValue
						m := make(map[string]any)
						genMeta(subSrc, &m)
						amt[key] = m
						umt[key] = DeepCopyMap(m)
					}
				} else {
					tg[key] = srcValue
					amt[key] = map[string]any{"timestamp": time.Now().UnixMilli()}
					umt[key] = map[string]any{"timestamp": time.Now().UnixMilli()}
				}
			} else {
				tg[key] = srcValue
				if sm, ok := srcValue.(map[string]any); ok {
					m := make(map[string]any)
					genMeta(sm, &m)
					amt[key] = m
					umt[key] = DeepCopyMap(m)
				} else {
					amt[key] = map[string]any{"timestamp": time.Now().UnixMilli()}
					umt[key] = map[string]any{"timestamp": time.Now().UnixMilli()}
				}
			}
		}
	}
}

func genMeta(s map[string]any, outMeta *map[string]any) {
	m := *outMeta
	for k, v := range s {
		sc, err := isScalar(v)
		if err != nil {
			log.Fatalf("%s", err)
		}
		if sc {
			m[k] = map[string]any{"timestamp": time.Now().UnixMilli()}
		} else {
			sm := make(map[string]any)
			m[k] = sm
			genMeta(v.(map[string]any), &sm)
		}
	}
}

func isScalar(v any) (bool, error) {
	var isScalar bool
	switch v.(type) {
	case map[string]any:
		isScalar = false
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
		float32, float64,
		string, bool, nil,
		json.Number,
		[]any:
		isScalar = true
	default:
		return false, fmt.Errorf("unsupported type %T for source, value %v", v, v)
	}
	return isScalar, nil
}

// DeltaState get the delta state by diff
// return the delta state and delta state metadata
func DeltaState(desired, reported, meta map[string]any) (StateValue, MetaValue) {
	if desired == nil {
		return nil, nil
	}
	if reported == nil {
		return desired, meta
	}
	var delta = map[string]any{}
	var deltaMeta = map[string]any{}
	for k, v := range desired {
		d, dm := deltaDiff(k, v, reported[k], meta)
		if d != nil {
			delta[k] = d
		}
		if dm != nil {
			deltaMeta[k] = dm
		}
	}
	return delta, deltaMeta
}

// deltaDiff find the field of target that is different from the source
//   - set the target value in delta at given path
//   - set the timestamp value in delta metadata at given path
func deltaDiff(key string, target, source any, meta map[string]any) (delta any, deltaMeta any) {
	switch t := target.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
		float32, float64,
		string, bool, nil:
		if t == source {
			return
		} else {
			delta = target
			if m, ok := meta[key]; ok {
				deltaMeta = m
			}
		}
	case map[string]any:
		if s, ok := source.(map[string]any); !ok {
			delta = target
			if m, ok := meta[key]; ok {
				deltaMeta = m
			}
		} else {
			var subMeta map[string]any
			if tmp, ok := meta[key]; ok {
				subMeta = tmp.(map[string]any)
			} else {
				subMeta = make(map[string]any)
			}
			for k, v := range t {
				sd, sdm := deltaDiff(k, v, s[k], subMeta)
				if sd != nil {
					if delta == nil {
						delta = make(map[string]any)
					}
					delta.(map[string]any)[k] = sd
				}
				if sdm != nil {
					if deltaMeta == nil {
						deltaMeta = make(map[string]any)
					}
					deltaMeta.(map[string]any)[k] = sdm
				}
			}
		}
	default:
		log.Errorf("unexpected value for shadow, key %q value %v", key, target)
	}
	return
}

func GetStateValue(s StateValue, path string) (any, bool) {
	return ValueByPath(s, path)
}

func GetMetadata(meta MetaValue, path string) (any, bool) {
	return ValueByPath(meta, path)
}

func ValueByPath(m map[string]any, path string) (any, bool) {
	if m == nil {
		return nil, false
	}
	p := splitPath(path)
	mm := m
	for i := 0; i < len(p); i++ {
		kv, ok := mm[p[i]]
		if !ok {
			return nil, false
		}
		if i == len(p)-1 {
			return kv, true
		}
		if nm, ok := kv.(map[string]any); ok {
			mm = nm
		} else {
			return nil, false
		}
	}
	return nil, false
}

func splitPath(p string) []string {
	return strings.Split(p, statePathSeparator)
}

func MergeTags(current TagsValue, expect TagsValue) TagsValue {
	if current == nil {
		current = TagsValue{}
	}
	for k, v := range expect {
		current[k] = v
		if v == nil {
			delete(current, k)
		}
	}

	return current
}
