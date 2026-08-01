package shadow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"log"
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
	srcCopy := DeepCopyMap(srcM)
	doMergeState(&tgtM, srcCopy, &am, &um)
	*tgt = tgtM
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
		genMeta(*tgt, allMeta)
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
						tg[key] = removeNilFieldsForValue(srcValue)
						m := make(map[string]any)
						genMeta(subSrc, &m)
						amt[key] = m
						umt[key] = DeepCopyMap(m)
					}
				} else {
					tg[key] = removeNilFieldsForValue(srcValue)
					amt[key] = map[string]any{"timestamp": time.Now().UnixMilli()}
					umt[key] = map[string]any{"timestamp": time.Now().UnixMilli()}
				}
			} else {
				tg[key] = removeNilFieldsForValue(srcValue)
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
		if v == nil {
			continue
		}
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

// removeNilFieldsForValue Before the value is saved to Shadow, remove the null field in it.
func removeNilFieldsForValue(srcValue any) any {
	if m, ok := srcValue.(map[string]any); ok {
		origLen := len(m)
		for k, v := range m {
			if v == nil {
				delete(m, k)
				continue
			}
			if sm, ok := v.(map[string]any); ok {
				if sr := removeNilFieldsForValue(sm); sr == nil {
					delete(m, k)
				}
			}
		}
		// If the map is empty after deletion and the original map is not empty, it means the field is deleted, so return nil.
		// Otherwise, return the original map, cause tio support empty object field.
		if len(m) == 0 && origLen > 0 {
			return nil
		}
		return srcValue
	}
	return srcValue
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
	case []any:
		if reflect.DeepEqual(t, source) {
			return
		} else {
			delta = target
			if m, ok := meta[key]; ok {
				deltaMeta = m
			}
		}
	default:
		slog.Error("unexpected value for shadow", "key", key, "value", target)
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
	for i := range p {
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

func MergePatch(old, patch map[string]any) (map[string]any, bool) {
	if patch == nil {
		return cloneMap(old), false
	}

	result := cloneMap(old)
	if result == nil {
		result = make(map[string]any)
	}

	changed := false
	for k, patchVal := range patch {
		oldVal, exists := result[k]

		if patchVal == nil {
			if exists {
				delete(result, k)
				changed = true
			}
			continue
		}

		patchMap, patchIsMap := patchVal.(map[string]any)
		oldMap, oldIsMap := oldVal.(map[string]any)

		if patchIsMap {
			if oldIsMap {
				merged, subChanged := MergePatch(oldMap, patchMap)
				if subChanged {
					result[k] = merged
					changed = true
				}
			} else {
				merged, subChanged := MergePatch(nil, patchMap)
				result[k] = merged
				if subChanged || !oldIsMap {
					changed = true
				}
			}
		} else {
			if !exists || !deepEqual(oldVal, patchVal) {
				result[k] = patchVal
				changed = true
			}
		}
	}

	return result, changed
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = cloneValue(v)
	}
	return result
}

func cloneValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			result[i] = cloneValue(item)
		}
		return result
	default:
		return value
	}
}

func deepEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	aMap, aIsMap := a.(map[string]any)
	bMap, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		if len(aMap) != len(bMap) {
			return false
		}
		for k, v := range aMap {
			if !deepEqual(v, bMap[k]) {
				return false
			}
		}
		return true
	}

	aSlice, aIsSlice := a.([]any)
	bSlice, bIsSlice := b.([]any)
	if aIsSlice && bIsSlice {
		if len(aSlice) != len(bSlice) {
			return false
		}
		for i, v := range aSlice {
			if !deepEqual(v, bSlice[i]) {
				return false
			}
		}
		return true
	}

	if af, aok := toFloat64(a); aok {
		if bf, bok := toFloat64(b); bok {
			return af == bf
		}
	}

	return a == b
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}
